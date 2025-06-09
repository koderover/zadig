/*
Copyright 2022 The KodeRover Authors.

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

package gitlab

import (
	"crypto/tls"
	"fmt"
	"time"

	"github.com/koderover/zadig/v2/pkg/shared/client/systemconfig"
	"github.com/koderover/zadig/v2/pkg/tool/cache"
	"github.com/koderover/zadig/v2/pkg/tool/httpclient"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

type AccessToken struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
	CreatedAt    int    `json:"created_at"`
}

const TokenExpirationThreshold int64 = 7000

func UpdateGitlabToken(id int, accessToken string) (string, error) {
	// if accessToken is empty, then it is either of ssh token type or username/password, we return empty
	if accessToken == "" {
		return "", nil
	}

	mu := cache.NewRedisLockWithExpiry(fmt.Sprintf("gitlab_token_refresh:%d", id), time.Second*10)
	err := mu.Lock()
	if err != nil {
		log.Errorf("failed to acquire gitlab token refresh lock, err: %s", err)
		return "", fmt.Errorf("failed to update gitlab token, err: %s", err)
	}
	defer mu.Unlock()

	ch, err := systemconfig.New().GetRawCodeHost(id)

	if err != nil {
		return "", fmt.Errorf("get codehost info error: [%s]", err)
	}

	oldToken := ch.AccessToken

	if time.Now().Unix()-ch.UpdatedAt <= TokenExpirationThreshold {
		return ch.AccessToken, nil
	}

	log.Infof("Starting to refresh gitlab token, old token issued time: %d", ch.UpdatedAt)

	token, err := refreshAccessToken(ch.Address, ch.AccessKey, ch.SecretKey, ch.RefreshToken, ch.DisableSSL)
	if err != nil {
		return "", err
	}

	newCodehost := &systemconfig.CodeHost{
		ID:                 ch.ID,
		Address:            ch.Address,
		Type:               ch.Type,
		AccessToken:        token.AccessToken,
		RefreshToken:       token.RefreshToken,
		Namespace:          ch.Namespace,
		Region:             ch.Region,
		AccessKey:          ch.AccessKey,
		SecretKey:          ch.SecretKey,
		Username:           ch.Username,
		Password:           ch.Password,
		EnableProxy:        ch.EnableProxy,
		UpdatedAt:          time.Now().Unix(),
		Alias:              ch.Alias,
		AuthType:           ch.AuthType,
		SSHKey:             ch.SSHKey,
		PrivateAccessToken: ch.PrivateAccessToken,
		DisableSSL:         ch.DisableSSL,
	}

	log.Infof("the update time of the codehost is: %d", newCodehost.UpdatedAt)

	// Since the new token is valid, we simply log the error
	// No error will be returned, only the new token is returned
	if err = systemconfig.New().UpdateCodeHost(ch.ID, newCodehost); err != nil {
		log.Errorf("failed to update codehost, err: %s", err)
	}

	log.Infof("gitlab token for client [%d] changed from [%s] to [%s]", id, oldToken, token.AccessToken)

	return token.AccessToken, nil
}

func refreshAccessToken(address, clientID, clientSecret, refreshToken string, disableSSL bool) (*AccessToken, error) {
	httpClient := httpclient.New(
		httpclient.SetHostURL(address),
		httpclient.SetTLSClientConfig(&tls.Config{InsecureSkipVerify: disableSSL}),
	)
	url := "/oauth/token"
	queryParams := make(map[string]string)
	queryParams["grant_type"] = "refresh_token"
	queryParams["refresh_token"] = refreshToken
	queryParams["client_id"] = clientID
	queryParams["client_secret"] = clientSecret

	var accessToken *AccessToken
	_, err := httpClient.Post(url, httpclient.SetQueryParams(queryParams), httpclient.SetResult(&accessToken))
	if err != nil {
		return nil, err
	}

	return accessToken, nil
}
