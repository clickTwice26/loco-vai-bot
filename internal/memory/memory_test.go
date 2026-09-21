package memory

import (
	"context"
	"testing"
	"time"
)

func TestInMemoryStore(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryStore(3)

	channelID := "chan-123"

	// Initial check
	hist, err := store.GetHistory(ctx, channelID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hist) != 0 {
		t.Fatalf("expected empty history, got %d", len(hist))
	}

	// Add 4 messages (should trim to 3)
	for i := 1; i <= 4; i++ {
		err := store.AppendMessage(ctx, channelID, Message{
			Role:      "user",
			Author:    "User1",
			Content:   string(rune('A' + i - 1)),
			Timestamp: time.Now(),
		})
		if err != nil {
			t.Fatalf("failed to append: %v", err)
		}
	}

	hist, err = store.GetHistory(ctx, channelID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hist) != 3 {
		t.Fatalf("expected 3 messages after trim, got %d", len(hist))
	}
	if hist[0].Content != "B" || hist[1].Content != "C" || hist[2].Content != "D" {
		t.Errorf("unexpected trimmed contents: %+v", hist)
	}

	// Clear history
	if err := store.ClearHistory(ctx, channelID); err != nil {
		t.Fatalf("failed to clear: %v", err)
	}
	hist, _ = store.GetHistory(ctx, channelID)
	if len(hist) != 0 {
		t.Errorf("expected 0 messages after clear, got %d", len(hist))
	}
}
