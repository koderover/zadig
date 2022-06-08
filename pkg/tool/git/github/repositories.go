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
	"strings"

	"github.com/27149chen/afero"
	"github.com/google/go-github/v35/github"

	"github.com/koderover/zadig/pkg/tool/git"
	"github.com/koderover/zadig/pkg/util"
	fsutil "github.com/koderover/zadig/pkg/util/fs"
)

func (c *Client) ListRepositoriesForAuthenticatedUser(ctx context.Context, user string, opts *ListOptions) ([]*github.Repository, error) {
	repositories, err := wrap(paginated(func(o *github.ListOptions) ([]interface{}, *github.Response, error) {
		// Note. parameter user is not used when list repositories because private repos will NOT be in response data from gitHub even user is the exact owner of these private repos
		rs, r, err := c.Repositories.List(ctx, "", &github.RepositoryListOptions{ListOptions: *o})
		var res []interface{}
		for _, r := range rs {
			res = append(res, r)
		}
		return res, r, err
	}, opts))

	if err != nil {
		return nil, err
	}

	var res []*github.Repository
	rs, ok := repositories.([]interface{})
	if !ok {
		return nil, nil
	}
	for _, r := range rs {
		res = append(res, r.(*github.Repository))
	}

	return res, err
}

func (c *Client) ListBranches(ctx context.Context, owner, repo string, opts *ListOptions) ([]*github.Branch, error) {
	branches, err := wrap(paginated(func(o *github.ListOptions) ([]interface{}, *github.Response, error) {
		bs, r, err := c.Repositories.ListBranches(ctx, owner, repo, &github.BranchListOptions{ListOptions: *o})
		var res []interface{}
		for _, b := range bs {
			res = append(res, b)
		}
		return res, r, err
	}, opts))

	if err != nil {
		return nil, err
	}

	var res []*github.Branch
	bs, ok := branches.([]interface{})
	if !ok {
		return nil, nil
	}
	for _, b := range bs {
		res = append(res, b.(*github.Branch))
	}

	return res, err
}

func (c *Client) ListTags(ctx context.Context, owner, repo string, opts *ListOptions) ([]*github.RepositoryTag, error) {
	tags, err := wrap(paginated(func(o *github.ListOptions) ([]interface{}, *github.Response, error) {
		ts, r, err := c.Repositories.ListTags(ctx, owner, repo, o)
		var res []interface{}
		for _, t := range ts {
			res = append(res, t)
		}
		return res, r, err
	}, opts))

	if err != nil {
		return nil, err
	}

	var res []*github.RepositoryTag
	ts, ok := tags.([]interface{})
	if !ok {
		return nil, nil
	}
	for _, t := range ts {
		res = append(res, t.(*github.RepositoryTag))
	}

	return res, err
}

