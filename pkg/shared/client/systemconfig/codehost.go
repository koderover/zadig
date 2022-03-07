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

package systemconfig

import (
	"fmt"

	"github.com/koderover/zadig/pkg/tool/httpclient"
)

const (
	GitLabProvider  = "gitlab"
	GitHubProvider  = "github"
	GerritProvider  = "gerrit"
	CodeHubProvider = "codehub"
)

type CodeHost struct {
	ID          int    `json:"id"`
	Address     string `json:"address"`
	Type        string `json:"type"`
	AccessToken string `json:"access_token"`
	Namespace   string `json:"namespace"`
	Region      string `json:"region"`
	// the field and tag not consistent because of db field
	AccessKey string `json:"application_id"`
	SecretKey string `json:"client_secret"`
	Username  string `json:"username"`
	Password  string `json:"password"`
	// the field determine whether the proxy is enabled
	EnableProxy bool `json:"enable_proxy"`
}

type Option struct {
	CodeHostType string
	Address      string
	Namespace    string
	CodeHostID   int
}

func GetCodeHostInfo(opt *Option) (*CodeHost, error) {
	return New().GetCodeHostByAddressAndOwner(opt.Address, opt.Namespace, opt.CodeHostType)
}

func (c *Client) GetCodeHost(id int) (*CodeHost, error) {
	url := fmt.Sprintf("/codehosts/%d", id)

	res := &CodeHost{}
	_, err := c.Get(url, httpclient.SetResult(res))
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (c *Client) ListCodeHosts() ([]*CodeHost, error) {
	url := "/codehosts"

	res := make([]*CodeHost, 0)
	_, err := c.Get(url, httpclient.SetResult(&res))
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (c *Client) GetCodeHostByAddressAndOwner(address, owner, source string) (*CodeHost, error) {
	url := "/codehosts"

	res := make([]*CodeHost, 0)

	req := map[string]string{
		"address": address,
		"owner":   owner,
		"source":  source,
	}

	_, err := c.Get(url, httpclient.SetQueryParams(req), httpclient.SetResult(&res))
	if err != nil {
		return nil, err
	}

	if len(res) == 0 {
		return nil, fmt.Errorf("no codehost found")
	} else if len(res) > 1 {
		return nil, fmt.Errorf("more than one codehosts found")
	}

	return res[0], nil
}
