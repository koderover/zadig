package agentupgrade

import (
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"strings"
)

func PublicKey(encoded string) (ed25519.PublicKey, error) {
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil || len(key) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("invalid agent upgrade public key")
	}
	return ed25519.PublicKey(key), nil
}
