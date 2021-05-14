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
	"encoding/json"

	"github.com/pkg/errors"
	"github.com/qiniu/x/log.v7"
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

func GetJiraInfo(poetryApiServer, ApiRootKey string) (*JiraInfo, error) {
	client := NewPoetryServer(poetryApiServer, ApiRootKey)
	header := client.GetRootTokenHeader()
	header.Set("content-type", "application/json")
	data, err := client.Do("/directory/jira?orgId=1", "GET", nil, header)
	if err != nil {
		log.Error("GetJiraInfo err :", err)
		return nil, errors.WithStack(err)
	}

	var Jira *JiraInfo
	if err := json.Unmarshal(data, &Jira); err != nil {
		log.Error("GetJiraInfo Unmarshal err :", err)
		return nil, errors.WithMessage(err, string(data))
	}

	return Jira, nil
}