func (c *Client) ListHooks(ctx context.Context, owner, repo string, opts *ListOptions) ([]*github.Hook, error) {
	hooks, err := wrap(paginated(func(o *github.ListOptions) ([]interface{}, *github.Response, error) {
		hs, r, err := c.Repositories.ListHooks(ctx, owner, repo, o)
		var res []interface{}
		for _, h := range hs {
			res = append(res, h)
		}
		return res, r, err
	}, opts))

	if err != nil {
		return nil, err
	}

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

func (c *Client) ListReleases(ctx context.Context, owner, repo string, opts *ListOptions) ([]*github.RepositoryRelease, error) {
	releases, err := wrap(paginated(func(o *github.ListOptions) ([]interface{}, *github.Response, error) {
		hs, r, err := c.Repositories.ListReleases(ctx, owner, repo, o)
		var res []interface{}
		for _, h := range hs {
			res = append(res, h)
		}
		return res, r, err
	}, opts))

	if err != nil {
		return nil, err
	}

	var res []*github.RepositoryRelease
	hs, ok := releases.([]interface{})
	if !ok {
		return nil, nil
	}
	for _, hook := range hs {
		res = append(res, hook.(*github.RepositoryRelease))
	}

	return res, err
}

func (c *Client) ListRepositoryCommits(ctx context.Context, owner, repo, path, branch string, opts *ListOptions) ([]*github.RepositoryCommit, error) {
	releases, err := wrap(paginated(func(o *github.ListOptions) ([]interface{}, *github.Response, error) {
		hs, r, err := c.Repositories.ListCommits(ctx, owner, repo, &github.CommitsListOptions{Path: path, SHA: branch, ListOptions: *o})
		var res []interface{}
		for _, h := range hs {
			res = append(res, h)
		}
		return res, r, err
	}, opts))

	if err != nil {
		return nil, err
	}

	var res []*github.RepositoryCommit
	hs, ok := releases.([]interface{})
	if !ok {
		return nil, nil
	}
	for _, hook := range hs {
		res = append(res, hook.(*github.RepositoryCommit))
	}

	return res, err
}

func (c *Client) GetLatestRepositoryCommit(ctx context.Context, owner, repo, path, branch string) (*github.RepositoryCommit, error) {
	cs, err := c.ListRepositoryCommits(ctx, owner, repo, path, branch, &ListOptions{PerPage: 1, NoPaginated: true})
	if err != nil || len(cs) == 0 {
		return nil, err
	}

	return cs[0], nil
}

func (c *Client) DeleteHook(ctx context.Context, owner, repo string, id int64) error {
	return wrapError(c.Repositories.DeleteHook(ctx, owner, repo, id))
}

func (c *Client) CreateHook(ctx context.Context, owner, repo string, hook *git.Hook) (*github.Hook, error) {
	h := &github.Hook{
		Config: map[string]interface{}{
			"url":          hook.URL,
			"content_type": "json",
			"secret":       hook.Secret,
		},
		Events: hook.Events,
		Active: hook.Active,
	}
	created, err := wrap(c.Repositories.CreateHook(ctx, owner, repo, h))
	if err != nil {
		return nil, err
	}

	res, ok := created.(*github.Hook)
	if !ok {
		return nil, fmt.Errorf("object is not a github Hook")
	}

	return res, nil
}

func (c *Client) UpdateHook(ctx context.Context, owner, repo string, id int64, hook *git.Hook) (*github.Hook, error) {
	h := &github.Hook{
		Config: map[string]interface{}{
			"url":          hook.URL,
			"content_type": "json",
			"secret":       hook.Secret,
		},
	}
	if len(hook.Events) > 0 {
		h.Events = hook.Events
	}
	if hook.Active != nil {
		h.Active = hook.Active
	}
	updated, err := wrap(c.Repositories.EditHook(ctx, owner, repo, id, h))
	if err != nil {
		return nil, err
	}

	res, ok := updated.(*github.Hook)
	if !ok {
		return nil, fmt.Errorf("object is not a github Hook")
	}

	return res, nil
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

// GetYAMLContents recursively gets all yaml contents under the given path. if split is true, manifests in the same file
// will be split to separated ones.
func (c *Client) GetYAMLContents(ctx context.Context, owner, repo, path, branch string, split bool) ([]string, error) {
	fileContent, directoryContent, err := c.GetContents(ctx, owner, repo, path, &github.RepositoryContentGetOptions{Ref: branch})
	if err != nil {
		return nil, err
	}

	var res []string
	if fileContent != nil {
		if strings.HasSuffix(fileContent.GetPath(), ".yaml") || strings.HasSuffix(fileContent.GetPath(), ".yml") {
			content, _ := fileContent.GetContent()
			if split {
				res = util.SplitManifests(content)
			} else {
				res = []string{content}
			}

		}

		return res, nil
	}

	for _, fileOrDir := range directoryContent {
		switch fileOrDir.GetType() {
		case "file":
			if strings.HasSuffix(fileOrDir.GetPath(), ".yaml") || strings.HasSuffix(fileOrDir.GetPath(), ".yml") {
				r, err := c.GetYAMLContents(ctx, owner, repo, fileOrDir.GetPath(), branch, split)
				if err != nil {
					return nil, err
				}
				res = append(res, r...)
			}
		case "dir":
			tree, err := c.GetTree(ctx, owner, repo, fileOrDir.GetSHA(), true)
			if err != nil {
				return nil, err
			}
			if tree == nil {
				continue
			}

			for _, ent := range tree.Entries {
				if ent.GetType() == "blob" && (strings.HasSuffix(ent.GetPath(), ".yaml") || strings.HasSuffix(ent.GetPath(), ".yml")) {
					r, err := c.GetYAMLContents(ctx, owner, repo, fmt.Sprintf("%s/%s", fileOrDir.GetPath(), ent.GetPath()), branch, split)
					if err != nil {
						return nil, err
					}
					res = append(res, r...)
				}
			}
		}
	}

	return res, nil
}

// GetTreeContents recursively gets all file contents under the given path, and writes to an in-memory file system.
func (c *Client) GetTreeContents(ctx context.Context, owner, repo, path, branch string) (afero.Fs, error) {
	fs := afero.NewMemMapFs()
	err := c.getTreeContents(ctx, owner, repo, branch, path, path, fs)
	if err != nil {
		return nil, err
	}

	return fs, nil
}

func (c *Client) getTreeContents(ctx context.Context, owner, repo, branch, base, path string, fs afero.Fs) error {
	fileContent, directoryContent, err := c.GetContents(ctx, owner, repo, path, &github.RepositoryContentGetOptions{Ref: branch})
	if err != nil {
		return err
	}

	if fileContent != nil {
		content, err := fileContent.GetContent()
		if err != nil {
			return err
		}
		return afero.WriteFile(fs, fsutil.ShortenFileBase(base, path), []byte(content), 0644)
	}

	for _, fileOrDir := range directoryContent {
		switch fileOrDir.GetType() {
		case "file":
			err = c.getTreeContents(ctx, owner, repo, branch, base, fileOrDir.GetPath(), fs)
			if err != nil {
				return err
			}

		case "dir":
			tree, err := c.GetTree(ctx, owner, repo, fileOrDir.GetSHA(), true)
			if err != nil {
				return err
			}
			if tree == nil {
				continue
			}

			for _, ent := range tree.Entries {
				if ent.GetType() == "blob" {
					err = c.getTreeContents(ctx, owner, repo, branch, base, fmt.Sprintf("%s/%s", fileOrDir.GetPath(), ent.GetPath()), fs)
					if err != nil {
						return err
					}
				}
			}
		}
	}

	return nil
}
