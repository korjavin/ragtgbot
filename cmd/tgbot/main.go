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
	"strconv"
	"strings"
	"syscall"
	"time"

	tele "gopkg.in/telebot.v3"
)

// OpenAI API types
type OpenAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type OpenAIChatRequest struct {
	Model    string          `json:"model"`
	Messages []OpenAIMessage `json:"messages"`
}

type OpenAIChatResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

const (
	defaultEmbeddingServiceAddress = "http://localhost:8000/embeddings" // Default address of the embedding service
	defaultQdrantServiceAddress    = "http://localhost:6333"            // Default address of the Qdrant HTTP API
	collectionName                 = "chat_history"
	openaiAPIURL                   = "https://api.openai.com/v1/chat/completions" // OpenAI API URL
	openaiModel                    = "gpt-4o-mini"                                // OpenAI model to use
	vectorSearchLimit              = 5                                            // Number of similar messages to retrieve
	restrictedAccessMessage        = "Sorry, this bot is restricted to answer outside of specific groups, but it's open-source and self-hosted, you can always host your own instance of it at https://github.com/korjavin/ragtgbot"
)

// Global variables for service addresses
var (
	embeddingServiceAddress string
	qdrantServiceAddress    string
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

	searchRequest := map[string]interface{}{
		"vector": map[string]interface{}{
			"name":   "data",
			"vector": embeddingInterface,
		},
		"limit":        limit,
		"with_payload": true,
	}

	requestBody, err := json.Marshal(searchRequest)
	if err != nil {
		log.Printf("Error marshaling search request: %v", err)
		return nil, err
	}

	// Log the request body for debugging
	log.Printf("Search request body: %s", string(requestBody))

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

// Function to get collection info
func getCollectionInfo(collectionName string) (map[string]interface{}, error) {
	log.Printf("Getting info for collection '%s'...", collectionName)

	// Get collection info
	qdrantURL := fmt.Sprintf("%s/collections/%s", qdrantServiceAddress, collectionName)
	resp, err := http.Get(qdrantURL)
	if err != nil {
		log.Printf("Error getting collection info: %v", err)
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
	var collectionInfo map[string]interface{}
	err = json.Unmarshal(respBody, &collectionInfo)
	if err != nil {
		log.Printf("Error unmarshaling collection info: %v", err)
		return nil, err
	}

	// Log the collection info
	infoBytes, _ := json.MarshalIndent(collectionInfo, "", "  ")
	log.Printf("Collection info: %s", string(infoBytes))

	return collectionInfo, nil
}

// Function to delete a collection
func deleteQdrantCollection(collectionName string) error {
	log.Printf("Deleting collection '%s'...", collectionName)

	// Delete collection
	qdrantURL := fmt.Sprintf("%s/collections/%s", qdrantServiceAddress, collectionName)
	req, err := http.NewRequest(http.MethodDelete, qdrantURL, nil)
	if err != nil {
		log.Printf("Error creating HTTP request: %v", err)
		return err
	}

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

	log.Printf("Collection '%s' deleted successfully", collectionName)
	return nil
}

// Function to call OpenAI API to generate an answer based on similar messages
func generateOpenAIAnswer(userQuestion string, similarMessages []map[string]interface{}) (string, error) {
	log.Printf("Generating answer with OpenAI for question: '%s'", userQuestion)

	// Get OpenAI API key from environment
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Printf("Error: OPENAI_API_KEY environment variable is not set")
		return "", fmt.Errorf("OpenAI API key is not configured")
	}

	// Format similar messages into snippets
	var snippets []string
	for _, result := range similarMessages {
		payload, ok := result["payload"].(map[string]interface{})
		if !ok {
			log.Printf("Warning: payload is not a map, skipping")
			continue
		}

		text, ok := payload["text"].(string)
		if !ok {
			log.Printf("Warning: text is not a string, skipping")
			continue
		}

		username, ok := payload["username"].(string)
		if !ok {
			username = "Unknown"
		}

		// Format the snippet with username
		snippet := fmt.Sprintf("%s: %s", username, text)
		snippets = append(snippets, snippet)
	}

	log.Printf("Constructed %d snippets from similar messages", len(snippets))

	// Construct the prompt
	prompt := "Using the following chat snippets, answer the question.\n\n" +
		strings.Join(snippets, "\n") + "\n\nQuestion: " + userQuestion + "\nAnswer:"

	log.Printf("Constructed prompt for OpenAI (length: %d characters)", len(prompt))

	// Prepare the request to OpenAI
	messages := []OpenAIMessage{
		{
			Role:    "user",
			Content: prompt,
		},
	}

	requestBody := OpenAIChatRequest{
		Model:    openaiModel,
		Messages: messages,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		log.Printf("Error marshaling OpenAI request: %v", err)
		return "", err
	}

	// Create the HTTP request
	req, err := http.NewRequest(http.MethodPost, openaiAPIURL, bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("Error creating OpenAI HTTP request: %v", err)
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	// Send the request
	log.Printf("Sending request to OpenAI API...")
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error sending request to OpenAI: %v", err)
		return "", err
	}
	defer resp.Body.Close()

	// Read the response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading OpenAI response: %v", err)
		return "", err
	}

	// Check for errors
	if resp.StatusCode != http.StatusOK {
		log.Printf("Error response from OpenAI (status %d): %s", resp.StatusCode, string(respBody))
		return "", fmt.Errorf("OpenAI API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	// Parse the response
	var openaiResp OpenAIChatResponse
	err = json.Unmarshal(respBody, &openaiResp)
	if err != nil {
		log.Printf("Error unmarshaling OpenAI response: %v", err)
		return "", err
	}

	// Extract the answer
	if len(openaiResp.Choices) == 0 {
		log.Printf("Error: OpenAI response contains no choices")
		return "", fmt.Errorf("OpenAI response contains no choices")
	}

	answer := openaiResp.Choices[0].Message.Content
	log.Printf("Successfully generated answer from OpenAI (length: %d characters)", len(answer))

	return answer, nil
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

		// Get collection info
		collectionInfo, err := getCollectionInfo(collectionName)
		if err != nil {
			log.Printf("Error getting collection info: %v", err)
		}

		// Check if the collection has vectors configured
		result, ok := collectionInfo["result"].(map[string]interface{})
		if !ok {
			log.Printf("Error: result field is not a map")
		} else {
			config, ok := result["config"].(map[string]interface{})
			if !ok {
				log.Printf("Error: config field is not a map")
			} else {
				params, ok := config["params"].(map[string]interface{})
				if !ok {
					log.Printf("Error: params field is not a map")
				} else {
					vectors, ok := params["vectors"].(map[string]interface{})
					if !ok || len(vectors) == 0 {
						log.Printf("Vectors are not configured in this collection, recreating...")

						// Delete the collection
						err = deleteQdrantCollection(collectionName)
						if err != nil {
							log.Printf("Error deleting collection: %v", err)
							return err
						}
					} else {
						log.Printf("Vectors configuration: %v", vectors)

						// Check if the vectors configuration has a "data" field
						_, hasDataVector := vectors["data"]
						if !hasDataVector {
							log.Printf("Vector with name 'data' is not configured in this collection, recreating...")

							// Delete the collection
							err = deleteQdrantCollection(collectionName)
							if err != nil {
								log.Printf("Error deleting collection: %v", err)
								return err
							}
						} else {
							return nil
						}
					}
				}
			}
		}
	}

	log.Printf("Collection '%s' does not exist, creating...", collectionName)

	// Create collection
	requestBody, err := json.Marshal(map[string]interface{}{
		"vectors": map[string]interface{}{
			"data": map[string]interface{}{
				"size":     384, // Embedding size
				"distance": "Cosine",
			},
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

// Function to check if a chat is allowed
func isAllowedChat(chatID int64, allowedGroups []int64) bool {
	// If no restrictions set, allow all
	if len(allowedGroups) == 0 {
		return true
	}

	for _, groupID := range allowedGroups {
		if chatID == groupID {
			return true
		}
	}
	return false
}

func main() {
	log.Println("Starting Telegram RAG bot...")

	// Telegram Bot Token
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN environment variable is required")
	}
	log.Println("Telegram token found")

	// Check for OpenAI API key
	openaiKey := os.Getenv("OPENAI_API_KEY")
	if openaiKey == "" {
		log.Fatal("OPENAI_API_KEY environment variable is required")
	}
	log.Println("OpenAI API key found")

	// Set service addresses from environment variables or use defaults
	embeddingServiceAddress = os.Getenv("EMBEDDING_SERVICE_ADDRESS")
	if embeddingServiceAddress == "" {
		embeddingServiceAddress = defaultEmbeddingServiceAddress
		log.Printf("EMBEDDING_SERVICE_ADDRESS not set, using default: %s", embeddingServiceAddress)
	} else {
		log.Printf("Using embedding service at: %s", embeddingServiceAddress)
	}

	qdrantServiceAddress = os.Getenv("QDRANT_SERVICE_ADDRESS")
	if qdrantServiceAddress == "" {
		qdrantServiceAddress = defaultQdrantServiceAddress
		log.Printf("QDRANT_SERVICE_ADDRESS not set, using default: %s", qdrantServiceAddress)
	} else {
		log.Printf("Using Qdrant service at: %s", qdrantServiceAddress)
	}

	// Parse allowed groups
	var allowedGroups []int64
	if groupsList := os.Getenv("TG_GROUP_LIST"); groupsList != "" {
		groups := strings.Split(groupsList, ",")
		for _, group := range groups {
			if groupID, err := strconv.ParseInt(strings.TrimSpace(group), 10, 64); err == nil {
				allowedGroups = append(allowedGroups, groupID)
				log.Printf("Added allowed group ID: %d", groupID)
			} else {
				log.Printf("Warning: invalid group ID in TG_GROUP_LIST: %s", group)
			}
		}
		log.Printf("Restricted to %d groups", len(allowedGroups))
	} else {
		log.Println("No group restrictions set, bot will respond in all chats")
	}

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
		log.Printf("Received message in chat %d: '%s' from %s", c.Chat().ID, c.Text(), c.Sender().Username)

		// Check if this chat is allowed
		if !isAllowedChat(c.Chat().ID, allowedGroups) {
			log.Printf("Message from restricted chat %d, ignoring", c.Chat().ID)
			if strings.Contains(c.Text(), "@"+b.Me.Username) {
				return c.Send(restrictedAccessMessage)
			}
			return nil
		}

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

			// Search the vector database for top similar messages
			log.Println("Searching vector database for similar messages...")
			searchResults, err := searchQdrant(queryEmbeddings, vectorSearchLimit)
			if err != nil {
				log.Printf("Error searching vector database: %v", err)
				return c.Send("Error processing your query")
			}
			log.Printf("Found %d results in vector database", len(searchResults))

			// Generate answer using OpenAI
			log.Println("Generating answer using OpenAI...")
			aiAnswer, err := generateOpenAIAnswer(query, searchResults)

			// Prepare the response with both AI answer and relevant messages
			var fullResponse strings.Builder

			// Add AI-generated answer if available
			if err != nil {
				log.Printf("Error generating answer with OpenAI: %v", err)
				fullResponse.WriteString("I couldn't generate an AI answer due to an error.\n\n")
			} else {
				log.Println("Successfully generated AI answer")
				fullResponse.WriteString(aiAnswer)
				fullResponse.WriteString("\n\n")
			}

			// Add top 5 relevant messages
			fullResponse.WriteString("Here are some relevant messages:\n")
			messageCount := 0
			for i, result := range searchResults {
				if i >= 1 { // Limit to top 5 results
					break
				}

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

				username, ok := payload["username"].(string)
				if !ok {
					username = "Unknown"
				}

				score, ok := result["score"].(float64)
				if !ok {
					score = 0
				}

				log.Printf("Result %d: Score=%f, User=%s, Text='%s'", i+1, score, username, text)

				// Truncate text to first 50 characters if longer
				displayText := text
				if len(displayText) > 50 {
					displayText = displayText[:50] + "..."
				}

				fullResponse.WriteString(fmt.Sprintf("- %s: %s\n", username, displayText))
				messageCount++
			}

			if messageCount == 0 {
				fullResponse.WriteString("No relevant messages found.\n")
			}

			// Send the combined answer
			log.Println("Sending combined response to user...")
			return c.Send(fullResponse.String())
		}

		// Check if the message is from the bot itself
		if c.Sender().Username == b.Me.Username {
			log.Println("Ignoring bot's own message, not storing in vector database")
			return nil
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
