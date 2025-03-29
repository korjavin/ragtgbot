package main

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/korjavin/ragtgbot/internal/buffer"
	"github.com/stretchr/testify/assert"
)

// createTestMessage creates a test message with given ID, text, and timestamp
func createTestMessage(id int64, text string, timestamp int64) Message {
	textJSON, _ := json.Marshal(text)
	return Message{
		ID:           id,
		Type:         "message",
		Date:         time.Unix(timestamp, 0).Format(time.RFC3339),
		DateUnixtime: fmt.Sprintf("%d", timestamp),
		From:         "user",
		Text:         textJSON,
	}
}

// setupTestBuffer creates a function that simulates processing messages through the buffer
func setupTestBuffer(t *testing.T) func([]Message) []int64 {
	return func(messages []Message) []int64 {
		var processedChunks []int64
		msgBuffer := buffer.NewMessageBuffer()
		var lastMessageID int64 = 0
		var lastTimestamp int64 = 0

		// Mock processBuffer function
		processBufferFn := func(buf *buffer.MessageBuffer, msgID int64) error {
			processedChunks = append(processedChunks, msgID)
			return nil
		}

		for _, message := range messages {
			if message.Type == "message" {
				text, err := message.GetText()
				assert.NoError(t, err)

				// Skip messages without text
				if text == "" {
					continue
				}

				// Get message timestamp before processing
				currentTimestamp, err := parseTimestamp(message.DateUnixtime)
				assert.NoError(t, err)

				// Calculate potential new buffer size if we add this message
				potentialSize := msgBuffer.Size + len(text)

				// Check if adding this message would exceed limits
				if !msgBuffer.IsEmpty() {
					timeProximity := true
					if lastTimestamp > 0 && currentTimestamp > 0 {
						timeProximity = (currentTimestamp - lastTimestamp) <= timeProximityLimit
					}

					// Process buffer if:
					// 1. Adding this message would exceed hard limit, or
					// 2. Adding this message would exceed soft limit AND messages are not close in time
					if potentialSize >= hardLimitChunkSize ||
						(potentialSize >= softLimitChunkSize && !timeProximity) {
						processBufferFn(msgBuffer, lastMessageID)
						msgBuffer.Clear()
					}
				}

				// Add message to buffer and update tracking variables
				msgBuffer.Add(message.From, text)
				lastMessageID = message.ID
				lastTimestamp = currentTimestamp
			}
		}

		// Process remaining messages in buffer
		if !msgBuffer.IsEmpty() {
			processBufferFn(msgBuffer, lastMessageID)
		}

		return processedChunks
	}
}

// createString generates a string of specified length
func createString(length int) string {
	const chars = "abcdefghijklmnopqrstuvwxyz "
	result := make([]byte, length)
	for i := range result {
		result[i] = chars[i%len(chars)]
	}
	return string(result)
}

func TestSoftLimitChunking(t *testing.T) {
	processMessages := setupTestBuffer(t)

	// Create messages with text sizes just below and above the soft limit
	// All messages have timestamps within the proximity limit
	baseTime := time.Now().Unix()
	messages := []Message{
		createTestMessage(1, createString(500), baseTime),
		createTestMessage(2, createString(500), baseTime+10),
		createTestMessage(3, createString(500), baseTime+20),
		createTestMessage(4, createString(500), baseTime+30),
		// This message makes the buffer exceed the soft limit (2000)
		createTestMessage(5, createString(500), baseTime+40),
		createTestMessage(6, createString(500), baseTime+50),
	}

	// All messages are within time proximity, so chunking should happen
	// only based on the hard limit
	processedChunks := processMessages(messages)

	// Since we're under hard limit and all messages are within time proximity,
	// we should only have one chunk (the final one)
	assert.Equal(t, 1, len(processedChunks), "Expected only one chunk due to soft limit with time proximity")
	assert.Equal(t, int64(6), processedChunks[0], "Expected last message ID to be 6")
}

