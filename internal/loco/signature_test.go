package loco

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestGenerateSignature(t *testing.T) {
	secret := "test-secret-key"
	body := []byte(`{"message":"hello world"}`)
	now := time.Unix(1758532500, 0)

	sigHeader := GenerateSignature(secret, body, now)

	if !strings.HasPrefix(sigHeader, "t=1758532500,v1=") {
		t.Errorf("expected header to start with t=1758532500,v1=, got %s", sigHeader)
	}

	parts := strings.Split(sigHeader, ",v1=")
	if len(parts) != 2 {
		t.Fatalf("malformed header format")
	}

	expectedPayload := fmt.Sprintf("1758532500.%s", string(body))
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(expectedPayload))
	expectedHex := hex.EncodeToString(mac.Sum(nil))

	if parts[1] != expectedHex {
		t.Errorf("expected hex %s, got %s", expectedHex, parts[1])
	}
}
