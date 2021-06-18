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

type JiraInfo struct {
	ID             int64  `json:"id"`
	Host           string `json:"host"`
	User           string `json:"user"`
	AccessToken    string `json:"accessToken"`
	OrganizationID int    `json:"organizationId"`
	CreatedAt      int64  `json:"created_at"`
	UpdatedAt      int64  `json:"updated_at"`
}

func (c *Client) GetJiraInfo() (*JiraInfo, error) {
	url := "/directory/jira"

	jira := &JiraInfo{}
	_, err := c.Get(url, httpclient.SetResult(jira), httpclient.SetQueryParam("orgId", "1"))
	if err != nil {
		return nil, err
	}

	return jira, nil
}

func GetJiraInfo(host, token string) (*JiraInfo, error) {
	c := New(host, token)
	return c.GetJiraInfo()
}
