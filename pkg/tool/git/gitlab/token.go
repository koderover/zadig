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
	"fmt"
	"sync"
	"time"

	"github.com/koderover/zadig/pkg/shared/client/systemconfig"
	"github.com/koderover/zadig/pkg/tool/httpclient"
	"github.com/koderover/zadig/pkg/tool/log"
)

type AccessToken struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
	CreatedAt    int    `json:"created_at"`
}

var CodeHostLockMap sync.Map

const TokenExpirationThreshold int64 = 7000

func UpdateGitlabToken(id int, accessToken string) (string, error) {
	// if accessToken is empty, then it is either of ssh token type or username/password, we return empty
	if accessToken == "" {
		return "", nil
	}

	lockInterface, _ := CodeHostLockMap.LoadOrStore(id, &sync.RWMutex{})
	lock := lockInterface.(*sync.RWMutex)
	lock.Lock()
	defer lock.Unlock()

	ch, err := systemconfig.New().GetCodeHost(id)

	if err != nil {
		return "", fmt.Errorf("get codehost info error: [%s]", err)
	}

	if time.Now().Unix()-ch.UpdatedAt <= TokenExpirationThreshold {
		return ch.AccessToken, nil
	}

	log.Infof("Starting to refresh gitlab token")

	token, err := refreshAccessToken(ch.Address, ch.AccessKey, ch.SecretKey, ch.RefreshToken)
	if err != nil {
		return "", err
	}

	ch.AccessToken = token.AccessToken
	ch.RefreshToken = token.RefreshToken
	ch.UpdatedAt = int64(token.CreatedAt)

	// Since the new token is valid, we simply log the error
	// No error will be returned, only the new token is returned
	if err = systemconfig.New().UpdateCodeHost(ch.ID, ch); err != nil {
		log.Errorf("failed to update codehost, err: %s", err)
	}

	log.Infof("gitlab token for client [%d] changed from [%s] to [%s]", id, accessToken, token.AccessToken)

	return token.AccessToken, nil
}

func refreshAccessToken(address, clientID, clientSecret, refreshToken string) (*AccessToken, error) {
	httpClient := httpclient.New(
		httpclient.SetHostURL(address),
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
