package b2b

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// SignPayload computes the HMAC-SHA256 signature for a given body
func SignPayload(secret []byte, body []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// AttachSignature sets the X-Webhook-Signature and X-Webhook-Timestamp headers
func AttachSignature(req *http.Request, secret []byte, body []byte) {
	sig := SignPayload(secret, body)
	req.Header.Set("X-Webhook-Signature", sig)
	req.Header.Set("X-Webhook-Timestamp", strconv.FormatInt(time.Now().Unix(), 10))
}

// VerifyWebhook checks if the signature and timestamp are valid
func VerifyWebhook(secret []byte, body []byte, sigHeader string, tsHeader string) error {
	// 1. Reject stale requests (replay attack prevention)
	ts, err := strconv.ParseInt(tsHeader, 10, 64)
	if err != nil || time.Now().Unix()-ts > 300 { // 5-minute window
		return fmt.Errorf("request timestamp too old or invalid")
	}

	// 2. Recompute HMAC over raw body bytes
	expected := SignPayload(secret, body)

	// 3. Constant-time comparison
	if !hmac.Equal([]byte(expected), []byte(sigHeader)) {
		return fmt.Errorf("signature mismatch")
	}

	return nil
}
