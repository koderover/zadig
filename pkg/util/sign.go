package util

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

// ComputeHmacSha256 According to ak/sk generate secret key
func ComputeHmacSha256(ak string, sk string) string {
	key := []byte(sk)
	h := hmac.New(sha256.New, key)
	h.Write([]byte(ak))
	sha := hex.EncodeToString(h.Sum(nil))
	hex.EncodeToString(h.Sum(nil))
	return sha
}
