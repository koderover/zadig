/*
 * Copyright 2023 The KodeRover Authors.
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

package dingtalk

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/imroc/req/v3"
	cache "github.com/patrickmn/go-cache"
	"github.com/pkg/errors"
	"github.com/tidwall/gjson"
)

var (
	tokenCache = cache.New(time.Hour*2, time.Minute*5)
)

type Client struct {
	*req.Client
	AppKey    string
	AppSecret string

	cacheLock sync.RWMutex
}

func NewClient(key, secret string) (client *Client) {
	client = &Client{
		Client: req.C().
			SetJsonUnmarshal(func(data []byte, v interface{}) error {
				if result := gjson.Get(string(data), "result").String(); result != "" {
					return json.Unmarshal([]byte(result), v)
				}
				return json.Unmarshal(data, v)
			}).
			OnBeforeRequest(func(c *req.Client, req *req.Request) (err error) {
				// QPS limit
				limit.Wait(1)
				// get or refresh access token
				token, found := tokenCache.Get(key)
				if !found {
					token, err = client.RefreshAccessToken()
					if err != nil {
						return errors.Wrap(err, "refresh access token")
					}
				}
				// DingTalk some api use header, some use query param
				req.SetHeader("x-acs-dingtalk-access-token", token.(string))
				req.AddQueryParam("access_token", token.(string))
				return nil
			}).
			OnAfterResponse(func(client *req.Client, resp *req.Response) error {
				if resp.Err != nil {
					resp.Err = errors.Wrapf(resp.Err, "body: %s", resp.String())
					return nil
				}
				if !resp.IsSuccessState() {
					resp.Err = errors.Errorf("unexpected status code %d, body: %s", resp.GetStatusCode(), resp.String())
					return nil
				}
				if gjson.Get(resp.String(), "errcode").Int() != 0 {
					resp.Err = errors.Errorf("DingTalk API Error %s", resp.String())
					return nil
				}
				return nil
			}),
		AppKey:    key,
		AppSecret: secret,
	}
	return client
}

type TokenResponse struct {
	AccessToken string `json:"accessToken"`
	ExpireIn    int    `json:"expireIn"`
}

func (c *Client) RefreshAccessToken() (string, error) {
	var tokenResponse *TokenResponse
	resp, err := req.R().SetBodyJsonMarshal(struct {
		AppKey    string `json:"appKey"`
		AppSecret string `json:"appSecret"`
	}{
		AppKey:    c.AppKey,
		AppSecret: c.AppSecret,
	}).SetSuccessResult(&tokenResponse).Post("https://api.dingtalk.com/v1.0/oauth2/accessToken")
	if err != nil {
		return "", errors.Wrap(err, "request failed")
	}
	if resp.IsErrorState() {
		return "", errors.Errorf("unexpected status code %d, body: %s", resp.GetStatusCode(), resp.String())
	}
	tokenCache.Set(c.AppKey, tokenResponse.AccessToken, cache.DefaultExpiration)
	return tokenResponse.AccessToken, nil
}
