package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/cheggaaa/pb/v3"
	"github.com/korjavin/ragtgbot/internal/buffer"
)

var qdrantBaseURL string // Base URL for Qdrant service

func main() {
	// Get filename from arguments
	if len(os.Args) != 2 {
		fmt.Println("Usage: go run cmd/uploadbackup/main.go <filename>")
		return
	}
	filename := os.Args[1]

	// Determine Qdrant URL from environment variable or use default
	qdrantAddr := os.Getenv("QDRANT_SERVICE_ADDRESS")
	if qdrantAddr != "" {
		qdrantBaseURL = qdrantAddr
		log.Printf("Using Qdrant address from env: %s", qdrantBaseURL)
	} else {
		qdrantBaseURL = "http://localhost:6333" // Default URL
		log.Printf("Using default Qdrant address: %s", qdrantBaseURL)
	}

	// 1. Read the JSON file
	jsonFile, err := os.Open(filename)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer jsonFile.Close()

	byteValue, _ := io.ReadAll(jsonFile)

	// 2. Parse the JSON
	var backup TelegramBackup
	err = json.Unmarshal(byteValue, &backup)
	if err != nil {
		fmt.Printf("Error unmarshaling JSON: %v\n", err)

		// Try to unmarshal into a map to see the structure
		var rawData map[string]interface{}
		if jsonErr := json.Unmarshal(byteValue, &rawData); jsonErr == nil {
			if messages, ok := rawData["messages"].([]interface{}); ok {
				// Find problematic messages
				for i, msg := range messages {
					if msgMap, ok := msg.(map[string]interface{}); ok {
						if text, exists := msgMap["text"]; exists {
							switch text.(type) {
							case string:
								// This is fine
							case []interface{}:
								fmt.Printf("Found array text at message index %d, ID: %v\n",
									i, msgMap["id"])
							default:
								fmt.Printf("Found unusual text type at message index %d, type: %T\n",
									i, text)
							}
						}
					}
				}
			}
		}

		return
	}

	// Create Qdrant collection if it doesn't exist
	err = createQdrantCollection("chat_history")
	if err != nil {
		fmt.Println(err)
		//return // Don't return, just log the error and continue
	}

	// Initialize progress bar
	bar := pb.StartNew(len(backup.Messages))
	defer bar.Finish()

	// Initialize message buffer
	msgBuffer := buffer.NewMessageBuffer()
	lastMessageID := int64(0)
	var lastTimestamp int64 = 0

	// 3. Iterate through messages and extract data
	for _, message := range backup.Messages {
		if message.Type == "message" {
			// Extract text using our new method
			text, err := message.GetText()
			if err != nil {
				fmt.Printf("Error extracting text from message ID %d: %v\n", message.ID, err)
				continue
			}

			// Skip messages without text
			if text == "" {
				bar.Increment()
				continue
			}

			username := message.From
			lastMessageID = message.ID

			// Parse message timestamp
			currentTimestamp, err := parseTimestamp(message.DateUnixtime)
			if err != nil {
				fmt.Printf("Error parsing timestamp for message ID %d: %v\n", message.ID, err)
				currentTimestamp = 0
			}

			// Process buffer based on size and time proximity
			if !msgBuffer.IsEmpty() {
				// Check if we need to process the buffer
				timeProximity := true
				if lastTimestamp > 0 && currentTimestamp > 0 {
					timeProximity = (currentTimestamp - lastTimestamp) <= timeProximityLimit
				}

				// Process buffer if:
				// 1. Buffer exceeds hard limit, or
				// 2. Buffer exceeds soft limit AND messages are not close in time
				if msgBuffer.Size >= hardLimitChunkSize ||
					(msgBuffer.Size >= softLimitChunkSize && !timeProximity) {
					if err := processBuffer(msgBuffer, lastMessageID); err != nil {
						fmt.Printf("Error processing buffer at message ID %d: %v\n", lastMessageID, err)
					}
					msgBuffer.Clear()
				}
			}

			// Add message to buffer
			msgBuffer.Add(username, text)
			lastTimestamp = currentTimestamp
		}
		bar.Increment()
	}

	// Process remaining messages in buffer
	if !msgBuffer.IsEmpty() {
		if err := processBuffer(msgBuffer, lastMessageID); err != nil {
			fmt.Printf("Error processing final buffer: %v\n", err)
		}
	}

	fmt.Println("Finished processing Telegram backup")
}

func processBuffer(buffer *buffer.MessageBuffer, messageID int64) error {
	// Get buffer contents
	text, username, _ := buffer.GetContents()

	// Get embedding for combined text
	embedding, err := getEmbedding(text)
	if err != nil {
		return fmt.Errorf("error getting embedding: %v", err)
	}

	// Save to Qdrant
	err = saveToQdrant(messageID, text, username, embedding)
	if err != nil {
		return fmt.Errorf("error saving to Qdrant: %v", err)
	}

	return nil
}

func getEmbedding(text string) ([]float64, error) {
	// Replace with your embedding service URL
	embeddingServiceURL := "http://localhost:8000/embeddings"

	requestBody, err := json.Marshal(map[string][]string{
		"texts": {text},
	})
	if err != nil {
		return nil, err
	}

	resp, err := http.Post(embeddingServiceURL, "application/json", bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var embeddingString string
	err = json.Unmarshal(body, &embeddingString)
	if err != nil {
		log.Println("Error unmarshaling embedding string:", err)
		return nil, err
	}

	var embeddingList [][]float64
	err = json.Unmarshal([]byte(embeddingString), &embeddingList)
	if err != nil {
		log.Println("Error unmarshaling embedding list:", err)
		return nil, err
	}

	if len(embeddingList) > 0 {
		return embeddingList[0], nil
	}

	return nil, fmt.Errorf("no embedding found")
}

func saveToQdrant(messageID int64, text string, username string, embedding []float64) error {
	// Qdrant saving logic using HTTP API
	qdrantURL := fmt.Sprintf("%s/collections/chat_history/points", qdrantBaseURL)

	point := map[string]interface{}{
		"id": messageID,
		"vector": map[string]interface{}{
			"data": embedding,
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
		return err
	}

	req, err := http.NewRequest(http.MethodPut, qdrantURL, bytes.NewBuffer(requestBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	_, err = io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	//log.Println(string(body)) // Print the response from Qdrant

	return nil
}

func createQdrantCollection(collectionName string) error {
	qdrantURL := fmt.Sprintf("%s/collections/%s", qdrantBaseURL, collectionName)

	// Check if collection exists
	resp, err := http.Get(qdrantURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		log.Printf("Collection %s already exists\n", collectionName)
		return nil
	}

	requestBody, err := json.Marshal(map[string]interface{}{
		"vectors_config": map[string]interface{}{
			"size":     384, // Embedding size from all-MiniLM-L6-v2
			"distance": "Cosine",
		},
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPut, qdrantURL, bytes.NewBuffer(requestBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err = client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	_, err = io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	//log.Println(string(body)) // Print the response from Qdrant

	return nil
}
