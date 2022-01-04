/*
Copyright 2021 The KodeRover Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"strings"
	"sync"

	fsutil "github.com/koderover/zadig/pkg/util/fs"
)

const aesKeyFile = "etc/encryption/aes"

type Aes struct {
	block cipher.Block
}

var aesKey string
var once sync.Once

func getAESKey() string {
	once.Do(func() {
		keyByte, err := fs.ReadFile(fsutil.Root(), aesKeyFile)
		if err != nil {
			panic("Failed to read aes key from secret")
		}
		aesKey = strings.TrimSpace(string(keyByte))
	})

	return aesKey
}

func GetAesKey() string {
	return getAESKey()
}

func AesEncrypt(src string) (string, error) {
	client, err := NewAes(getAESKey())
	if err != nil {
		return "", err
	}
	dest, err := client.Encrypt(src)
	if err != nil {
		return "", err
	}
	return dest, nil
}

func AesDecrypt(src string, aesKey ...string) (string, error) {
	var (
		err    error
		client *Aes
	)
	if len(aesKey) > 0 {
		client, err = NewAes(aesKey[0])
	} else {
		client, err = NewAes(getAESKey())
	}
	if err != nil {
		return "", err
	}
	dest, err := client.Decrypt(src)
	if err != nil {
		return "", err
	}
	return dest, nil
}

func NewAes(key string) (*Aes, error) {
	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return nil, err
	}
	return &Aes{block: block}, nil
}

func (a *Aes) Encrypt(plaintext string) (string, error) {
	cipherData := make([]byte, aes.BlockSize+len(plaintext))
	iv := cipherData[:aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return "", err
	}

	cipher.NewCFBEncrypter(a.block, iv).XORKeyStream(cipherData[aes.BlockSize:], []byte(plaintext))
	return hex.EncodeToString(cipherData), nil
}

func (a *Aes) Decrypt(d string) (string, error) {
	cipherData, err := hex.DecodeString(d)
	if err != nil {
		return "", err
	}

	if len(cipherData) < aes.BlockSize {
		return "", errors.New("cipherData too short")
	}
	iv := cipherData[:aes.BlockSize]
	cipherData = cipherData[aes.BlockSize:]
	cipher.NewCFBDecrypter(a.block, iv).XORKeyStream(cipherData, cipherData)
	return string(cipherData), nil
}
