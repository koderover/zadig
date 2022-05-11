package gitee

import (
	"context"

	"github.com/27149chen/afero"
	"github.com/google/go-github/github"

	gitservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/git"
)

func (c *Client) GetTreeContents(owner, repo, path, branch string) (afero.Fs, error) {
	panic("implement me")
}

func (c *Client) GetFileContent(owner, repo, path, branch string) ([]byte, error) {
	fileContent, _, err := c.Client.GetContents(context.TODO(), owner, repo, path, &github.RepositoryContentGetOptions{Ref: branch})
	if err != nil {
		return nil, err
	}

	if fileContent == nil {
		return nil, nil
	}

	return []byte(res), err
}

func (c *Client) GetTree(owner, repo, path, branch string) ([]*gitservice.TreeNode, error) {
	var treeNodes []*gitservice.TreeNode

	tns, err := c.Client.GetTrees(context.TODO(), owner, repo, branch, 0)
	if err != nil {
		return nil, err
	}
	for _, t := range tns.Tree {
		treeNodes = append(treeNodes, gitservice.ToTreeNode(&t))
	}
	return treeNodes, nil
}

func (c *Client) GetYAMLContents(owner, repo, path, branch string, isDir, split bool) ([]string, error) {
	panic("implement me")
}
