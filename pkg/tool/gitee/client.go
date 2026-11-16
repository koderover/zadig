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

package gitee

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"gitee.com/openeuler/go-gitee/gitee"
	"golang.org/x/oauth2"

	"github.com/koderover/zadig/v2/pkg/shared/client/systemconfig"
	"github.com/koderover/zadig/v2/pkg/tool/cache"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

type Client struct {
	*gitee.APIClient
	config *gitee.Configuration
}

func NewClient(id int, address, accessToken, proxyAddr string, enableProxy bool) *Client {
	if accessToken != "" {
		mu := cache.NewRedisLock(fmt.Sprintf("gitee_token_refresh:%d", id))
		err := mu.Lock()
		if err != nil {
			log.Errorf("failed to acquire lock, err: %s", err)
		}
		defer mu.Unlock()

		ch, err := systemconfig.New().GetCodeHost(id)
		// The normal expiration time is 86400
		if err == nil && (time.Now().Unix()-ch.UpdatedAt) >= 86000 {
			token, err := RefreshAccessToken(ch.Address, ch.RefreshToken)
			if err == nil {
				accessToken = token.AccessToken
				ch.AccessToken = token.AccessToken
				ch.RefreshToken = token.RefreshToken

				if err = systemconfig.New().UpdateCodeHostToken(ch.ID, ch); err != nil {
					log.Errorf("failed to updateCodeHost err:%s", err)
				}
			} else {
				log.Errorf("failed to refresh accessToken, err:%s", err)
			}
		} else if err != nil {
			log.Errorf("failed to get codeHost id: %d, err:%s", id, err)
		}
	}
	return NewStaticClient(address, accessToken, proxyAddr, enableProxy)
}

// NewStaticClient uses the supplied token without refreshing it or writing codehost state.
func NewStaticClient(address, accessToken, proxyAddr string, enableProxy bool) *Client {
	client := http.DefaultClient
	if enableProxy {
		p, err := url.Parse(proxyAddr)
		if err == nil {
			client = &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(p)}}
		}
	}
	if accessToken != "" {
		ctx := context.WithValue(context.Background(), oauth2.HTTPClient, client)
		client = oauth2.NewClient(ctx, oauth2.StaticTokenSource(&oauth2.Token{AccessToken: accessToken}))
	}
	conf := gitee.NewConfiguration()
	conf.BasePath = address + "/api"
	conf.HTTPClient = client
	return &Client{APIClient: gitee.NewAPIClient(conf), config: conf}
}
