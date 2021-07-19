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

package poetry

import (
	"github.com/koderover/zadig/pkg/tool/httpclient"
)

type CodeHost struct {
	ID          int    `json:"id"`
	OrgID       int    `json:"orgId"`
	Address     string `json:"address"`
	Type        string `json:"type"`
	AccessToken string `json:"accessToken"`
	Namespace   string `json:"namespace"`
	Region      string `json:"region"`
	AccessKey   string `json:"applicationId"`
	SecretKey   string `json:"clientSecret"`
	Username    string `json:"username"`
	Password    string `json:"password"`
}

func (c *Client) ListCodeHosts() ([]*CodeHost, error) {
	url := "/directory/codehostss/search"

	codeHosts := make([]*CodeHost, 0)
	_, err := c.Get(url, httpclient.SetResult(&codeHosts), httpclient.SetQueryParam("orgId", "1"))
	if err != nil {
		return nil, err
	}

	return codeHosts, nil
}
