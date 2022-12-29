/*
 * Copyright 2022 The KodeRover Authors.
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

package jira

import (
	"github.com/pkg/errors"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/tool/jira"
)

type JiraInfo struct {
	ID             int64  `json:"id"`
	Host           string `json:"host"`
	User           string `json:"user"`
	AccessToken    string `json:"access_token"`
	OrganizationID int    `json:"organizationId"`
	UpdatedAt      int64  `json:"updated_at"`
}

func GetJiraInfo() (*JiraInfo, error) {
	resp, err := mongodb.NewProjectManagementColl().GetJira()
	if err != nil {
		return nil, err
	}
	// since in some case, db will return no error even if it does not have anything, we simply do a compatibility change
	if resp == nil {
		return nil, nil
	}

	jira := &JiraInfo{
		Host:        resp.JiraHost,
		User:        resp.JiraUser,
		AccessToken: resp.JiraToken,
		UpdatedAt:   resp.UpdatedAt,
	}

	return jira, nil
}

func SendComment(key, message, link, linkTitle string) error {
	info, err := GetJiraInfo()
	if err != nil {
		return errors.Wrap(err, "get jira info")
	}
	client := jira.NewJiraClient(info.User, info.AccessToken, info.Host)
	return client.Issue.AddComment(key, message, link, linkTitle)
}
