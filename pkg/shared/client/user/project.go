/*
Copyright 2023 The KodeRover Authors.

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

package user

import (
	"github.com/koderover/zadig/pkg/tool/httpclient"
)

type InitializeProjectResp struct {
	Roles []string `json:"roles"`
}

func (c *Client) InitializeProject(projectKey string, isPublic bool, admins []string) error {
	url := "policy/internal/initializeProject"

	body := map[string]interface{}{
		"namespace": projectKey,
		"is_public": isPublic,
		"admins":    admins,
	}

	_, err := c.Post(url, httpclient.SetBody(body))
	if err != nil {
		return err
	}
	return nil
}
