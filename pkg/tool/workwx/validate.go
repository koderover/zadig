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
	"crypto/sha1"
	"encoding/hex"
	"sort"
	"strings"
)

func Validate(host, corpID string, agentID int, agentSecret string) error {
	client := NewClient(host, corpID, agentID, agentSecret)
	_, err := client.getAccessToken(true)
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
