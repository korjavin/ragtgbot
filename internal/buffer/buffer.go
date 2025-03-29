package buffer

import (
	"fmt"
	"sync"
)

// MessageBuffer stores messages until they're ready for processing
type MessageBuffer struct {
	Text     string
	Username string
	Size     int
	mutex    sync.Mutex
}

// NewMessageBuffer creates a new MessageBuffer
func NewMessageBuffer() *MessageBuffer {
	return &MessageBuffer{}
}

// Add adds a message to the buffer
func (b *MessageBuffer) Add(username, text string) {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	if b.Text == "" {
		b.Username = username // Set username from first message
		b.Text = fmt.Sprintf("%s: %s", username, text)
	} else {
		b.Text += fmt.Sprintf("\n%s: %s", username, text)
	}
	b.Size += len(text)
}

// Clear resets the buffer
func (b *MessageBuffer) Clear() {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	b.Text = ""
	b.Username = ""
	b.Size = 0
}

// IsEmpty returns true if the buffer is empty
func (b *MessageBuffer) IsEmpty() bool {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	return b.Size == 0
}

// GetContents returns the buffer contents in a thread-safe way
func (b *MessageBuffer) GetContents() (string, string, int) {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	return b.Text, b.Username, b.Size
}
