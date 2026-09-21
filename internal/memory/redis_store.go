package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	defaultKeyPrefix = "loco:chat:history:"
	defaultTTL       = 24 * time.Hour
)

// RedisStore implements Store using Redis list structures.
type RedisStore struct {
	client     *redis.Client
	maxHistory int
	logger     *slog.Logger
}

// NewRedisStore connects to Redis and returns a Redis-backed memory store.
func NewRedisStore(ctx context.Context, redisURL string, maxHistory int, logger *slog.Logger) (*RedisStore, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("invalid redis url: %w", err)
	}

	client := redis.NewClient(opts)

	// Verify connection
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := client.Ping(pingCtx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("failed to ping redis: %w", err)
	}

	logger.Info("connected to Redis memory store successfully")

	return &RedisStore{
		client:     client,
		maxHistory: maxHistory,
		logger:     logger.With("module", "redis-memory"),
	}, nil
}

func (r *RedisStore) key(channelID string) string {
	return defaultKeyPrefix + channelID
}

// GetHistory retrieves the stored chat messages from Redis.
func (r *RedisStore) GetHistory(ctx context.Context, channelID string) ([]Message, error) {
	key := r.key(channelID)
	rawList, err := r.client.LRange(ctx, key, 0, -1).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch history from redis: %w", err)
	}

	messages := make([]Message, 0, len(rawList))
	for _, raw := range rawList {
		var m Message
		if err := json.Unmarshal([]byte(raw), &m); err != nil {
			r.logger.Warn("failed to unmarshal message from redis", "error", err)
			continue
		}
		messages = append(messages, m)
	}

	return messages, nil
}

// AppendMessage pushes a message to the Redis list and trims older items beyond maxHistory.
func (r *RedisStore) AppendMessage(ctx context.Context, channelID string, msg Message) error {
	key := r.key(channelID)

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	pipe := r.client.TxPipeline()
	pipe.RPush(ctx, key, data)
	// Keep only the last maxHistory items
	startIdx := int64(-r.maxHistory)
	pipe.LTrim(ctx, key, startIdx, -1)
	pipe.Expire(ctx, key, defaultTTL)

	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("failed to execute redis pipeline: %w", err)
	}

	return nil
}

// ClearHistory deletes the history key in Redis.
func (r *RedisStore) ClearHistory(ctx context.Context, channelID string) error {
	return r.client.Del(ctx, r.key(channelID)).Err()
}

// Close closes the Redis client.
func (r *RedisStore) Close() error {
	return r.client.Close()
}
