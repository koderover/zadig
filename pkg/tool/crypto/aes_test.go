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
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAes_Crypt(t *testing.T) {
	ast := require.New(t)

	aes, err := NewAes("aaaaaaaaa")
	ast.Nil(err)

	encrypted, err := aes.Encrypt("hello")
	ast.Nil(err)

	decrypted, err := aes.Decrypt(encrypted)
	ast.Nil(err)
	ast.Equal("hello", decrypted)
}
