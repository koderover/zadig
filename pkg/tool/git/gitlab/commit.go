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
	"github.com/xanzy/go-gitlab"
)

func (c *Client) GetLatestCommit(owner, repo string, branch, path string) (*gitlab.Commit, error) {
	// List commits with specified ref and path
	opts := &gitlab.ListCommitsOptions{
		Path:    &path,
		RefName: &branch,
		ListOptions: gitlab.ListOptions{
			PerPage: 1,
			Page:    1,
		},
	}
	commits, err := wrap(c.Commits.ListCommits(generateProjectName(owner, repo), opts))
	if err != nil {
		return nil, err
	}
	cs, ok := commits.([]*gitlab.Commit)
	if !ok || len(cs) == 0 {
		return nil, nil
	}

	return cs[0], nil
}

func (c *Client) GetSingleCommitOfProject(owner, repo, commitSha string) (*gitlab.Commit, error) {
	commit, err := wrap(c.Commits.GetCommit(generateProjectName(owner, repo), commitSha))
	if err != nil {
		return nil, err
	}
	if ct, ok := commit.(*gitlab.Commit); ok {
		return ct, nil
	}

	return nil, err
}
