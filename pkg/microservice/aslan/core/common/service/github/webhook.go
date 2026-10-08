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

package github

import (
	"context"
	"fmt"
	githubapi "github.com/google/go-github/v35/github"
	"strconv"

	gitservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/git"
	"github.com/koderover/zadig/v2/pkg/tool/git"
	"github.com/koderover/zadig/v2/pkg/util"
)

func (c *Client) CreateWebHook(owner, repo string) (string, error) {
	hook, err := c.CreateHook(context.TODO(), owner, repo, &git.Hook{
		URL:    gitservice.WebHookURL(),
		Secret: util.GetGitHookSecret(),
		Events: []string{git.PushEvent, git.PullRequestEvent, git.BranchOrTagCreateEvent, git.CheckRunEvent},
	})
	if err != nil {
		return "", err
	}

	return strconv.Itoa(int(hook.GetID())), nil
}

// EnsureManagedReviewThreadHook preserves existing configuration and only adds
// the missing event. All matching Zadig callbacks are checked before creation.
func EnsureManagedReviewThreadHook(ctx context.Context, cli *githubapi.Client, owner, repo, hookURL string) (string, error) {
	var hookID int64
	opts := &githubapi.ListOptions{PerPage: 100}
	for {
		hooks, resp, err := cli.Repositories.ListHooks(ctx, owner, repo, opts)
		if err != nil {
			return "", err
		}
		for _, hook := range hooks {
			if hook == nil || hook.Config["url"] != hookURL {
				continue
			}
			hookID = hook.GetID()
			enabled := false
			for _, event := range hook.Events {
				if event == git.PullRequestReviewThreadEvent || event == "*" {
					enabled = true
					break
				}
			}
			if enabled {
				continue
			}
			req, err := cli.NewRequest("PATCH", fmt.Sprintf("repos/%s/%s/hooks/%d", owner, repo, hookID), map[string]interface{}{"add_events": []string{git.PullRequestReviewThreadEvent}})
			if err != nil {
				return "", err
			}
			if _, err := cli.Do(ctx, req, nil); err != nil {
				return "", err
			}
		}
		if resp == nil || resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	if hookID > 0 {
		return strconv.FormatInt(hookID, 10), nil
	}
	hook, _, err := cli.Repositories.CreateHook(ctx, owner, repo, &githubapi.Hook{
		Config: map[string]interface{}{"url": hookURL, "content_type": "json", "secret": util.GetGitHookSecret()},
		Events: []string{git.PushEvent, git.PullRequestEvent, git.BranchOrTagCreateEvent, git.CheckRunEvent, git.PullRequestReviewThreadEvent},
		Active: githubapi.Bool(true),
	})
	if err != nil {
		return "", err
	}
	return strconv.FormatInt(hook.GetID(), 10), nil
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

func (c *Client) RefreshWebHookSecret(secret, owner, repo, hookID string) error {
	// special case when the webhook is created manually, we don't delete it
	if hookID == "" {
		return nil
	}

	hookIDInt, err := strconv.ParseInt(hookID, 10, 64)
	if err != nil {
		return err
	}

	webhookURL := gitservice.WebHookURL()
	_, err = c.UpdateHook(context.TODO(), owner, repo, hookIDInt, &git.Hook{
		URL:    webhookURL,
		Secret: secret,
	})

	return err
}
