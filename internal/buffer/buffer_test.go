package buffer

import (
	"sync"
	"testing"
)

func TestNewMessageBuffer(t *testing.T) {
	buffer := NewMessageBuffer()
	if buffer == nil {
		t.Fatal("NewMessageBuffer returned nil")
	}
	if buffer.Text != "" {
		t.Errorf("New buffer should have empty text, got %q", buffer.Text)
	}
	if buffer.Username != "" {
		t.Errorf("New buffer should have empty username, got %q", buffer.Username)
	}
	if buffer.Size != 0 {
		t.Errorf("New buffer should have size 0, got %d", buffer.Size)
	}
}

func TestMessageBuffer_Add(t *testing.T) {
	buffer := NewMessageBuffer()

	// Test adding first message
	buffer.Add("user1", "hello")
	expectedText := "user1: hello"
	if buffer.Text != expectedText {
		t.Errorf("Buffer text = %q, want %q", buffer.Text, expectedText)
	}
	if buffer.Username != "user1" {
		t.Errorf("Buffer username = %q, want %q", buffer.Username, "user1")
	}
	if buffer.Size != 5 { // "hello" is 5 characters
		t.Errorf("Buffer size = %d, want %d", buffer.Size, 5)
	}

	// Test adding second message
	buffer.Add("user2", "world")
	expectedText = "user1: hello\nuser2: world"
	if buffer.Text != expectedText {
		t.Errorf("Buffer text = %q, want %q", buffer.Text, expectedText)
	}
	if buffer.Username != "user1" { // Username should still be from first message
		t.Errorf("Buffer username = %q, want %q", buffer.Username, "user1")
	}
	if buffer.Size != 10 { // "hello" + "world" = 10 characters
		t.Errorf("Buffer size = %d, want %d", buffer.Size, 10)
	}
}

func TestMessageBuffer_Clear(t *testing.T) {
	buffer := NewMessageBuffer()
	buffer.Add("user", "some text")

	buffer.Clear()
	if buffer.Text != "" {
		t.Errorf("Cleared buffer should have empty text, got %q", buffer.Text)
	}
	if buffer.Username != "" {
		t.Errorf("Cleared buffer should have empty username, got %q", buffer.Username)
	}
	if buffer.Size != 0 {
		t.Errorf("Cleared buffer should have size 0, got %d", buffer.Size)
	}
}

func TestMessageBuffer_IsEmpty(t *testing.T) {
	buffer := NewMessageBuffer()

	if !buffer.IsEmpty() {
		t.Error("New buffer should be empty")
	}

	buffer.Add("user", "text")
	if buffer.IsEmpty() {
		t.Error("Buffer with content should not be empty")
	}

	buffer.Clear()
	if !buffer.IsEmpty() {
		t.Error("Cleared buffer should be empty")
	}
}

func TestMessageBuffer_GetContents(t *testing.T) {
	buffer := NewMessageBuffer()
	buffer.Add("user", "hello world")

	text, username, size := buffer.GetContents()

	if text != "user: hello world" {
		t.Errorf("GetContents text = %q, want %q", text, "user: hello world")
	}
	if username != "user" {
		t.Errorf("GetContents username = %q, want %q", username, "user")
	}
	if size != 11 { // "hello world" is 11 characters
		t.Errorf("GetContents size = %d, want %d", size, 11)
	}
}

func TestMessageBuffer_Concurrency(t *testing.T) {
	buffer := NewMessageBuffer()
	const numGoroutines = 100
	const messagesPerGoroutine = 10

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < messagesPerGoroutine; j++ {
				buffer.Add("user", "a") // Add a single character each time
			}
		}(i)
	}

	wg.Wait()

	_, _, size := buffer.GetContents()
	expectedSize := numGoroutines * messagesPerGoroutine // 100 * 10 = 1000
	if size != expectedSize {
		t.Errorf("Buffer size after concurrent additions = %d, want %d", size, expectedSize)
	}
}
