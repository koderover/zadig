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

	"github.com/xanzy/go-gitlab"
)

func (c *Client) ListTree(owner, repo string, ref string, path string) ([]*gitlab.TreeNode, error) {
	// Recursive default value is false,
	opts := &gitlab.ListTreeOptions{
		Ref:  &ref,
		Path: &path,
	}

	tn, err := wrap(c.Repositories.ListTree(generateProjectName(owner, repo), opts))
	if t, ok := tn.([]*gitlab.TreeNode); ok {
		return t, err
	}

	return nil, err
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

func (c *Client) GetFileContent(owner, repo string, ref, path string) ([]byte, error) {
	opts := &gitlab.GetFileOptions{
		Ref: gitlab.String(ref),
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
