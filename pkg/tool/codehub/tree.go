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

package codehub

import (
	"encoding/json"
	"fmt"
)

type FileTree struct {
	Result FileTreeResult `json:"result"`
	Status string         `json:"status"`
}

type FileTreeResult struct {
	Trees []Trees `json:"trees"`
	Total int     `json:"total"`
}

type Trees struct {
	BlobID   string `json:"blob_id"`
	Commit   Commit `json:"commit"`
	FileName string `json:"file_name"`
	FilePath string `json:"file_path"`
	Type     string `json:"type"`
}

type TreeNode struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
	Path string `json:"path"`
}

func (c *CodeHubClient) FileTree(repoUUID, branchName, path string) ([]*TreeNode, error) {
	treeNodes := make([]*TreeNode, 0)

	fileTrees := new(FileTree)
	body, err := c.sendRequest("GET", fmt.Sprintf("/v1/repositories/%s/branch/%s/sub-files?path=%s", repoUUID, branchName, path), []byte{})
	if err != nil {
		return treeNodes, err
	}
	defer body.Close()

	if err = json.NewDecoder(body).Decode(fileTrees); err != nil {
		return treeNodes, err
	}

	if fileTrees.Result.Total == 0 {
		return treeNodes, nil
	}

	for _, treeInfo := range fileTrees.Result.Trees {
		treeNodes = append(treeNodes, &TreeNode{
			ID:   treeInfo.BlobID,
			Name: treeInfo.FileName,
			Path: treeInfo.FilePath,
			Type: treeInfo.Type,
		})
	}
	return treeNodes, nil
}
