package memory

import (
	"context"
	"log/slog"
	"strings"
)

// NewStore initializes a RedisStore if redisURL is specified, or falls back to an InMemoryStore.
func NewStore(ctx context.Context, redisURL string, maxHistory int, logger *slog.Logger) Store {
	if maxHistory <= 0 {
		maxHistory = 15
	}

	if strings.TrimSpace(redisURL) != "" {
		store, err := NewRedisStore(ctx, redisURL, maxHistory, logger)
		if err == nil {
			return store
		}
		logger.Warn("failed to initialize Redis store, falling back to in-memory store", "error", err)
	} else {
		logger.Info("no REDIS_URL configured, using in-memory store for chat memory")
	}

	return NewInMemoryStore(maxHistory)
}
