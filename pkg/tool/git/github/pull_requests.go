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

	"github.com/google/go-github/v35/github"

	"github.com/koderover/zadig/v2/pkg/tool/log"
)

func (c *Client) GetPullRequest(ctx context.Context, owner string, repo string, number int) (*github.PullRequest, error) {
	pr, err := wrap(c.PullRequests.Get(ctx, owner, repo, number))
	if p, ok := pr.(*github.PullRequest); ok {
		return p, err
	}

	return nil, err
}

func (c *Client) ListPullRequests(ctx context.Context, owner string, repo string, opts *github.PullRequestListOptions) ([]*github.PullRequest, error) {
	prs, err := wrap(c.PullRequests.List(ctx, owner, repo, opts))
	if p, ok := prs.([]*github.PullRequest); ok {
		return p, err
	}

	return nil, err
}

func (c *Client) ListCommitsForPR(ctx context.Context, owner string, repo string, number int, opts *ListOptions) ([]*github.RepositoryCommit, error) {
	commits, err := wrap(paginated(func(o *github.ListOptions) ([]interface{}, *github.Response, error) {
		cs, r, err := c.PullRequests.ListCommits(ctx, owner, repo, number, o)
		var res []interface{}
		for _, c := range cs {
			res = append(res, c)
		}
		return res, r, err
	}, opts))

	if err != nil {
		return nil, err
	}

	var res []*github.RepositoryCommit
	cs, ok := commits.([]interface{})
	if !ok {
		return nil, nil
	}
	for _, c := range cs {
		res = append(res, c.(*github.RepositoryCommit))
	}

	return res, err
}

func (c *Client) ListCommitsForBranch(ctx context.Context, owner, repo, branch string, opts *ListOptions) ([]*github.RepositoryCommit, error) {
	paginationOpts := &github.ListOptions{
		Page:    1,
		PerPage: 100,
	}
	if opts != nil {
		paginationOpts.Page = opts.Page
		paginationOpts.PerPage = opts.PerPage
	}
	listOpts := &github.CommitsListOptions{
		SHA:         branch,
		ListOptions: *paginationOpts,
	}
	commits, err := wrap(c.Repositories.ListCommits(ctx, owner, repo, listOpts))

	if err != nil {
		return nil, err
	}

	var res []*github.RepositoryCommit
	cs, ok := commits.([]*github.RepositoryCommit)
	if !ok {
		log.Infof("failed to convert resp into repository commit")
		return nil, nil
	}
	for _, c := range cs {
		res = append(res, c)
	}

	return res, err
}

func (c *Client) ListFiles(ctx context.Context, owner string, repo string, number int, opts *ListOptions) ([]*github.CommitFile, error) {
	files, err := wrap(paginated(func(o *github.ListOptions) ([]interface{}, *github.Response, error) {
		is, r, err := c.PullRequests.ListFiles(ctx, owner, repo, number, o)
		var res []interface{}
		for _, i := range is {
			res = append(res, i)
		}
		return res, r, err
	}, opts))

	if err != nil {
		return nil, err
	}

	var res []*github.CommitFile
	fs, ok := files.([]interface{})
	if !ok {
		return nil, nil
	}
	for _, file := range fs {
		res = append(res, file.(*github.CommitFile))
	}

	return res, err
}
