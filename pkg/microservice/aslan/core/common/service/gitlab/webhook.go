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
	"context"
	"fmt"
	"strconv"
	"time"

	gitservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/git"
	"github.com/koderover/zadig/v2/pkg/tool/git"
	"github.com/koderover/zadig/v2/pkg/util"
	"github.com/xanzy/go-gitlab"
)

func (c *Client) CreateWebHook(owner, repo string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	return EnsureManagedEmojiHook(ctx, c.Client.Client, owner+"/"+repo, gitservice.WebHookURL())
}

// EnsureManagedEmojiHook adds Emoji events to existing Zadig hooks, or creates
// a hook with the usual workflow events and Emoji events when none exists.
func EnsureManagedEmojiHook(ctx context.Context, cli *gitlab.Client, project, hookURL string) (string, error) {
	var hookID int
	opts := &gitlab.ListProjectHooksOptions{PerPage: 100}
	for {
		hooks, resp, err := cli.Projects.ListProjectHooks(project, opts, gitlab.WithContext(ctx))
		if err != nil {
			return "", err
		}
		for _, hook := range hooks {
			if hook.URL != hookURL {
				continue
			}
			hookID = hook.ID
			req, err := cli.NewRequest("PUT", fmt.Sprintf("projects/%s/hooks/%d", gitlab.PathEscape(project), hook.ID), map[string]interface{}{"url": hook.URL, "emoji_events": true}, []gitlab.RequestOptionFunc{gitlab.WithContext(ctx)})
			if err != nil {
				return "", err
			}
			if _, err = cli.Do(req, nil); err != nil {
				return "", err
			}
		}
		if resp == nil || resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	if hookID > 0 {
		return strconv.Itoa(hookID), nil
	}
	req, err := cli.NewRequest("POST", fmt.Sprintf("projects/%s/hooks", gitlab.PathEscape(project)), map[string]interface{}{
		"url": hookURL, "token": util.GetGitHookSecret(), "push_events": true,
		"merge_requests_events": true, "tag_push_events": true, "emoji_events": true,
	}, []gitlab.RequestOptionFunc{gitlab.WithContext(ctx)})
	if err != nil {
		return "", err
	}
	var hook gitlab.ProjectHook
	if _, err = cli.Do(req, &hook); err != nil {
		return "", err
	}
	return strconv.Itoa(hook.ID), nil
}

func (c *Client) DeleteWebHook(owner, repo, hookID string) error {
	// special case when the webhook is created manually, we don't delete it
	if hookID == "" {
		return nil
	}

	hookIDInt, err := strconv.Atoi(hookID)
	if err != nil {
		return err
	}
	return c.DeleteProjectHook(owner, repo, hookIDInt)
}

func (c *Client) RefreshWebHookSecret(secret, owner, repo, hookID string) error {
	// special case when the webhook is created manually, we don't delete it
	if hookID == "" {
		return nil
	}

	hookIDInt, err := strconv.Atoi(hookID)
	if err != nil {
		return err
	}
	_, err = c.UpdateProjectHook(owner, repo, hookIDInt, &git.Hook{
		URL:    gitservice.WebHookURL(),
		Secret: secret,
	})
	return err
}
