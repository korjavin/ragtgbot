package main

import (
	"encoding/json"
	"fmt"
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
	// fmt.Printf("Message ID %d has non-string text: %s\n", m.ID, string(m.Text)) // Keep commented or remove if not needed

	// Try to unmarshal as a generic JSON value to see what we're dealing with
	// var rawValue interface{}
	// if err := json.Unmarshal(m.Text, &rawValue); err == nil {
	// 	fmt.Printf("Message ID %d text type: %T\n", m.ID, rawValue) // Keep commented or remove if not needed
	// }

	// If that fails, try to unmarshal as array of text entities
	var textArray []map[string]interface{}
	err = json.Unmarshal(m.Text, &textArray)
	if err != nil {
		return "", fmt.Errorf("failed to parse text field (ID: %d): %v, raw text: %s",
			m.ID, err, string(m.Text))
	}

	// Extract text from array
	var result string
	// fmt.Printf("Message ID %d has text array with %d entities\n", m.ID, len(textArray)) // Keep commented or remove if not needed
	for _, entity := range textArray {
		// fmt.Printf("  Entity %d keys: %v\n", i, getMapKeys(entity)) // Keep commented or remove if not needed
		if text, ok := entity["text"].(string); ok {
			result += text
			// fmt.Printf("  Entity %d text: %s\n", i, text) // Keep commented or remove if not needed
		} // else if text, exists := entity["text"]; exists {
		// fmt.Printf("  Entity %d has text of type %T: %v\n", i, text, text) // Keep commented or remove if not needed
		// }
	}
	return result, nil
}

type EmbeddingResponse struct {
	Embedding []float64 `json:"embedding"`
}

const (
	maxChunkSize       = 3072     // Maximum characters in a chunk (old value, keeping for reference)
	softLimitChunkSize = 1000     // Soft limit for chunk size
	hardLimitChunkSize = 2000     // Hard limit for chunk size
	timeProximityLimit = 3600 * 2 // Time proximity limit in seconds (2 hours, corrected from 24)
)

// parseTimestamp converts a Unix timestamp string to int64
func parseTimestamp(timestampStr string) (int64, error) {
	var timestamp int64
	_, err := fmt.Sscanf(timestampStr, "%d", &timestamp)
	return timestamp, err
}
