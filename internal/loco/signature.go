package loco

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

// GenerateSignature creates the X-Localoy-Signature header value.
// Format: t=<unix_timestamp>,v1=<hex(HMAC-SHA256(secret, "<t>.<body>"))>
func GenerateSignature(secret string, body []byte, now time.Time) string {
	timestamp := now.Unix()
	payloadToSign := fmt.Sprintf("%d.%s", timestamp, string(body))

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payloadToSign))
	signatureHex := hex.EncodeToString(mac.Sum(nil))

	return fmt.Sprintf("t=%d,v1=%s", timestamp, signatureHex)
}
