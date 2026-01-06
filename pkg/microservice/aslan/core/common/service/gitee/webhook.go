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
	"strconv"

	gitservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/git"
	"github.com/koderover/zadig/v2/pkg/tool/git"
	"github.com/koderover/zadig/v2/pkg/util"
)

func (c *Client) CreateWebHook(owner, repo string) (string, error) {
	hook, err := c.CreateHook(c.Address, c.AccessToken, owner, repo, &git.Hook{
		URL:    gitservice.WebHookURL(),
		Secret: util.GetGitHookSecret(),
	})
	if err != nil {
		return "", err
	}

	return strconv.Itoa(int(hook.ID)), nil
}

func (c *Client) DeleteWebHook(owner, repo, hookID string) error {
	// special case when the webhook is created manually, we don't delete it
	if hookID == "" {
		return nil
	}

	hookIDInt, err := strconv.ParseInt(hookID, 10, 64)
	if err != nil {
		return err
	}
	return c.DeleteHook(context.TODO(), owner, repo, hookIDInt)
}
