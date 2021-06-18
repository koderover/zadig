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
	"fmt"

	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/tool/httpclient"
)

type Team struct {
	ID           int         `json:"id"`
	OrgID        int         `json:"orgId"`
	Name         string      `json:"name"`
	Desc         string      `json:"desc"`
	IsTeamLeader bool        `json:"isTeamLeader"`
	Users        []*UserInfo `json:"leaders"`
	CreatedAt    int64       `json:"created_at"`
	UpdatedAt    int64       `json:"updated_at"`
}

func (c *Client) ListTeams(orgID int, log *zap.SugaredLogger) ([]*Team, error) {
	url := "/directory/teamss/search"
	resp := make([]*Team, 0)
	_, err := c.Get(url, httpclient.SetQueryParam("orgId", fmt.Sprintf("%d", orgID)), httpclient.SetResult(&resp))

	if err != nil {
		log.Errorf("ListTeams error: %v", err)
		return nil, err
	}

	return resp, nil
}

func (c *Client) GetTeam(teamID int) (*Team, error) {
	url := fmt.Sprintf("/directory/teams/%d", teamID)
	resp := new(Team)
	_, err := c.Get(url, httpclient.SetResult(&resp))

	if err != nil {
		return nil, err
	}

	return resp, nil
}
