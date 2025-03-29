package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	tele "gopkg.in/telebot.v3"
)

const (
	embeddingServiceAddress = "http://localhost:8000/embeddings" // Address of the embedding service
	qdrantServiceAddress    = "http://localhost:6333"            // Address of the Qdrant HTTP API
	collectionName          = "chat_history"
)

type TextList struct {
	Texts []string `json:"texts"`
}

// Function to get embeddings from the embedding service
func getEmbeddings(texts []string) ([]float32, error) {
	log.Printf("Getting embeddings for %d texts", len(texts))

	jsonData, err := json.Marshal(TextList{Texts: texts})
	if err != nil {
		log.Printf("Error marshaling text data: %v", err)
		return nil, err
	}

	log.Printf("Sending request to embedding service at %s", embeddingServiceAddress)
	resp, err := http.Post(embeddingServiceAddress, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("Error connecting to embedding service: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	log.Printf("Received response from embedding service with status: %s", resp.Status)
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading response body: %v", err)
		return nil, err
	}

	// The embedding service returns a string containing a JSON array of arrays
	var embeddingString string
	err = json.Unmarshal(body, &embeddingString)
	if err != nil {
		log.Printf("Error unmarshaling embedding string: %v", err)
		return nil, err
	}

	// Parse the string as a JSON array of arrays of float32
	var embeddingList [][]float32
	err = json.Unmarshal([]byte(embeddingString), &embeddingList)
	if err != nil {
		log.Printf("Error unmarshaling embedding list: %v", err)
		return nil, err
	}

	// Make sure we have at least one embedding
	if len(embeddingList) == 0 {
		log.Printf("No embeddings returned from service")
		return nil, fmt.Errorf("no embeddings returned from service")
	}

	// Use the first embedding (corresponding to the first text)
	embeddings := embeddingList[0]
	log.Printf("Successfully generated embeddings of dimension %d", len(embeddings))
	return embeddings, nil
}

// Function to save a message to Qdrant using HTTP API
func saveToQdrant(messageID int64, text string, username string, embedding []float32) error {
	log.Printf("Saving message to Qdrant with ID: %d", messageID)

	// Qdrant saving logic using HTTP API
	qdrantURL := fmt.Sprintf("%s/collections/%s/points", qdrantServiceAddress, collectionName)
	log.Printf("Using Qdrant URL: %s", qdrantURL)

	// Convert float32 slice to interface{} slice for JSON marshaling
	embeddingInterface := make([]interface{}, len(embedding))
	for i, v := range embedding {
		embeddingInterface[i] = v
	}

	point := map[string]interface{}{
		"id": messageID,
		"vector": map[string]interface{}{
			"data": embeddingInterface,
		},
		"payload": map[string]string{
			"text":     text,
			"username": username,
		},
	}

	requestBody, err := json.Marshal(map[string][]map[string]interface{}{
		"points": {point},
	})
	if err != nil {
		log.Printf("Error marshaling point data: %v", err)
		return err
	}

	req, err := http.NewRequest(http.MethodPut, qdrantURL, bytes.NewBuffer(requestBody))
	if err != nil {
		log.Printf("Error creating HTTP request: %v", err)
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error sending HTTP request: %v", err)
		return err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading response body: %v", err)
		return err
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("Error response from Qdrant: %s", string(respBody))
		return fmt.Errorf("error response from Qdrant: %s", string(respBody))
	}

	log.Printf("Successfully saved message to Qdrant with ID: %d", messageID)
	return nil
}

// Function to search for similar messages in Qdrant using HTTP API
func searchQdrant(embedding []float32, limit int) ([]map[string]interface{}, error) {
	log.Printf("Searching Qdrant for similar messages with limit: %d", limit)

	// Qdrant search logic using HTTP API
	qdrantURL := fmt.Sprintf("%s/collections/%s/points/search", qdrantServiceAddress, collectionName)
	log.Printf("Using Qdrant URL: %s", qdrantURL)

	// Convert float32 slice to interface{} slice for JSON marshaling
	embeddingInterface := make([]interface{}, len(embedding))
	for i, v := range embedding {
		embeddingInterface[i] = v
	}

	requestBody, err := json.Marshal(map[string]interface{}{
		"vector": map[string]interface{}{
			"data": embeddingInterface,
		},
		"limit": limit,
	})
	if err != nil {
		log.Printf("Error marshaling search request: %v", err)
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, qdrantURL, bytes.NewBuffer(requestBody))
	if err != nil {
		log.Printf("Error creating HTTP request: %v", err)
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error sending HTTP request: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading response body: %v", err)
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("Error response from Qdrant: %s", string(respBody))
		return nil, fmt.Errorf("error response from Qdrant: %s", string(respBody))
	}

	// Parse the response
	var searchResult map[string]interface{}
	err = json.Unmarshal(respBody, &searchResult)
	if err != nil {
		log.Printf("Error unmarshaling search result: %v", err)
		return nil, err
	}

	// Extract the result array
	resultArray, ok := searchResult["result"].([]interface{})
	if !ok {
		log.Printf("Error: result field is not an array")
		return nil, fmt.Errorf("result field is not an array")
	}

	// Convert to a more usable format
	results := make([]map[string]interface{}, len(resultArray))
	for i, r := range resultArray {
		result, ok := r.(map[string]interface{})
		if !ok {
			log.Printf("Error: result item is not a map")
			return nil, fmt.Errorf("result item is not a map")
		}
		results[i] = result
	}

	log.Printf("Found %d results in Qdrant", len(results))
	return results, nil
}

// Function to check if a collection exists and create it if it doesn't
func createQdrantCollection(collectionName string) error {
	log.Printf("Checking if collection '%s' exists...", collectionName)

	// Check if collection exists
	qdrantURL := fmt.Sprintf("%s/collections/%s", qdrantServiceAddress, collectionName)
	resp, err := http.Get(qdrantURL)
	if err != nil {
		log.Printf("Error checking if collection exists: %v", err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		log.Printf("Collection '%s' already exists", collectionName)
		return nil
	}

	log.Printf("Collection '%s' does not exist, creating...", collectionName)

	// Create collection
	requestBody, err := json.Marshal(map[string]interface{}{
		"vectors_config": map[string]interface{}{
			"size":     384, // Embedding size
			"distance": "Cosine",
		},
	})
	if err != nil {
		log.Printf("Error marshaling collection creation request: %v", err)
		return err
	}

	req, err := http.NewRequest(http.MethodPut, qdrantURL, bytes.NewBuffer(requestBody))
	if err != nil {
		log.Printf("Error creating HTTP request: %v", err)
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err = client.Do(req)
	if err != nil {
		log.Printf("Error sending HTTP request: %v", err)
		return err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading response body: %v", err)
		return err
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("Error response from Qdrant: %s", string(respBody))
		return fmt.Errorf("error response from Qdrant: %s", string(respBody))
	}

	log.Printf("Collection '%s' created successfully", collectionName)
	return nil
}

func main() {
	log.Println("Starting Telegram RAG bot...")

	// Telegram Bot Token
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN environment variable is required")
	}
	log.Println("Telegram token found")

	// Create Qdrant collection if it doesn't exist
	err := createQdrantCollection(collectionName)
	if err != nil {
		log.Fatalf("Failed to create/check Qdrant collection: %v", err)
	}

	// Telebot settings
	log.Println("Configuring Telegram bot...")
	pref := tele.Settings{
		Token:  token,
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	}

	// Create new bot
	log.Println("Creating Telegram bot instance...")
	b, err := tele.NewBot(pref)
	if err != nil {
		log.Fatalf("Failed to create Telegram bot: %v", err)
		return
	}
	log.Printf("Telegram bot created successfully. Bot username: @%s", b.Me.Username)

	// Graceful shutdown
	log.Println("Setting up graceful shutdown...")
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	log.Println("Graceful shutdown configured")

	// Message handler
	log.Println("Setting up message handler...")
	b.Handle(tele.OnText, func(c tele.Context) error {
		log.Printf("Received message: '%s' from %s", c.Text(), c.Sender().Username)

		// Check if the bot is mentioned
		if strings.Contains(c.Text(), "@"+b.Me.Username) {
			log.Println("Bot was mentioned, processing as a query...")
			// Extract the query from the message
			query := strings.ReplaceAll(c.Text(), "@"+b.Me.Username, "")
			query = strings.TrimSpace(query)
			log.Printf("Extracted query: '%s'", query)

			// Get embedding for the query
			log.Println("Getting embeddings for query...")
			queryEmbeddings, err := getEmbeddings([]string{query})
			if err != nil {
				log.Printf("Error getting embedding for query: %v", err)
				return c.Send("Error processing your query")
			}
			log.Println("Embeddings generated successfully")

			// Search the vector database
			log.Println("Searching vector database for similar messages...")
			searchResults, err := searchQdrant(queryEmbeddings, 5)
			if err != nil {
				log.Printf("Error searching vector database: %v", err)
				return c.Send("Error processing your query")
			}
			log.Printf("Found %d results in vector database", len(searchResults))

			// Construct the answer from the search results
			log.Println("Constructing answer from search results...")
			var answer strings.Builder
			answer.WriteString("Here are some relevant messages:\n")
			for i, result := range searchResults {
				payload, ok := result["payload"].(map[string]interface{})
				if !ok {
					log.Printf("Error: payload is not a map")
					continue
				}
				text, ok := payload["text"].(string)
				if !ok {
					log.Printf("Error: text is not a string")
					continue
				}
				score, ok := result["score"].(float64)
				if !ok {
					log.Printf("Error: score is not a float64")
					score = 0
				}
				log.Printf("Result %d: Score=%f, Text='%s'", i+1, score, text)
				answer.WriteString(fmt.Sprintf("- %s\n", text))
			}

			// Send the answer
			log.Println("Sending answer to user...")
			return c.Send(answer.String())
		}

		// Calculate embedding for the message
		log.Println("Processing regular message for storage...")
		log.Println("Generating embeddings for message...")
		embeddings, err := getEmbeddings([]string{c.Text()})
		if err != nil {
			log.Printf("Error getting embedding: %v", err)
			return nil // Don't return an error to the user for background processing
		}
		log.Println("Embeddings generated successfully")

		// Store the message and its embedding in the vector database
		log.Println("Storing message in vector database...")
		id := time.Now().UnixNano()
		err = saveToQdrant(id, c.Text(), c.Sender().Username, embeddings)
		if err != nil {
			log.Printf("Error adding to vector database: %v", err)
			return nil
		}
		log.Printf("Message stored successfully with ID: %d", id)

		return nil
	})
	log.Println("Message handler configured")

	// Start the bot
	log.Println("Starting the Telegram bot...")
	go func() {
		log.Println("Bot is now running and listening for messages")
		b.Start()
	}()

	log.Println("Bot is running in the background. Press Ctrl+C to stop.")

	// Wait for shutdown signal
	<-ctx.Done()

	// Shutdown the bot
	log.Println("Shutdown signal received, stopping the bot...")
	b.Stop()
	log.Println("Telegram bot stopped successfully")
	log.Println("Goodbye!")
}
