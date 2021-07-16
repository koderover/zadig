package codehub

import (
	"encoding/json"
	"fmt"
)

type BranchList struct {
	Result struct {
		Total    int `json:"total"`
		Branches []struct {
			Commit             Commit `json:"commit"`
			Name               string `json:"name"`
			Protected          bool   `json:"protected"`
			DevelopersCanPush  bool   `json:"developers_can_push"`
			DevelopersCanMerge bool   `json:"developers_can_merge"`
			MasterCanPush      bool   `json:"master_can_push"`
			MasterCanMerge     bool   `json:"master_can_merge"`
			NoOneCanPush       bool   `json:"no_one_can_push"`
			NoOneCanMerge      bool   `json:"no_one_can_merge"`
		} `json:"branches"`
	} `json:"result"`
	Status string `json:"status"`
}

type Commit struct {
	ID            string `json:"id"`
	Message       string `json:"message"`
	AuthorName    string `json:"author_name"`
	CommittedDate string `json:"committed_date"`
	CommitterName string `json:"committer_name"`
}

type Branch struct {
	*Commit
	Name      string `json:"name"`
	Protected bool   `json:"protected"`
	Merged    bool   `json:"merged"`
}

func (c *CodeHubClient) BranchList(repoUUID string) ([]*Branch, error) {
	branchInfos := make([]*Branch, 0)

	branchList := new(BranchList)
	body, err := c.sendRequest("GET", fmt.Sprintf("/v1/repositories/%s/branches", repoUUID), "")
	if err != nil {
		return branchInfos, err
	}
	defer body.Close()

	if err = json.NewDecoder(body).Decode(branchList); err != nil {
		return branchInfos, err
	}

	if branchList.Result.Total == 0 {
		return branchInfos, nil
	}

	for _, branch := range branchList.Result.Branches {
		branchInfos = append(branchInfos, &Branch{
			Name:      branch.Name,
			Protected: branch.Protected,
			Merged:    branch.MasterCanMerge,
			Commit: &Commit{
				ID:            branch.Commit.ID,
				Message:       branch.Commit.Message,
				AuthorName:    branch.Commit.AuthorName,
				CommitterName: branch.Commit.CommitterName,
				CommittedDate: branch.Commit.CommittedDate,
			},
		})
	}
	return branchInfos, nil
}
