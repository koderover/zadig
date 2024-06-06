/*
 * Copyright 2024 The KodeRover Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package workwx

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"fmt"
)

func DecodeEncryptedMessage(key, message string) ([]byte, error) {
	// Step 1: base 64 decoding
	decodedBytes, err := base64.StdEncoding.DecodeString(message)
	if err != nil {
		return nil, err
	}

	decodedKeyBytes, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return nil, err
	}

	iv := decodedKeyBytes[:16]
	block, err := aes.NewCipher(decodedKeyBytes)
	if err != nil {
		return nil, err
	}

	// Ensure the ciphertext is a multiple of the block size
	if len(decodedBytes)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("ciphertext is not a multiple of the block size")
	}

	mode := cipher.NewCBCDecrypter(block, iv)
	plaintext := make([]byte, len(decodedBytes))
	mode.CryptBlocks(plaintext, decodedBytes)

	return plaintext, nil
}
