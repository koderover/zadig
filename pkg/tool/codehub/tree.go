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
	body, err := c.sendRequest("GET", fmt.Sprintf("/v1/repositories/%s/branch/%s/sub-files?path=%s", repoUUID, branchName, path), "")
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
