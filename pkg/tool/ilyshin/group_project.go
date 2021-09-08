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

	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/tool/httpclient"
)

func (c *Client) ListGroupProjects(namespace, keyword string, log *zap.SugaredLogger) ([]*Project, error) {
	url := fmt.Sprintf("/api/v4/groups/%s/projects", namespace)
	qs := map[string]string{
		"order_by": "name",
		"sort":     "asc",
		"per_page": "100",
	}
	if keyword != "" && len(keyword) > 2 {
		qs["search"] = keyword
	}
	var err error
	var gps []*Project
	if _, err = c.Get(url, httpclient.SetQueryParams(qs), httpclient.SetResult(&gps)); err != nil {
		log.Errorf("Failed to list group projects, error: %s", err)
		return gps, err
	}

	return gps, nil
}
