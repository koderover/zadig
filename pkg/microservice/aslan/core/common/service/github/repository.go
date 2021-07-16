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

	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/git"
)

func (c *Client) GetYAMLContents(owner, repo, branch, path string, isDir, split bool) ([]string, error) {
	return c.Client.GetYAMLContents(context.TODO(), owner, repo, branch, path, split)
}

func (c *Client) GetLatestRepositoryCommit(owner, repo, branch, path string) (*git.RepositoryCommit, error) {
	res, err := c.Client.GetLatestRepositoryCommit(context.TODO(), owner, repo, branch, path)
	return git.ToRepositoryCommit(res), err
}
