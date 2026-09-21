package config

import (
	"os"
	"testing"
)

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		envToken    string
		envAppID    string
		expectError bool
	}{
		{
			name:        "valid configuration",
			envToken:    "test-token",
			envAppID:    "123456789",
			expectError: false,
		},
		{
			name:        "missing token",
			envToken:    "",
			envAppID:    "123456789",
			expectError: true,
		},
		{
			name:        "missing app id",
			envToken:    "test-token",
			envAppID:    "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Clearenv()
			if tt.envToken != "" {
				os.Setenv("DISCORD_TOKEN", tt.envToken)
			}
			if tt.envAppID != "" {
				os.Setenv("APP_ID", tt.envAppID)
			}

			cfg, err := Load()
			if tt.expectError {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Fatalf("expected no error, got: %v", err)
				}
				if cfg.DiscordToken != tt.envToken {
					t.Errorf("expected token %s, got %s", tt.envToken, cfg.DiscordToken)
				}
				if cfg.AppID != tt.envAppID {
					t.Errorf("expected app ID %s, got %s", tt.envAppID, cfg.AppID)
				}
			}
		})
	}
}
