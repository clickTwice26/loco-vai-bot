package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Config represents the application configuration.
type Config struct {
	// DiscordToken is the Bot authentication token.
	DiscordToken string
	// AppID is the Discord Application ID (Client ID).
	AppID string
	// GuildID is an optional server ID used for fast command testing during development.
	// When empty, commands will be registered globally (which may take up to 1 hour to sync on Discord's side).
	GuildID string
	// Environment e.g. "development" or "production".
	Environment string
	// LogLevel e.g. "DEBUG", "INFO", "WARN", "ERROR".
	LogLevel string
	// LogFormat e.g. "text" or "json".
	LogFormat string
	// Port is the HTTP port for health checks.
	Port string
	// RemoveCommandsOnShutdown determines if commands should be deleted from Discord when bot shuts down.
	RemoveCommandsOnShutdown bool
}

// Load reads configuration from .env and environment variables.
func Load() (*Config, error) {
	// Attempt to load .env file; silently ignore if it doesn't exist (useful in production/Docker).
	_ = godotenv.Load()

	cfg := &Config{
		DiscordToken:             os.Getenv("DISCORD_TOKEN"),
		AppID:                    os.Getenv("APP_ID"),
		GuildID:                  os.Getenv("GUILD_ID"),
		Environment:              getEnvOrDefault("ENV", "development"),
		LogLevel:                 getEnvOrDefault("LOG_LEVEL", "INFO"),
		LogFormat:                getEnvOrDefault("LOG_FORMAT", "text"),
		Port:                     getEnvOrDefault("PORT", "8080"),
		RemoveCommandsOnShutdown: getBoolOrDefault("REMOVE_COMMANDS_ON_SHUTDOWN", false),
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return cfg, nil
}

func (c *Config) validate() error {
	if strings.TrimSpace(c.DiscordToken) == "" {
		return errors.New("DISCORD_TOKEN is required")
	}
	if strings.TrimSpace(c.AppID) == "" {
		return errors.New("APP_ID is required (Discord Application ID)")
	}
	return nil
}

// IsDevelopment returns true if running in development mode.
func (c *Config) IsDevelopment() bool {
	return strings.ToLower(c.Environment) == "development" || strings.ToLower(c.Environment) == "dev"
}

func getEnvOrDefault(key, fallback string) string {
	if val := os.Getenv(key); strings.TrimSpace(val) != "" {
		return strings.TrimSpace(val)
	}
	return fallback
}

func getBoolOrDefault(key string, fallback bool) bool {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}
	b, err := strconv.ParseBool(val)
	if err != nil {
		return fallback
	}
	return b
}
