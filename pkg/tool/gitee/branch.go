package gitee

import (
	"context"

	"gitee.com/openeuler/go-gitee/gitee"
)

func (c *Client) ListBranches(ctx context.Context, owner, repo string, opts *gitee.GetV5ReposOwnerRepoBranchesOpts) ([]gitee.Branch, error) {
	bs, _, err := c.RepositoriesApi.GetV5ReposOwnerRepoBranches(ctx, owner, repo, opts)
	if err != nil {
		return nil, err
	}
	return bs, nil
}
