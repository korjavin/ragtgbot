package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"testing"
	"time"
)

const (
	embeddingServiceAddress = "http://localhost:8000/embeddings" // Address of the embedding service
	qdrantServiceAddress    = "http://localhost:6333"            // Address of the Qdrant HTTP API
	testCollectionName      = "test_chat_history"                // Test collection name
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
func saveToQdrant(collectionName string, messageID int64, text string, username string, embedding []float32) error {
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
func searchQdrant(collectionName string, embedding []float32, limit int) ([]map[string]interface{}, error) {
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

	// Debug: Print the raw search result
	rawJSON, _ := json.MarshalIndent(searchResult, "", "  ")
	log.Printf("Raw search result: %s", string(rawJSON))

	// Extract the result array
	resultArray, ok := searchResult["result"].([]interface{})
	if !ok {
		log.Printf("Error: result field is not an array")
		return nil, fmt.Errorf("result field is not an array")
	}

	// Debug: Print the first result if available
	if len(resultArray) > 0 {
		firstResult, _ := json.MarshalIndent(resultArray[0], "", "  ")
		log.Printf("First result: %s", string(firstResult))
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

// Function to create a collection
func createQdrantCollection(collectionName string) error {
	log.Printf("Creating collection '%s'...", collectionName)

	// Create collection
	qdrantURL := fmt.Sprintf("%s/collections/%s", qdrantServiceAddress, collectionName)
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

	log.Printf("Collection '%s' created successfully", collectionName)
	return nil
}

// Integration test for saving and searching embeddings
func TestSaveAndSearchEmbeddings(t *testing.T) {
	// Skip this test if the SKIP_INTEGRATION_TESTS environment variable is set
	if os.Getenv("SKIP_INTEGRATION_TESTS") == "true" {
		t.Skip("Skipping integration test")
	}

	// Create a test collection
	err := createQdrantCollection(testCollectionName)
	if err != nil {
		t.Fatalf("Failed to create test collection: %v", err)
	}
	defer func() {
		// Clean up: delete the test collection
		err := deleteQdrantCollection(testCollectionName)
		if err != nil {
			t.Logf("Failed to delete test collection: %v", err)
		}
	}()

	// Test messages
	testMessages := []string{
		"test",
		"test 1",
		"test 2",
		"this is a completely different message",
	}

	// Save test messages
	for i, msg := range testMessages {
		// Get embeddings
		embedding, err := getEmbeddings([]string{msg})
		if err != nil {
			t.Fatalf("Failed to get embeddings for message %d: %v", i, err)
		}

		// Save to Qdrant
		id := time.Now().UnixNano() + int64(i)
		err = saveToQdrant(testCollectionName, id, msg, "test_user", embedding)
		if err != nil {
			t.Fatalf("Failed to save message %d to Qdrant: %v", i, err)
		}
	}

	// Wait a moment for Qdrant to process the points
	time.Sleep(1 * time.Second)

	// Search for "test"
	searchEmbedding, err := getEmbeddings([]string{"test"})
	if err != nil {
		t.Fatalf("Failed to get embeddings for search query: %v", err)
	}

	results, err := searchQdrant(testCollectionName, searchEmbedding, 5)
	if err != nil {
		t.Fatalf("Failed to search Qdrant: %v", err)
	}

	// Verify results
	if len(results) == 0 {
		t.Errorf("Expected to find results for 'test', but found none")
	} else {
		t.Logf("Found %d results for 'test'", len(results))

		// Debug: Print the structure of the first result
		if len(results) > 0 {
			for k, v := range results[0] {
				t.Logf("Result key: %s, type: %T", k, v)
			}
		}

		// Check that the top results contain "test"
		foundTestMessage := false
		for _, result := range results {
			// Try to extract payload in different ways
			var text string
			var found bool

			// Try direct access to payload.text
			if payload, ok := result["payload"].(map[string]interface{}); ok {
				if textVal, ok := payload["text"].(string); ok {
					text = textVal
					found = true
				}
			}

			// If not found, try to look for a document field
			if !found {
				if doc, ok := result["document"].(map[string]interface{}); ok {
					if textVal, ok := doc["text"].(string); ok {
						text = textVal
						found = true
					}
				}
			}

			// If still not found, try to look for a text field directly
			if !found {
				if textVal, ok := result["text"].(string); ok {
					text = textVal
					found = true
				}
			}

			if found && (text == "test" || text == "test 1" || text == "test 2") {
				foundTestMessage = true
				t.Logf("Found test message: %s", text)
			}
		}

		if !foundTestMessage {
			t.Errorf("Expected to find a test message in the results, but none was found")
		}
	}
}
