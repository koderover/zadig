package codehub

import (
	"encoding/json"
	"fmt"
)

type CommitList struct {
	Result CommitListResult `json:"result"`
	Status string           `json:"status"`
}

type CommitListResult struct {
	Total   int      `json:"total"`
	Commits []Commit `json:"commits"`
}

func (c *CodeHubClient) CommitList(repoOwner, repoName, branchName string) (*CommitList, error) {
	commitList := new(CommitList)
	body, err := c.sendRequest("GET", fmt.Sprintf("/v1/repositories/%s/%s/commits?ref_name=%s", repoOwner, repoName, branchName), "")
	if err != nil {
		return commitList, err
	}
	defer body.Close()

	if err = json.NewDecoder(body).Decode(commitList); err != nil {
		return commitList, err
	}

	if commitList.Status == "success" {
		return commitList, nil
	}

	return nil, fmt.Errorf("get commit list failed")
}
