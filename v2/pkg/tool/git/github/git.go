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
)

func (c *Client) GetTree(ctx context.Context, owner string, repo string, sha string, recursive bool) (*github.Tree, error) {
	tree, err := wrap(c.Git.GetTree(ctx, owner, repo, sha, recursive))
	if t, ok := tree.(*github.Tree); ok {
		return t, err
	}

	return nil, err
}
