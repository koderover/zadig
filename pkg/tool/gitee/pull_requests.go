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

	"gitee.com/openeuler/go-gitee/gitee"
	"github.com/antihax/optional"
)

func (c *Client) GetPullRequest(ctx context.Context, owner string, repo string, number int) (gitee.PullRequest, error) {
	pr, _, err := c.PullRequestsApi.GetV5ReposOwnerRepoPullsNumber(ctx, owner, repo, int32(number), nil)
	if err != nil {
		return gitee.PullRequest{}, err
	}

	return pr, err
}

func (c *Client) ListPullRequests(ctx context.Context, owner string, repo string, fetchAll bool) ([]gitee.PullRequest, error) {
	opts := &gitee.GetV5ReposOwnerRepoPullsOpts{PerPage: optional.NewInt32(100)}
	prs := make([]gitee.PullRequest, 0)
	for page := int32(1); ; page++ {
		if fetchAll {
			opts.Page = optional.NewInt32(page)
		}
		result, _, err := c.PullRequestsApi.GetV5ReposOwnerRepoPulls(ctx, owner, repo, opts)
		if err != nil {
			return nil, err
		}
		prs = append(prs, result...)
		if !fetchAll || len(result) < 100 {
			break
		}
	}

	return prs, nil
}

func (c *Client) ListCommitsForPR(ctx context.Context, owner string, repo string, number int, opts *gitee.GetV5ReposOwnerRepoPullsNumberCommitsOpts) ([]gitee.PullRequestCommits, error) {
	cs, _, err := c.PullRequestsApi.GetV5ReposOwnerRepoPullsNumberCommits(ctx, owner, repo, int32(number), opts)
	if err != nil {
		return nil, err
	}

	return cs, err
}

func (c *Client) ListFiles(ctx context.Context, owner string, repo string, number int, opts *gitee.GetV5ReposOwnerRepoPullsNumberFilesOpts) ([]gitee.PullRequestFiles, error) {
	is, _, err := c.PullRequestsApi.GetV5ReposOwnerRepoPullsNumberFiles(ctx, owner, repo, int32(number), opts)
	if err != nil {
		return nil, err
	}
	return is, err
}
