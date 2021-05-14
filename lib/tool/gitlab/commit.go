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
	"time"

	"github.com/xanzy/go-gitlab"
)

// RepoCommit : Repository commit struct
type RepoCommit struct {
	ID         string     `json:"id"`
	Title      string     `json:"title"`
	AuthorName string     `json:"author_name"`
	CreatedAt  *time.Time `json:"created_at"`
	Message    string     `json:"message"`
}

func (c *Client) GetLatestCommit(projectID int, branch, path string) (*RepoCommit, error) {
	commit := &RepoCommit{}
	// List commits with specified ref and path
	opts := &gitlab.ListCommitsOptions{
		Path:    &path,
		RefName: &branch,
		ListOptions: gitlab.ListOptions{
			PerPage: 1,
			Page:    1,
		},
	}
	commits, _, err := c.Commits.ListCommits(projectID, opts)
	if err != nil {
		return commit, err
	}
	if len(commits) > 0 {
		commit.ID = commits[0].ID
		commit.Title = commits[0].Title
		commit.AuthorName = commits[0].AuthorName
		commit.CreatedAt = commits[0].CreatedAt
		commit.Message = commits[0].Message
	}
	return commit, nil
}

// GetLatestCommit By string pro
func (c *Client) GetLatestCommitByProName(projectID, branch, path string) (*RepoCommit, error) {
	commit := &RepoCommit{}
	// List commits with specified ref and path
	opts := &gitlab.ListCommitsOptions{
		RefName: &branch,
		ListOptions: gitlab.ListOptions{
			PerPage: 1,
			Page:    1,
		},
	}
	commits, _, err := c.Commits.ListCommits(projectID, opts)
	if err != nil {
		return commit, err
	}
	if len(commits) > 0 {
		commit.ID = commits[0].ID
		commit.Title = commits[0].Title
		commit.AuthorName = commits[0].AuthorName
		commit.CreatedAt = commits[0].CreatedAt
		commit.Message = commits[0].Message
	}
	return commit, nil
}
