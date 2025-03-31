package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
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

// GetText extracts text from the message, handling plain strings and mixed arrays.
func (m *Message) GetText() (string, error) {
	if len(m.Text) == 0 {
		return "", nil
	}

	// Trim leading/trailing whitespace (like quotes) before trying to unmarshal
	trimmedText := bytes.TrimSpace(m.Text)
	if len(trimmedText) == 0 {
		return "", nil
	}

	// Handle plain string case first (most common)
	// Check if it looks like a JSON string (starts and ends with quotes)
	if trimmedText[0] == '"' && trimmedText[len(trimmedText)-1] == '"' {
		var textStr string
		// Use Unmarshal on the trimmed text which should be a valid JSON string
		err := json.Unmarshal(trimmedText, &textStr)
		if err == nil {
			return textStr, nil
		}
		// If unmarshal fails even for quoted string, log or proceed?
		// Let's proceed to array parsing attempt, as the error might be misleading.
		// fmt.Printf("Warning: Failed to unmarshal seemingly plain string (ID: %d): %v, raw: %s\n", m.ID, err, string(m.Text))
	}

	// Try to unmarshal as an array of interfaces for mixed types
	var textParts []interface{}
	err := json.Unmarshal(trimmedText, &textParts)
	if err != nil {
		// If it's not a string (or failed string parse) and not an array, return error.
		// It might be a simple unquoted string or other format not handled here.
		// Let's try returning the raw text directly if it doesn't start with '['
		if trimmedText[0] != '[' {
			// Attempt to return the raw text, assuming it might be an unquoted literal string
			// This is a fallback, might need refinement based on actual data variations.
			return string(m.Text), nil // Return original raw text, not trimmed
		}
		// Otherwise, it failed to parse as an array, return the error.
		return "", fmt.Errorf("failed to parse text field (ID: %d) as string or array: %v, raw text: %s",
			m.ID, err, string(m.Text))
	}

	var result strings.Builder // Use strings.Builder for efficiency
	for _, part := range textParts {
		switch v := part.(type) {
		case string:
			result.WriteString(v)
		case map[string]interface{}:
			// Check if it's a text entity (like link, bold, etc.) with a "text" field
			if textVal, ok := v["text"]; ok {
				if textStr, isString := textVal.(string); isString {
					result.WriteString(textStr)
				}
				// else: text value exists but is not a string, ignore.
			}
			// else: map doesn't contain "text" key (e.g., could be other entity types), ignore.
		default:
			// Ignore other types within the array (e.g., numbers, booleans)
		}
	}

	return result.String(), nil
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
