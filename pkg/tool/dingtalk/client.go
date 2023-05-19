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
	"sync"
	"time"

	"github.com/imroc/req/v3"
	"github.com/pkg/errors"

	cache "github.com/patrickmn/go-cache"
)

var (
	once       sync.Once
	tokenCache = cache.New(time.Hour*2, time.Minute*5)
)

type Client struct {
	*req.Client
	AppKey      string
	AppSecret   string
	accessToken string
}

func NewClient(key, secret string) (client *Client) {
	client = &Client{
		Client: req.C().
			SetBaseURL("https://api.dingtalk.com").
			OnBeforeRequest(func(c *req.Client, req *req.Request) (err error) {
				token, found := tokenCache.Get(key)
				if !found {
					token, err = client.RefreshAccessToken()
					if err != nil {
						return errors.Wrap(err, "refresh access token")
					}
				}
				req.SetHeader("x-acs-dingtalk-access-token", token.(string))
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
				return nil
			}),
	}
	return client
}

type TokenResponse struct {
	AccessToken string `json:"accessToken"`
	ExpireIn    int    `json:"expireIn"`
}

func (c *Client) RefreshAccessToken() (token string, err error) {
	var resp *TokenResponse
	_, err = req.R().SetBodyJsonMarshal(struct {
		AppKey    string `json:"appKey"`
		AppSecret string `json:"appSecret"`
	}{
		AppKey:    c.AppKey,
		AppSecret: c.AppSecret,
	}).SetSuccessResult(&resp).Post("https://api.dingtalk.com/v1.0/oauth2/accessToken")
	if err != nil {
		return "", errors.Wrap(err, "request failed")
	}
	tokenCache.Set(c.AppKey, resp.AccessToken, cache.DefaultExpiration)
	return resp.AccessToken, nil
}
