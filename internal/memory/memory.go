package memory

import (
	"context"
	"time"
)

// Message represents a single chat message preserved in the channel's memory context.
type Message struct {
	Role      string    `json:"role"`      // "user" or "model"
	Author    string    `json:"author"`    // Username or "Loco"
	Content   string    `json:"content"`   // The actual message text
	Timestamp time.Time `json:"timestamp"` // Time the message occurred
}

// Store defines the interface for persisting and retrieving channel chat history.
type Store interface {
	// GetHistory returns the most recent messages for a channel up to maxHistory.
	GetHistory(ctx context.Context, channelID string) ([]Message, error)
	// AppendMessage adds a new message to the channel's conversation history.
	AppendMessage(ctx context.Context, channelID string, msg Message) error
	// ClearHistory wipes the conversation history for a channel.
	ClearHistory(ctx context.Context, channelID string) error
	// Close cleanly closes any underlying connection.
	Close() error
}
