package webhooks

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

// SignatureHeader names the header carrying the HMAC-SHA256 digest of the
// request body.
const SignatureHeader = "X-IaaS-Signature"

// Sign computes the HMAC-SHA256 signature of body using the webhook's secret,
// hex-encoded. Receivers recompute the digest with their secret and compare.
func Sign(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

// Verify reports whether signature is a valid HMAC-SHA256 digest of body for
// the given secret. It is constant-time against the expected digest.
func Verify(secret string, body []byte, signature string) bool {
	want := Sign(secret, body)
	return hmac.Equal([]byte(want), []byte(signature))
}
