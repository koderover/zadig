package gitee

import (
	"context"

	"gitee.com/openeuler/go-gitee/gitee"
)

func (c *Client) ListTags(ctx context.Context, owner string, repo string, opts *gitee.GetV5ReposOwnerRepoTagsOpts) (gitee.Tag, error) {
	tags, _, err := c.RepositoriesApi.GetV5ReposOwnerRepoTags(ctx, owner, repo, opts)
	if err != nil {
		return gitee.Tag{}, err
	}
	return tags, err
}
