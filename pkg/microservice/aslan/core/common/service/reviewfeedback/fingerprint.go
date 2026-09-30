package reviewfeedback

import (
	"encoding/hex"
	"strings"
)

const fingerprintMarker = "<!-- zadig-ai-review-fingerprint:"

func FingerprintMarker(fingerprint string) string {
	if fingerprint == "" {
		return ""
	}
	return fingerprintMarker + hex.EncodeToString([]byte(fingerprint)) + " -->"
}

func FingerprintFromBody(body string) string {
	start := strings.Index(body, fingerprintMarker)
	if start < 0 {
		return ""
	}
	encoded := body[start+len(fingerprintMarker):]
	end := strings.Index(encoded, " -->")
	if end < 0 {
		return ""
	}
	decoded, err := hex.DecodeString(encoded[:end])
	if err != nil {
		return ""
	}
	return string(decoded)
}
