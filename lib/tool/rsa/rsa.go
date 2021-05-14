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
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"io/ioutil"
)

//var defaultPrivateKey = []byte(`
//-----BEGIN RSA PRIVATE KEY-----
//MIICXQIBAAKBgQDZsfv1qscqYdy4vY+P4e3cAtmvppXQcRvrF1cB4drkv0haU24Y
//7m5qYtT52Kr539RdbKKdLAM6s20lWy7+5C0DgacdwYWd/7PeCELyEipZJL07Vro7
//Ate8Bfjya+wltGK9+XNUIHiumUKULW4KDx21+1NLAUeJ6PeW+DAkmJWF6QIDAQAB
//AoGBAJlNxenTQj6OfCl9FMR2jlMJjtMrtQT9InQEE7m3m7bLHeC+MCJOhmNVBjaM
//ZpthDORdxIZ6oCuOf6Z2+Dl35lntGFh5J7S34UP2BWzF1IyyQfySCNexGNHKT1G1
//XKQtHmtc2gWWthEg+S6ciIyw2IGrrP2Rke81vYHExPrexf0hAkEA9Izb0MiYsMCB
///jemLJB0Lb3Y/B8xjGjQFFBQT7bmwBVjvZWZVpnMnXi9sWGdgUpxsCuAIROXjZ40
//IRZ2C9EouwJBAOPjPvV8Sgw4vaseOqlJvSq/C/pIFx6RVznDGlc8bRg7SgTPpjHG
//4G+M3mVgpCX1a/EU1mB+fhiJ2LAZ/pTtY6sCQGaW9NwIWu3DRIVGCSMm0mYh/3X9
//DAcwLSJoctiODQ1Fq9rreDE5QfpJnaJdJfsIJNtX1F+L3YceeBXtW0Ynz2MCQBI8
//9KP274Is5FkWkUFNKnuKUK4WKOuEXEO+LpR+vIhs7k6WQ8nGDd4/mujoJBr5mkrw
//DPwqA3N5TMNDQVGv8gMCQQCaKGJgWYgvo3/milFfImbp+m7/Y3vCptarldXrYQWO
//AQjxwc71ZGBFDITYvdgJM1MTqc8xQek1FXn1vfpy2c6O
//-----END RSA PRIVATE KEY-----
//`)
//
//var defaultPublicKey = []byte(`
//-----BEGIN PUBLIC KEY-----
//MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQDZsfv1qscqYdy4vY+P4e3cAtmv
//ppXQcRvrF1cB4drkv0haU24Y7m5qYtT52Kr539RdbKKdLAM6s20lWy7+5C0Dgacd
//wYWd/7PeCELyEipZJL07Vro7Ate8Bfjya+wltGK9+XNUIHiumUKULW4KDx21+1NL
//AUeJ6PeW+DAkmJWF6QIDAQAB
//-----END PUBLIC KEY-----
//`)

var defaultPublicKey = []byte(`
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
`)

var defaultPrivateKey = []byte(`
-----BEGIN RSA PRIVATE KEY-----
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
/pmBk/w=
-----END RSA PRIVATE KEY-----
`)

var priv *rsa.PrivateKey
var pub *rsa.PublicKey

// LoadPrivKey ...
func LoadPrivKey(filename string) (err error) {
	var block *pem.Block
	if filename == "" {
		block, _ = pem.Decode(defaultPrivateKey)
	} else {
		b, err := ioutil.ReadFile(filename)
		if err != nil {
			return err
		}
		block, _ = pem.Decode(b)
	}
	if block == nil {
		return errors.New("private key error")
	}
	priv, err = x509.ParsePKCS1PrivateKey(block.Bytes)
	return
}

// LoadPubKey ...
func LoadPubKey(filename string) (err error) {
	var block *pem.Block
	if filename == "" {
		block, _ = pem.Decode(defaultPublicKey)
	} else {
		b, err := ioutil.ReadFile(filename)
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

//RSA解密
// cipherText 需要解密的byte数据
func RSA_Decrypt(cipherText []byte) ([]byte, error) {
	if err := LoadPrivKey(""); err != nil {
		return []byte{}, err
	}
	//对密文进行解密
	plainText, err := rsa.DecryptPKCS1v15(rand.Reader, priv, cipherText)
	//返回明文
	return plainText, err
}

//RSA加密
// plainText 要加密的数据
func RSA_Encrypt(plainText []byte) []byte {
	if err := LoadPubKey(""); err != nil {
		return nil
	}
	//对明文进行加密
	cipherText, err := rsa.EncryptPKCS1v15(rand.Reader, pub, plainText)
	if err != nil {
		panic(err)
	}
	//返回密文
	return cipherText
}
