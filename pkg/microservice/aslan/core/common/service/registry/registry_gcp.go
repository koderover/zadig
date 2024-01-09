/*
Copyright 2023 The KodeRover Authors.

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

package registry

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"golang.org/x/oauth2/jws"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

func parseKey(key []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(key)
	if block != nil {
		key = block.Bytes
	}
	parsedKey, err := x509.ParsePKCS8PrivateKey(key)
	if err != nil {
		parsedKey, err = x509.ParsePKCS1PrivateKey(key)
		if err != nil {
			return nil, fmt.Errorf("private key should be a PEM or plain PKCS1 or PKCS8; parse error: %v", err)
		}
	}
	parsed, ok := parsedKey.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("private key is invalid")
	}
	return parsed, nil
}

func jwtAccessTokenSourceFromJSON(jsonKey []byte, audience string) (string, error) {
	cfg, err := google.JWTConfigFromJSON(jsonKey)
	if err != nil {
		return "", fmt.Errorf("google: could not parse JSON key: %v", err)
	}
	pk, err := parseKey(cfg.PrivateKey)
	if err != nil {
		return "", fmt.Errorf("google: could not parse key: %v", err)
	}

	iat := time.Now()
	exp := iat.Add(time.Hour)
	cs := &jws.ClaimSet{
		Iss:   cfg.Email,
		Sub:   cfg.Email,
		Aud:   audience,
		Scope: "https://www.googleapis.com/auth/cloud-platform",
		Iat:   iat.Unix(),
		Exp:   exp.Unix(),
	}
	hdr := &jws.Header{
		Algorithm: "RS256",
		Typ:       "JWT",
		KeyID:     cfg.PrivateKeyID,
	}
	msg, err := jws.Encode(hdr, cs, pk)
	if err != nil {
		return "", fmt.Errorf("gogle: could not encode JWT: %v", err)
	}
	tok, err := &oauth2.Token{AccessToken: msg, TokenType: "Bearer", Expiry: exp}, nil

	if err != nil {
		return "", err
	}
	return tok.AccessToken, nil
}

type GCPTokenResp struct {
	AccessToken string `json:"access_token"`
}

func doExchange(token string) (string, error) {
	d := url.Values{}
	d.Set("grant_type", "urn:ietf:params:oauth:grant-type:jwt-bearer")
	d.Add("assertion", token)

	client := &http.Client{}
	req, err := http.NewRequest("POST", "https://oauth2.googleapis.com/token", strings.NewReader(d.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	tokenResp := &GCPTokenResp{}
	err = json.Unmarshal(body, tokenResp)
	if err != nil {
		return "", err
	}

	return tokenResp.AccessToken, nil
}

func ExchangeAccessToken(keyData string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(keyData)
	if err != nil {
		return "", fmt.Errorf("failed to decode keyfile: %v", err)
	}

	assertToken, err := jwtAccessTokenSourceFromJSON(data, "https://oauth2.googleapis.com/token")
	if err != nil {
		return "", fmt.Errorf("failed to get assert token: %v", err)
	}

	exchangeToken, err := doExchange(assertToken)
	if err != nil {
		return "", fmt.Errorf("failed to exchange token: %v", err)
	}
	return exchangeToken, nil
}

var gcpTokenLock sync.Mutex

func (s *v2RegistryService) ensureGcpToken() error {
	if s.RegistryNS.RegProvider != config.RegistryTypeGCP {
		return nil
	}
	if s.RegistryNS.AccessToken != "" && time.Now().Unix()+300 < s.RegistryNS.AccessTokenExpireAt {
		return nil
	}

	gcpTokenLock.Lock()
	defer gcpTokenLock.Unlock()

	token, err := ExchangeAccessToken(s.RegistryNS.SecretKey)
	if err != nil {
		return fmt.Errorf("failed to exchange token: %v", err)
	}
	s.RegistryNS.AccessToken = token
	s.RegistryNS.AccessTokenExpireAt = time.Now().Add(1 * time.Hour).Unix()

	err = mongodb.NewRegistryNamespaceColl().Update(s.RegistryNS.ID.Hex(), s.RegistryNS)
	if err != nil {
		log.Errorf("failed to update registry namespace: %v", err)
	}

	return nil
}
