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

package rsa

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadPrivKey(t *testing.T) {
	err := ioutil.WriteFile("tmp/rsa.priv", defaultPrivateKey, 0777)
	require.NoError(t, err)
	err = LoadPrivKey("tmp/rsa.priv")
	require.NoError(t, err)
}

func TestLoadPubKey(t *testing.T) {
	err := ioutil.WriteFile("tmp/rsa.pub", defaultPublicKey, 0777)
	require.NoError(t, err)
	err = LoadPubKey("tmp/rsa.pub")
	require.NoError(t, err)
}

func TestAddOperationData(t *testing.T) {
	str := `
	{
		"domain":"http://os.koderover.com",
		"username":"hello",
		"url":"http://os.koderover.com/v1/projects",
		"createdAt":1595244747
	}`
	encrypt := RSA_Encrypt([]byte(str))
	encodeString := base64.StdEncoding.EncodeToString(encrypt)
	fmt.Println(encodeString)
}

func TestAddAdminUser(t *testing.T) {
	str := `
	{
		"domain":"koderover.com",
		"username":"hello",
		"email":"hello@koderover.com",
		"created_at":1595244747
	}`
	encrypt := RSA_Encrypt([]byte(str))
	encodeString := base64.StdEncoding.EncodeToString(encrypt)
	fmt.Println(encodeString)
}
