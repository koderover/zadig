/*
Copyright 2022 The KodeRover Authors.

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

package gitee

import (
	"context"
	"fmt"
	"time"

	"github.com/koderover/zadig/pkg/tool/httpclient"
)

type Tag struct {
	Name    string `json:"name"`
	Message string `json:"message"`
	Commit  Commit `json:"commit"`
}

type Commit struct {
	Sha  string    `json:"sha"`
	Date time.Time `json:"date"`
}

func (c *Client) ListTags(ctx context.Context, hostURL, accessToken, owner string, repo string) ([]Tag, error) {
	apiHost := fmt.Sprintf("%s/%s", hostURL, "api")
	httpClient := httpclient.New(
		httpclient.SetHostURL(apiHost),
	)
	url := fmt.Sprintf("/v5/repos/%s/%s/tags", owner, repo)
	var tags []Tag
	_, err := httpClient.Get(url, httpclient.SetQueryParam("access_token", accessToken), httpclient.SetResult(&tags))
	if err != nil {
		return nil, err
	}
	return tags, nil
}
