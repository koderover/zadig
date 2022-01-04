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

package cmd

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"os"
)

var pub *rsa.PublicKey

func RSAEncrypt(plainText []byte) ([]byte, error) {
	if err := LoadPubKey(""); err != nil {
		return nil, err
	}
	//对明文进行加密
	cipherText, err := rsa.EncryptPKCS1v15(rand.Reader, pub, plainText)
	if err != nil {
		return nil, err
	}
	//返回密文
	return cipherText, nil
}

// LoadPubKey ...
func LoadPubKey(filename string) (err error) {
	var block *pem.Block
	if filename == "" {
		block, _ = pem.Decode([]byte(defaultPublicKey))
	} else {
		b, err := os.ReadFile(filename)
		if err != nil {
			return err
		}
		block, _ = pem.Decode(b)
	}
	if block == nil {
		return errors.New("public key error")
	}
	pubInterface, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return
	}
	pub = pubInterface.(*rsa.PublicKey)
	return
}

var defaultPublicKey = `
-----BEGIN RSA PUBLIC KEY-----
MIIBpTANBgkqhkiG9w0BAQEFAAOCAZIAMIIBjQKCAYQAz5IqagSbovHGXmUf7wTB
XrR+DZ0u3p5jsgJW08ISJl83t0rCCGMEtcsRXJU8bE2dIIfndNwvmBiqh13/WnJd
+jgyIm6i1ZfNmf/R8pEqVXpOAOuyoD3VLT9tfWvz9nPQbjVI+PsUHH7nVR0Jwxem
NsH/7MC2O15t+2DVC1533UlhjT/pKFDdTri0mgDrLZHp6gPF5d7/yQ7cPbzv6/0p
0UgIdStT7IhkDfsJDRmLAz09znv5tQQtHfJIMdAKxwHw9mExcL2gE40sOezrgj7m
srOnJd65N8anoMGxQqNv+ycAHB9aI1Yrtgue2KKzpI/Fneghd/ZavGVFWKDYoFP3
531Ga/CiCwtKfM0vQezfLZKAo3qpb0Edy2BcDHhLwidmaFwh8ZlXuaHbNaF/FiVR
h7uu0/9B/gn81o2f+c8GSplWB5bALhQH8tJZnvmWZGI9OnrIlWmQZsuUBooTul9Q
ZJ/w3sE1Zoxa+Or1/eWijqtIfhukOJBNyGaj+esFg6uEeBgHAgMBAAE=
-----END RSA PUBLIC KEY-----
`
