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

package util

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"github.com/dgrijalva/jwt-go"
	"github.com/koderover/zadig/pkg/config"
)

// ComputeHmacSha256 According to ak/sk generate secret key
func ComputeHmacSha256(ak string, sk string) string {
	key := []byte(sk)
	h := hmac.New(sha256.New, key)
	h.Write([]byte(ak))
	sha := hex.EncodeToString(h.Sum(nil))
	hex.EncodeToString(h.Sum(nil))
	return sha
}

type Claims struct {
	Name            string          `json:"name"`
	Email           string          `json:"email"`
	Uid             string          `json:"uid"`
	FederatedClaims FederatedClaims `json:"federated_claims"`
	jwt.StandardClaims
}

type FederatedClaims struct {
	ConnectorId string `json:"connector_id"`
	UserId      string `json:"user_id"`
}

func CreateToken(claims *Claims) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(config.SecretKey()))
	if err != nil {
		return "", err
	}
	return tokenString, nil
}
