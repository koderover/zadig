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

func (c *Client) ListTree(projectID int, ref string, path string) ([]*gitlab.TreeNode, error) {
	// Recursive default value is false,
	opts := &gitlab.ListTreeOptions{
		Ref:  &ref,
		Path: &path,
	}

	nodes, _, err := c.Repositories.ListTree(projectID, opts)
	return nodes, err
}

func (c *Client) GetRawFile(projectID int, sha string, fileName string) (content []byte, err error) {
	content = make([]byte, 0)
	opts := &gitlab.GetFileOptions{
		Ref: &sha,
	}

	file, _, err := c.RepositoryFiles.GetFile(projectID, fileName, opts)
	if err != nil {
		return content, err
	}
	content, _, err = c.Repositories.RawBlobContent(projectID, file.BlobID)
	return content, err
}

func (c *Client) GetFileContent(projectID int, ref, path string) ([]byte, error) {
	opts := &gitlab.GetFileOptions{
		Ref: gitlab.String(ref),
	}
	fileContent, _, err := c.RepositoryFiles.GetFile(projectID, path, opts)
	if err != nil {
		return nil, err
	}
	content, err := base64.StdEncoding.DecodeString(fileContent.Content)
	return content, err
}

func (c *Client) Compare(projectID int, from, to string) ([]*gitlab.Diff, error) {
	opts := &gitlab.CompareOptions{
		From: &from,
		To:   &to,
	}
	compare, _, err := c.Repositories.Compare(projectID, opts)
	if err != nil {
		return nil, err
	}

	return compare.Diffs, nil
}
