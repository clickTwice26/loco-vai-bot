package memory

import (
	"context"
	"sync"
)

// InMemoryStore is an in-process, thread-safe memory store used when Redis is not configured.
type InMemoryStore struct {
	mu         sync.RWMutex
	histories  map[string][]Message
	maxHistory int
}

// NewInMemoryStore creates a new in-memory store.
func NewInMemoryStore(maxHistory int) *InMemoryStore {
	return &InMemoryStore{
		histories:  make(map[string][]Message),
		maxHistory: maxHistory,
	}
}

// GetHistory returns a copy of stored messages for a channel.
func (m *InMemoryStore) GetHistory(ctx context.Context, channelID string) ([]Message, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	history, exists := m.histories[channelID]
	if !exists {
		return []Message{}, nil
	}

	result := make([]Message, len(history))
	copy(result, history)
	return result, nil
}

// AppendMessage appends a message and trims the slice to maxHistory.
func (m *InMemoryStore) AppendMessage(ctx context.Context, channelID string, msg Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	history := m.histories[channelID]
	history = append(history, msg)

	if len(history) > m.maxHistory {
		history = history[len(history)-m.maxHistory:]
	}

	m.histories[channelID] = history
	return nil
}

// ClearHistory clears the history for a channel.
func (m *InMemoryStore) ClearHistory(ctx context.Context, channelID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.histories, channelID)
	return nil
}

// Close is a no-op for in-memory store.
func (m *InMemoryStore) Close() error {
	return nil
}
