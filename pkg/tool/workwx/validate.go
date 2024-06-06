package workwx

import (
	"crypto/sha1"
	"encoding/hex"
	"sort"
	"strings"
)

func Validate(host, corpID string, agentID int, agentSecret string) error {
	client := NewClient(host, corpID, agentID, agentSecret)
	_, err := client.getAccessToken()
	return err
}

func CallbackValidate(targetStr, token, timestamp, nonce, msgEncrypt string) bool {
	arr := []string{token, timestamp, nonce, msgEncrypt}
	sort.Strings(arr)

	hasher := sha1.New()
	// Write data to it
	hasher.Write([]byte(strings.Join(arr, "")))
	// Get the SHA-1 checksum as a byte slice
	hashBytes := hasher.Sum(nil)
	// Convert the byte slice to a hexadecimal string
	hashString := hex.EncodeToString(hashBytes)

	return targetStr == hashString
}
