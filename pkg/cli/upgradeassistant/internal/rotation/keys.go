package rotation

import (
	"encoding/base32"
	"fmt"
	"os"

	"github.com/koderover/zadig/v2/pkg/tool/crypto"
)

type Keys struct {
	old, next     *crypto.Aes
	webhookSecret string
}

func ReadKeys(oldFile, newFile string) (*Keys, error) {
	old, err := os.ReadFile(oldFile)
	if err != nil {
		return nil, fmt.Errorf("cannot read old key file")
	}
	next, err := os.ReadFile(newFile)
	if err != nil {
		return nil, fmt.Errorf("cannot read new key file")
	}
	return newKeys(string(old), string(next))
}

func newKeys(old, next string) (*Keys, error) {
	oldAES, err := crypto.NewAes(old)
	if err != nil {
		return nil, fmt.Errorf("old key must contain exactly 16, 24 or 32 bytes")
	}
	newAES, err := crypto.NewAes(next)
	if err != nil {
		return nil, fmt.Errorf("new key must contain exactly 16, 24 or 32 bytes")
	}
	if old == next {
		return nil, fmt.Errorf("old and new keys must differ")
	}
	return &Keys{old: oldAES, next: newAES, webhookSecret: crypto.Sha256([]byte(next))}, nil
}

func (k *Keys) reencrypt(cipher, kind, hash string) (string, error) {
	plain, err := k.old.Decrypt(cipher)
	if err != nil {
		return "", fmt.Errorf("invalid ciphertext")
	}
	switch kind {
	case "k8s_cluster":
		if hash == "" || crypto.Sha256([]byte(plain)) != hash {
			return "", fmt.Errorf("Agent Token plaintext hash mismatch")
		}
	case "user_mfa":
		decoded, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(plain)
		if err != nil || len(decoded) == 0 {
			return "", fmt.Errorf("MFA secret is not valid base32")
		}
	}
	next, err := k.next.Encrypt(plain)
	if err != nil {
		return "", fmt.Errorf("cannot encrypt record")
	}
	return next, nil
}
