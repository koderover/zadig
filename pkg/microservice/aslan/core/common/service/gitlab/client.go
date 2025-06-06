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

package gitlab

import (
	"github.com/koderover/zadig/v2/pkg/tool/git/gitlab"
)

type Client struct {
	*gitlab.Client
}

func NewClient(id int, address, accessToken, proxyAddr string, enableProxy bool, disableSSL bool) (*Client, error) {
	c, err := gitlab.NewClient(id, address, accessToken, proxyAddr, enableProxy, disableSSL)
	if err != nil {
		return nil, err
	}

	return &Client{Client: c}, nil
}
