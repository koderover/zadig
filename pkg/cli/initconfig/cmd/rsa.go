package cmd

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"io/ioutil"
)

var pub *rsa.PublicKey

func RSA_Encrypt(plainText []byte) ([]byte, error) {
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
