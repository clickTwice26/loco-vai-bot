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
}
