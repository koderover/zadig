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

type CommitListResp struct {
	Result CommitListResult `json:"result"`
	Status string           `json:"status"`
}

type CommitListResult struct {
	Total   int      `json:"total"`
	Commits []Commit `json:"commits"`
}

type Commit struct {
	ID            string `json:"id"`
	Message       string `json:"message"`
	AuthorName    string `json:"author_name"`
	CommittedDate string `json:"committed_date"`
	CommitterName string `json:"committer_name"`
}

func (c *CodeHubClient) GetLatestRepositoryCommit(repoOwner, repoName, branchName string) (*Commit, error) {
	commitListResp := new(CommitListResp)
	body, err := c.sendRequest("GET", fmt.Sprintf("/v1/repositories/%s/%s/commits?ref_name=%s&page_size=1&page_index=1", repoOwner, repoName, branchName), []byte{})
	if err != nil {
		return nil, err
	}
	defer body.Close()

	if err = json.NewDecoder(body).Decode(commitListResp); err != nil {
		return nil, err
	}

	if commitListResp.Status == "success" && len(commitListResp.Result.Commits) > 0 {
		return &Commit{
			ID:            commitListResp.Result.Commits[0].ID,
			Message:       commitListResp.Result.Commits[0].Message,
			AuthorName:    commitListResp.Result.Commits[0].AuthorName,
			CommittedDate: commitListResp.Result.Commits[0].CommittedDate,
			CommitterName: commitListResp.Result.Commits[0].CommitterName,
		}, nil
	}

	return nil, fmt.Errorf("get commit list failed")
}