func TestHardLimitChunking(t *testing.T) {
	processMessages := setupTestBuffer(t)

	// Create messages that will exceed the hard limit
	baseTime := time.Now().Unix()
	messages := []Message{
		createTestMessage(1, createString(1000), baseTime),
		createTestMessage(2, createString(1000), baseTime+10),
		createTestMessage(3, createString(1000), baseTime+20),
		// This message makes the buffer exceed the hard limit (4000)
		createTestMessage(4, createString(1500), baseTime+30),
		createTestMessage(5, createString(500), baseTime+40),
	}

	processedChunks := processMessages(messages)

	// Should have two chunks: one when hard limit is hit, and one for the remaining
	assert.Equal(t, 2, len(processedChunks), "Expected two chunks due to hard limit")
	assert.Equal(t, int64(3), processedChunks[0], "First chunk should end at message 3")
	assert.Equal(t, int64(5), processedChunks[1], "Second chunk should end at message 5")
}

func TestTimeProximityChunking(t *testing.T) {
	processMessages := setupTestBuffer(t)

	// Create messages where some exceed the time proximity threshold
	baseTime := time.Now().Unix()
	messages := []Message{
		createTestMessage(1, createString(500), baseTime),
		createTestMessage(2, createString(500), baseTime+10),
		createTestMessage(3, createString(500), baseTime+20),
		createTestMessage(4, createString(700), baseTime+30),
		// This message is over an hour later
		createTestMessage(5, createString(700), baseTime+timeProximityLimit+100),
		createTestMessage(6, createString(500), baseTime+timeProximityLimit+200),
	}

	processedChunks := processMessages(messages)

	// Should have two chunks: one when time proximity is broken after soft limit,
	// and one for the remaining messages
	assert.Equal(t, 2, len(processedChunks), "Expected two chunks due to time proximity")
	assert.Equal(t, int64(4), processedChunks[0], "First chunk should end at message 4")
	assert.Equal(t, int64(6), processedChunks[1], "Second chunk should end at message 6")
}

func TestEmptyMessageSkipping(t *testing.T) {
	processMessages := setupTestBuffer(t)

	baseTime := time.Now().Unix()
	messages := []Message{
		createTestMessage(1, createString(500), baseTime),
		createTestMessage(2, "", baseTime+10), // Empty message, should be skipped
		createTestMessage(3, createString(500), baseTime+20),
		createTestMessage(4, "", baseTime+30), // Empty message, should be skipped
		createTestMessage(5, createString(500), baseTime+40),
	}

	processedChunks := processMessages(messages)

	// Only one chunk with the non-empty messages
	assert.Equal(t, 1, len(processedChunks), "Expected one chunk with non-empty messages")
	assert.Equal(t, int64(5), processedChunks[0], "Chunk should end with last non-empty message")
}

func TestCombinedConditions(t *testing.T) {
	processMessages := setupTestBuffer(t)

	baseTime := time.Now().Unix()
	messages := []Message{
		// First chunk: exceeds soft limit and time proximity broken
		createTestMessage(1, createString(1000), baseTime),
		createTestMessage(2, createString(1000), baseTime+10),
		createTestMessage(3, createString(500), baseTime+timeProximityLimit+100),

		// Second chunk: exceeds hard limit
		createTestMessage(4, createString(1500), baseTime+timeProximityLimit+200),
		createTestMessage(5, createString(1500), baseTime+timeProximityLimit+300),
		createTestMessage(6, createString(1500), baseTime+timeProximityLimit+400),

		// Third chunk: remaining messages
		createTestMessage(7, createString(500), baseTime+timeProximityLimit+500),
	}

	processedChunks := processMessages(messages)

	// Should have three chunks based on our processing logic
	assert.Equal(t, 3, len(processedChunks), "Expected three chunks from combined conditions")
	assert.Equal(t, int64(2), processedChunks[0], "First chunk due to soft limit + time proximity break")
	assert.Equal(t, int64(5), processedChunks[1], "Second chunk due to hard limit")
	assert.Equal(t, int64(7), processedChunks[2], "Third chunk for remaining messages")
}
