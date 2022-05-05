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
)

func (c *Client) CreateMergeRequestComment(ctx context.Context, owner, repo string, number int32, pullRequestCommentPostParam gitee.PullRequestCommentPostParam) (gitee.PullRequestComments, error) {
	pullRequestComments, _, err := c.PullRequestsApi.PostV5ReposOwnerRepoPullsNumberComments(ctx, owner, repo, number, pullRequestCommentPostParam)
	if err != nil {
		return gitee.PullRequestComments{}, err
	}
	return pullRequestComments, nil
}

func (c *Client) UpdateMergeRequestComment(ctx context.Context, owner, repo string, id int32, pullRequestCommentPatchParam gitee.PullRequestCommentPatchParam) error {
	_, _, err := c.PullRequestsApi.PatchV5ReposOwnerRepoPullsCommentsId(ctx, owner, repo, id, pullRequestCommentPatchParam)
	if err != nil {
		return err
	}
	return nil
}
