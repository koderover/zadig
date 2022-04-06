package gitee

import (
	"context"
	"fmt"
	"time"

	"gitee.com/openeuler/go-gitee/gitee"

	"github.com/koderover/zadig/pkg/tool/httpclient"
)

func (c *Client) ListBranches(ctx context.Context, owner, repo string, opts *gitee.GetV5ReposOwnerRepoBranchesOpts) ([]gitee.Branch, error) {
	bs, _, err := c.RepositoriesApi.GetV5ReposOwnerRepoBranches(ctx, owner, repo, opts)
	if err != nil {
		return nil, err
	}
	return bs, nil
}

type Branch struct {
	Name   string       `json:"name"`
	Commit BranchCommit `json:"commit"`
}

type BranchCommit struct {
	Sha    string                      `json:"sha"`
	URL    string                      `json:"url"`
	Commit BranchCommmitInternalCommit `json:"commit"`
}

type BranchCommmitInternalCommit struct {
	Author    BranchAuthor    `json:"author"`
	URL       string          `json:"url"`
	Message   string          `json:"message"`
	Tree      BranchTree      `json:"tree"`
	Committer BranchCommitter `json:"committer"`
}

type BranchAuthor struct {
	Name  string    `json:"name"`
	Date  time.Time `json:"date"`
	Email string    `json:"email"`
}

type BranchTree struct {
	Sha string `json:"sha"`
	URL string `json:"url"`
}

type BranchCommitter struct {
	Name  string    `json:"name"`
	Date  time.Time `json:"date"`
	Email string    `json:"email"`
}

func (c *Client) GetSingleBranch(accessToken, owner, repo, branch string) (*Branch, error) {
	httpClient := httpclient.New(
		httpclient.SetHostURL(GiteeHOSTURL),
	)
	url := fmt.Sprintf("/v5/repos/%s/%s/branches/%s", owner, repo, branch)
	var branchInfo *Branch
	_, err := httpClient.Get(url, httpclient.SetQueryParam("access_token", accessToken), httpclient.SetResult(&branchInfo))
	if err != nil {
		return nil, err
	}
	return branchInfo, nil
}
