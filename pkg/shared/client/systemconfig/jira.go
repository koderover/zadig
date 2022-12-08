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
	"github.com/koderover/zadig/pkg/microservice/systemconfig/core/jira/service"
	"github.com/koderover/zadig/pkg/tool/log"
)

type JiraInfo struct {
	ID             int64  `json:"id"`
	Host           string `json:"host"`
	User           string `json:"user"`
	AccessToken    string `json:"access_token"`
	OrganizationID int    `json:"organizationId"`
	CreatedAt      int64  `json:"created_at"`
	UpdatedAt      int64  `json:"updated_at"`
}

func (c *Client) GetJiraInfo() (*JiraInfo, error) {
	resp, err := service.GeJiraInternal(log.SugaredLogger())
	if err != nil {
		return nil, err
	}
	// since in some case, db will return no error even if it does not have anything, we simply do a compatibility change
	if resp == nil {
		return nil, nil
	}

	jira := &JiraInfo{
		Host:        resp.Host,
		User:        resp.User,
		AccessToken: resp.AccessToken,
		CreatedAt:   resp.CreatedAt,
		UpdatedAt:   resp.UpdatedAt,
	}

	return jira, nil
}
