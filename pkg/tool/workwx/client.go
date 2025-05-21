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
	"fmt"
	"strings"
	"time"

	"github.com/koderover/zadig/v2/pkg/tool/log"

	"github.com/koderover/zadig/v2/pkg/config"
	"github.com/koderover/zadig/v2/pkg/tool/cache"
	"github.com/koderover/zadig/v2/pkg/tool/httpclient"
)

var WorkWxClient *Client

type Client struct {
	Host        string
	CorpID      string
	AgentSecret string
	AgentID     int
}

const (
	expireThreshold int64 = 300
)

func NewClient(host, corpID string, agentID int, agentSecret string) *Client {
	client := &Client{
		CorpID:      corpID,
		AgentSecret: agentSecret,
		AgentID:     agentID,
	}

	if host == "" {
		client.Host = "https://qyapi.weixin.qq.com"
	} else {
		client.Host = strings.TrimSuffix(host, "/")
	}

	return client
}

func (c *Client) getAccessToken(skipCache bool) (string, error) {
	redisCache := cache.NewRedisCache(config.RedisCommonCacheTokenDB())
	// first try to get the access token from redis
	accessTokenKey := fmt.Sprintf("workwx-%s-%s-%d-ak")
	ak, err := redisCache.GetString(accessTokenKey)
	if err == nil && !skipCache {
		return ak, nil
	}

	// if no token is in the redis, then it is expired, we lock this instance and try to refresh the token
	accessTokenLockKey := fmt.Sprintf("%s-%s-%d-lock", c.Host, c.CorpID, c.AgentID)
	mu := cache.NewRedisLockWithExpiry(accessTokenLockKey, time.Second*10)

	err = mu.Lock()
	if err != nil {
		return "", fmt.Errorf("failed to update workwx token, err: %s", err)
	}
	defer mu.Unlock()

	// once the lock has been acquired, we first check the redis again to avoid re-refresh.
	ak, err = redisCache.GetString(accessTokenKey)
	if err == nil {
		return ak, nil
	}

	// finally we made sure the access token does not exist, refresh the token
	url := fmt.Sprintf("%s/%s", c.Host, getAccessTokenAPI)

	requestQuery := map[string]string{
		"corpsecret": c.AgentSecret,
		"corpid":     c.CorpID,
	}

	resp := new(getAccessTokenResp)

	_, err = httpclient.Get(
		url,
		httpclient.SetQueryParams(requestQuery),
		httpclient.SetResult(resp),
	)

	if err != nil {
		return "", fmt.Errorf("failed to refresh access token for workwx, err: %s", err)
	}

	if workwxErr := resp.ToError(); workwxErr != nil {
		return "", fmt.Errorf("failed to refresh access token for workwx, err: %s", err)
	}

	expirationTime := time.Duration(resp.ExpiresIn - expireThreshold)
	err = redisCache.SetNX(accessTokenKey, resp.AccessToken, expirationTime*time.Second)
	if err != nil {
		log.Warn("failed to save the created workwx access token into redis, err: %s", err)
	}

	return resp.AccessToken, nil
}
