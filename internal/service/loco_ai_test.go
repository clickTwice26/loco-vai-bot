package service

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"localoy-bot/internal/memory"
)

type mockGeminiClient struct {
	response string
	err      error
}

func (m *mockGeminiClient) GenerateChatResponse(ctx context.Context, systemPrompt string, history []memory.Message, currentMessage memory.Message) (string, error) {
	return m.response, m.err
}

func TestLocoAIService(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	memStore := memory.NewInMemoryStore(10)
	geminiMock := &mockGeminiClient{response: "wassup bro"}

	svc := NewLocoAIService(geminiMock, memStore, "target-channel-123", 1.0, logger)

	ctx := context.Background()

	t.Run("Ignore non-target channel", func(t *testing.T) {
		reply, responded, err := svc.ProcessMessage(ctx, "other-channel", "alice", "hello", false, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if responded || reply != "" {
			t.Errorf("expected no response in non-target channel")
		}
	})

	t.Run("Respond in target channel", func(t *testing.T) {
		reply, responded, err := svc.ProcessMessage(ctx, "target-channel-123", "alice", "hello loco", false, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !responded {
			t.Errorf("expected response for target channel")
		}
		if reply != "wassup bro" {
			t.Errorf("expected 'wassup bro', got: %s", reply)
		}

		// Verify memory was updated
		hist, err := memStore.GetHistory(ctx, "target-channel-123")
		if err != nil {
			t.Fatalf("unexpected memory error: %v", err)
		}
		// Expect 2 items: Alice's message and Loco's reply
		if len(hist) != 2 {
			t.Errorf("expected 2 items in history, got %d", len(hist))
		}
	})
}
