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

func (c *CodeHubClient) GetLatestRepositoryCommit(repoOwner, repoName, branchName string) (*Commit, error) {
	commit := new(Commit)
	commitListResp := new(CommitListResp)
	body, err := c.sendRequest("GET", fmt.Sprintf("/v1/repositories/%s/%s/commits?ref_name=%s&page_size=1&page_index=1", repoOwner, repoName, branchName), "")
	if err != nil {
		return commit, err
	}
	defer body.Close()

	if err = json.NewDecoder(body).Decode(commitListResp); err != nil {
		return commit, err
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
