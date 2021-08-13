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

	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/git"
)

func (c *Client) GetTree(owner, repo, path, branch string) ([]*git.TreeNode, error) {
	var treeNodes []*git.TreeNode

	_, dirContent, err := c.GetContents(context.TODO(), owner, repo, path, &github.RepositoryContentGetOptions{Ref: branch})
	if err != nil {
		return nil, err
	}
	for _, dir := range dirContent {
		treeNodes = append(treeNodes, git.ToTreeNode(dir))
	}
	return treeNodes, nil
}
