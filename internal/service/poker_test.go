package service

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"

	"localoy-bot/internal/memory"

	"github.com/bwmarrin/discordgo"
)

func TestPokerFallback(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	memStore := memory.NewInMemoryStore(5)

	ps := NewPokerService(nil, memStore, "test-channel", 60, 120, logger)

	target := &discordgo.User{
		ID:       "123456789",
		Username: "testuser",
	}

	ctx := context.Background()
	poke, err := ps.CraftPoke(ctx, target)
	if err != nil {
		t.Fatalf("unexpected error crafting poke: %v", err)
	}

	if !strings.Contains(poke, "123456789") {
		t.Errorf("expected mention tag or ID in poke, got: %s", poke)
	}

	// Test custom small intervals from environment
	customPs := NewPokerService(nil, memStore, "test-channel", 3, 20, logger).(*pokerService)
	if customPs.minInterval.Minutes() != 3 {
		t.Errorf("expected min interval to be 3 minutes, got %v", customPs.minInterval)
	}
	if customPs.maxInterval.Minutes() != 20 {
		t.Errorf("expected max interval to be 20 minutes, got %v", customPs.maxInterval)
	}
}
