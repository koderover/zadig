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
)

func (c *Client) ListHooks(ctx context.Context, owner, repo string, opts *ListOptions) ([]*github.Hook, error) {
	hooks, err := wrap(paginated(func(o *github.ListOptions) ([]interface{}, *github.Response, error) {
		hs, r, err := c.Repositories.ListHooks(ctx, owner, repo, o)
		var res []interface{}
		for _, h := range hs {
			res = append(res, h)
		}
		return res, r, err
	}, opts))

	var res []*github.Hook
	hs, ok := hooks.([]interface{})
	if !ok {
		return nil, nil
	}
	for _, hook := range hs {
		res = append(res, hook.(*github.Hook))
	}

	return res, err
}

func (c *Client) DeleteHook(ctx context.Context, owner, repo string, id int64) error {
	return wrapError(c.Repositories.DeleteHook(ctx, owner, repo, id))
}

func (c *Client) CreateHook(ctx context.Context, owner, repo string, hook *github.Hook) (*github.Hook, error) {
	created, err := wrap(c.Repositories.CreateHook(ctx, owner, repo, hook))
	if h, ok := created.(*github.Hook); ok {
		return h, err
	}

	return nil, err
}

func (c *Client) CreateStatus(ctx context.Context, owner, repo, ref string, status *github.RepoStatus) (*github.RepoStatus, error) {
	created, err := wrap(c.Repositories.CreateStatus(ctx, owner, repo, ref, status))
	if s, ok := created.(*github.RepoStatus); ok {
		return s, err
	}

	return nil, err
}

func (c *Client) GetContents(ctx context.Context, owner, repo, path string, opts *github.RepositoryContentGetOptions) (*github.RepositoryContent, []*github.RepositoryContent, error) {
	fileContent, directoryContent, resp, err := c.Repositories.GetContents(ctx, owner, repo, path, opts)
	return fileContent, directoryContent, wrapError(resp, err)
}
