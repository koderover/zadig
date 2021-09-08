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

type Tag struct {
	Name    string       `json:"name"`
	Message string       `json:"message"`
	Release *ReleaseNote `json:"release"`
	Commit  *Commit      `json:"commit"`
}

type ReleaseNote struct {
	TagName     string `json:"tag_name"`
	Description string `json:"description"`
}

func (c *Client) ListTags(owner, repo string, log *zap.SugaredLogger) ([]*Tag, error) {
	url := fmt.Sprintf("/api/v4/projects/%s/repository/tags", generateProjectName(owner, repo))
	qs := map[string]string{
		"per_page": "100",
	}

	var err error
	var tags []*Tag
	if _, err = c.Get(url, httpclient.SetQueryParams(qs), httpclient.SetResult(&tags)); err != nil {
		log.Errorf("Failed to list project tags, error: %s", err)
		return tags, err
	}

	return tags, nil
}
