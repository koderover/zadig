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

package ilyshin

import (
	"fmt"
	"net/url"

	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/tool/httpclient"
)

type Branch struct {
	Name               string  `json:"name"`
	Protected          bool    `json:"protected"`
	Merged             bool    `json:"merged"`
	Default            bool    `json:"default"`
	CanPush            bool    `json:"can_push"`
	DevelopersCanPush  bool    `json:"developers_can_push"`
	DevelopersCanMerge bool    `json:"developers_can_merge"`
	Commit             *Commit `json:"commit"`
}

func (c *Client) ListBranches(owner, repo string, log *zap.SugaredLogger) ([]*Branch, error) {
	url := fmt.Sprintf("/api/v4/projects/%s/repository/branches", generateProjectName(owner, repo))
	qs := map[string]string{
		"per_page": "100",
	}

	var err error
	var branches []*Branch
	if _, err = c.Get(url, httpclient.SetQueryParams(qs), httpclient.SetResult(&branches)); err != nil {
		log.Errorf("Failed to list project branches, error: %s", err)
		return branches, err
	}

	return branches, nil
}

func generateProjectName(owner, repo string) string {
	return url.PathEscape(fmt.Sprintf("%s/%s", owner, repo))
}
