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
	"encoding/base64"
	"strings"

	"github.com/27149chen/afero"
	"github.com/xanzy/go-gitlab"

	"github.com/koderover/zadig/pkg/util"
	fsutil "github.com/koderover/zadig/pkg/util/fs"
)

func (c *Client) ListTree(owner, repo, path, branch string, recursive bool, opts *ListOptions) ([]*gitlab.TreeNode, error) {
	nodes, err := wrap(paginated(func(o *gitlab.ListOptions) ([]interface{}, *gitlab.Response, error) {
		popts := &gitlab.ListTreeOptions{
			ListOptions: *o,
			Ref:         &branch,
			Path:        &path,
			Recursive:   &recursive,
		}

		ns, r, err := c.Repositories.ListTree(generateProjectName(owner, repo), popts)
		var res []interface{}
		for _, n := range ns {
			res = append(res, n)
		}
		return res, r, err
	}, opts))

	if err != nil {
		return nil, err
	}

	var res []*gitlab.TreeNode
	ns, ok := nodes.([]interface{})
	if !ok {
		return nil, nil
	}
	for _, s := range ns {
		res = append(res, s.(*gitlab.TreeNode))
	}

	return res, err
}

func (c *Client) GetRawFile(owner, repo string, sha string, fileName string) ([]byte, error) {
	opts := &gitlab.GetFileOptions{
		Ref: &sha,
	}

	f, err := wrap(c.RepositoryFiles.GetFile(generateProjectName(owner, repo), fileName, opts))
	if err != nil {
		return nil, err
	}
	file, ok := f.(*gitlab.File)
	if !ok {
		return nil, err
	}
	ct, err := wrap(c.Repositories.RawBlobContent(generateProjectName(owner, repo), file.BlobID))
	if t, ok := ct.([]byte); ok {
		return t, err
	}

	return nil, err
}

func (c *Client) GetFileContent(owner, repo, path, branch string) ([]byte, error) {
	opts := &gitlab.GetFileOptions{
		Ref: gitlab.String(branch),
	}
	f, err := wrap(c.RepositoryFiles.GetFile(generateProjectName(owner, repo), path, opts))
	if err != nil {
		return nil, err
	}
	file, ok := f.(*gitlab.File)
	if !ok {
		return nil, err
	}

	return base64.StdEncoding.DecodeString(file.Content)
}

func (c *Client) Compare(projectID int, from, to string) ([]*gitlab.Diff, error) {
	opts := &gitlab.CompareOptions{
		From: &from,
		To:   &to,
	}

	compare, err := wrap(c.Repositories.Compare(projectID, opts))
	if err != nil {
		return nil, err
	}
	if cp, ok := compare.(*gitlab.Compare); ok {
		return cp.Diffs, nil
	}

	return nil, err
}

// GetYAMLContents recursively gets all yaml contents under the given path. if split is true, manifests in the same file
// will be split to separated ones.
func (c *Client) GetYAMLContents(owner, repo, path, branch string, isDir, split bool) ([]string, error) {
	var res []string
	if !isDir {
		if !(strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml")) {
			return nil, nil
		}

		ct, err := c.GetFileContent(owner, repo, path, branch)
		if err != nil {
			return nil, err
		}

		content := string(ct)
		if split {
			res = util.SplitManifests(content)
		} else {
			res = []string{content}
		}

		return res, nil
	}

	treeNodes, err := c.ListTree(owner, repo, path, branch, true, nil)
	if err != nil {
		return nil, err
	}

	for _, tn := range treeNodes {
		if tn.Type != "blob" {
			continue
		}
		r, err := c.GetYAMLContents(owner, repo, tn.Path, branch, false, split)
		if err != nil {
			return nil, err
		}

		res = append(res, r...)
	}

	return res, nil
}

// GetTreeContents recursively gets all file contents under the given path, and writes to an in-memory file system.
func (c *Client) GetTreeContents(owner, repo, path, branch string) (afero.Fs, error) {
	fs := afero.NewMemMapFs()
	err := c.getTreeContents(owner, repo, branch, path, path, true, fs)
	if err != nil {
		return nil, err
	}

	return fs, nil
}

func (c *Client) getTreeContents(owner, repo, branch, base, path string, isDir bool, fs afero.Fs) error {
	if !isDir {
		ct, err := c.GetFileContent(owner, repo, path, branch)
		if err != nil {
			return err
		}

		return afero.WriteFile(fs, fsutil.ShortenFileBase(base, path), ct, 0644)
	}

	treeNodes, err := c.ListTree(owner, repo, path, branch, true, nil)
	if err != nil {
		return err
	}

	for _, tn := range treeNodes {
		if tn.Type != "blob" {
			continue
		}
		err = c.getTreeContents(owner, repo, branch, base, tn.Path, false, fs)
		if err != nil {
			return err
		}
	}

	return nil
}
