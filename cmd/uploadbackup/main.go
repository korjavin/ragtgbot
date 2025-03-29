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
)

type TelegramBackup struct {
	Name     string    `json:"name"`
	Type     string    `json:"type"`
	ID       int64     `json:"id"`
	Messages []Message `json:"messages"`
}

type Message struct {
	ID           int64  `json:"id"`
	Type         string `json:"type"`
	Date         string `json:"date"`
	DateUnixtime string `json:"date_unixtime"`
	From         string `json:"from"`
	FromID       string `json:"from_id"`
	Text         string `json:"text"`
}

type EmbeddingResponse struct {
	Embedding []float64 `json:"embedding"`
}

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
		fmt.Println(err)
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

	// 3. Iterate through messages and extract data
	for _, message := range backup.Messages {
		if message.Type == "message" {
			text := message.Text
			username := message.From

			// 4. Call the embedding service
			embedding, err := getEmbedding(text)
			if err != nil {
				fmt.Println(err)
				continue
			}

			// 5. Save to Qdrant
			err = saveToQdrant(message.ID, text, username, embedding)
			if err != nil {
				fmt.Println(err)
				continue
			}

			//fmt.Printf("Saving to Qdrant: User=%s: %v\n", username, embedding)
			//fmt.Printf("Saved message from %s: %s\n", username, text)
		}
		bar.Increment()
	}

	fmt.Println("Finished processing Telegram backup")
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
