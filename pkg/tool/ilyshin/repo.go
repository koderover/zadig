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

package ilyshin

import (
	"fmt"

	"github.com/koderover/zadig/pkg/tool/httpclient"
	"github.com/koderover/zadig/pkg/tool/log"
)

type TreeNode struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
	Path string `json:"path"`
	Mode string `json:"mode"`
}

type File struct {
	FileName     string `json:"file_name"`
	FilePath     string `json:"file_path"`
	Size         int    `json:"size"`
	Encoding     string `json:"encoding"`
	Content      string `json:"content"`
	Ref          string `json:"ref"`
	BlobID       string `json:"blob_id"`
	CommitID     string `json:"commit_id"`
	SHA256       string `json:"content_sha256"`
	LastCommitID string `json:"last_commit_id"`
}

type Diff struct {
	Diff        string `json:"diff"`
	NewPath     string `json:"new_path"`
	OldPath     string `json:"old_path"`
	AMode       string `json:"a_mode"`
	BMode       string `json:"b_mode"`
	NewFile     bool   `json:"new_file"`
	RenamedFile bool   `json:"renamed_file"`
	DeletedFile bool   `json:"deleted_file"`
}

func (c *Client) ListTree(owner, repo string, ref string, path string) ([]*TreeNode, error) {
	url := fmt.Sprintf("/api/v4/projects/%s/repository/tree", generateProjectName(owner, repo))
	qs := map[string]string{
		"ref":      ref,
		"path":     path,
		"per_page": "100",
	}

	var err error
	var treeNodes []*TreeNode
	if _, err = c.Get(url, httpclient.SetQueryParams(qs), httpclient.SetResult(&treeNodes)); err != nil {
		log.Errorf("Failed to list project tree nodes, error: %s", err)
		return treeNodes, err
	}

	return treeNodes, nil
}

func (c *Client) GetRawFile(owner, repo string, sha string, fileName string) ([]byte, error) {
	File, err := c.GetFile(owner, repo, sha, fileName)
	if err != nil {
		return nil, err
	}
	var resp []byte
	url := fmt.Sprintf("/api/v4/projects/%s/repository/blobs/%s/raw", generateProjectName(owner, repo), File.BlobID)
	if _, err = c.Get(url, httpclient.SetResult(&resp)); err != nil {
		log.Errorf("Failed to get project blob raw, error: %s", err)
		return resp, err
	}

	return resp, nil
}

func (c *Client) GetFile(owner, repo string, ref, path string) (*File, error) {
	url := fmt.Sprintf("/api/v4/projects/%s/repository/files/%s", generateProjectName(owner, repo), path)
	qs := map[string]string{
		"ref": ref,
	}

	var err error
	var file *File
	if _, err = c.Get(url, httpclient.SetQueryParams(qs), httpclient.SetResult(&file)); err != nil {
		log.Errorf("Failed to get project file content, error: %s", err)
		return file, err
	}

	return file, nil
}

func (c *Client) Compare(projectID int, from, to string) ([]*Diff, error) {
	url := fmt.Sprintf("/api/v4/projects/%d/repository/compare", projectID)
	qs := map[string]string{
		"from": from,
		"to":   to,
	}

	var err error
	var diffs []*Diff
	if _, err = c.Get(url, httpclient.SetQueryParams(qs), httpclient.SetResult(&diffs)); err != nil {
		log.Errorf("Failed to compare content, error: %s", err)
		return diffs, err
	}

	return diffs, nil
}
