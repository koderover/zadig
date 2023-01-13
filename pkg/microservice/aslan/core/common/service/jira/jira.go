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
	"net/http"

	"github.com/imroc/req/v3"
	"github.com/pkg/errors"

	"github.com/koderover/zadig/pkg/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/jira"
	"github.com/koderover/zadig/pkg/tool/log"
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
	resp, err := req.C().R().Get(config.AslanServiceAddress() + "/api/system/project_management")
	if err != nil {
		log.Errorf("GetJiraInfo: send request error %v", err)
		return nil, errors.Wrap(err, "send request")
	}
	if resp.GetStatusCode() != http.StatusOK {
		log.Errorf("GetJiraInfo: unexpected status code %d", resp.GetStatusCode())
		return nil, errors.Errorf("unexpected status code %d", resp.GetStatusCode())
	}
	var list []*commonmodels.ProjectManagement
	if err := resp.UnmarshalJson(&list); err != nil {
		log.Errorf("GetJiraInfo: unmarshal error %v", err)
		return nil, errors.Wrap(err, "unmarshal")
	}

	for _, v := range list {
		if v.Type == setting.PMJira {
			return &JiraInfo{
				Host:        v.JiraHost,
				User:        v.JiraUser,
				AccessToken: v.JiraToken,
				UpdatedAt:   v.UpdatedAt,
			}, nil
		}
	}
	log.Warnf("GetJiraInfo: not found")
	return nil, errors.New("not found")
}

func SendComment(key, message string) error {
	info, err := GetJiraInfo()
	if err != nil {
		return errors.Wrap(err, "get jira info")
	}
	client := jira.NewJiraClient(info.User, info.AccessToken, info.Host)
	return client.Issue.AddCommentV2(key, message)
}
