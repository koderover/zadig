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

package github

import (
	"github.com/koderover/zadig/pkg/tool/git/github"
)

type Client struct {
	*github.Client
}

func NewClient(accessToken, proxyAddress string) *Client {
	return &Client{
		Client: github.NewClient(&github.Config{AccessToken: accessToken, Proxy: proxyAddress}),
	}
}

func GetGithubAppClientByOwner(appID int, appKey, owner, proxy string) (*Client, error) {
	gc, err := github.NewAppClient(&github.Config{
		AppKey: appKey,
		AppID:  appID,
		Owner:  owner,
		Proxy:  proxy,
	})

	if err != nil {
		return nil, err
	}

	return &Client{Client: gc}, nil
}
