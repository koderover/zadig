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

package gitlab

import (
	"strconv"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	gitservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/git"
	"github.com/koderover/zadig/pkg/tool/git"
)

func (c *Client) CreateWebHook(owner, repo string) (string, error) {
	projectHook, err := c.AddProjectHook(owner, repo, &git.Hook{
		URL:    config.WebHookURL(),
		Secret: gitservice.GetHookSecret(),
		Events: []string{git.PushEvent, git.PullRequestEvent, git.BranchOrTagCreateEvent},
	})

	return strconv.Itoa(projectHook.ID), err
}

func (c *Client) DeleteWebHook(owner, repo, hookID string) error {
	hookIDInt, err := strconv.Atoi(hookID)
	if err != nil {
		return err
	}
	return c.DeleteProjectHook(owner, repo, hookIDInt)
}
