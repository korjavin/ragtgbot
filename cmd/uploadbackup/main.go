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

type TelegramBackup struct {
	Name     string    `json:"name"`
	Type     string    `json:"type"`
	ID       int64     `json:"id"`
	Messages []Message `json:"messages"`
}

type Message struct {
	ID           int64           `json:"id"`
	Type         string          `json:"type"`
	Date         string          `json:"date"`
	DateUnixtime string          `json:"date_unixtime"`
	From         string          `json:"from,omitempty"`
	FromID       string          `json:"from_id,omitempty"`
	Text         json.RawMessage `json:"text"`
	Actor        string          `json:"actor,omitempty"`
	ActorID      string          `json:"actor_id,omitempty"`
	Action       string          `json:"action,omitempty"`
}

// GetText extracts text from the message, handling both string and array formats
func (m *Message) GetText() (string, error) {
	// If Text is empty, return empty string
	if len(m.Text) == 0 {
		return "", nil
	}

	// Try to unmarshal as string first
	var textStr string
	err := json.Unmarshal(m.Text, &textStr)
	if err == nil {
		return textStr, nil
	}

	// Log the raw text for debugging
	fmt.Printf("Message ID %d has non-string text: %s\n", m.ID, string(m.Text))

	// Try to unmarshal as a generic JSON value to see what we're dealing with
	var rawValue interface{}
	if err := json.Unmarshal(m.Text, &rawValue); err == nil {
		fmt.Printf("Message ID %d text type: %T\n", m.ID, rawValue)
	}

	// If that fails, try to unmarshal as array of text entities
	var textArray []map[string]interface{}
	err = json.Unmarshal(m.Text, &textArray)
	if err != nil {
		return "", fmt.Errorf("failed to parse text field (ID: %d): %v, raw text: %s",
			m.ID, err, string(m.Text))
	}

	// Extract text from array with detailed logging
	var result string
	fmt.Printf("Message ID %d has text array with %d entities\n", m.ID, len(textArray))
	for i, entity := range textArray {
		fmt.Printf("  Entity %d keys: %v\n", i, getMapKeys(entity))
		if text, ok := entity["text"].(string); ok {
			result += text
			fmt.Printf("  Entity %d text: %s\n", i, text)
		} else if text, exists := entity["text"]; exists {
			fmt.Printf("  Entity %d has text of type %T: %v\n", i, text, text)
		}
	}
	return result, nil
}

// Helper function to get map keys
func getMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

type EmbeddingResponse struct {
	Embedding []float64 `json:"embedding"`
}

const (
	maxChunkSize = 3072 // Maximum characters in a chunk
)

func main() {
	// Get filename from arguments
	if len(os.Args) != 2 {
		fmt.Println("Usage: go run cmd/uploadbackup/main.go <filename>")
		return
	}
	filename := os.Args[1]

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

	// 3. Iterate through messages and extract data
	for _, message := range backup.Messages {
		if message.Type == "message" {
			// Extract text using our new method
			text, err := message.GetText()
			if err != nil {
				fmt.Printf("Error extracting text from message ID %d: %v\n", message.ID, err)
				continue
			}

			username := message.From
			lastMessageID = message.ID

			// Add message to buffer
			msgBuffer.Add(username, text)

			// Process buffer if it exceeds max size
			if msgBuffer.Size >= maxChunkSize {
				if err := processBuffer(msgBuffer, lastMessageID); err != nil {
					fmt.Printf("Error processing buffer at message ID %d: %v\n", lastMessageID, err)
				}
				msgBuffer.Clear()
			}
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
	qdrantURL := "http://localhost:6333/collections/chat_history/points"

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
	qdrantURL := fmt.Sprintf("http://localhost:6333/collections/%s", collectionName)

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
