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

type BranchList struct {
	Result struct {
		Total    int        `json:"total"`
		Branches []Branches `json:"branches"`
	} `json:"result"`
	Status string `json:"status"`
}

type Branches struct {
	Commit             Commit `json:"commit"`
	Name               string `json:"name"`
	Protected          bool   `json:"protected"`
	DevelopersCanPush  bool   `json:"developers_can_push"`
	DevelopersCanMerge bool   `json:"developers_can_merge"`
	MasterCanPush      bool   `json:"master_can_push"`
	MasterCanMerge     bool   `json:"master_can_merge"`
	NoOneCanPush       bool   `json:"no_one_can_push"`
	NoOneCanMerge      bool   `json:"no_one_can_merge"`
}

type Branch struct {
	Commit    *Commit `json:"commit"`
	Name      string  `json:"name"`
	Protected bool    `json:"protected"`
	Merged    bool    `json:"merged"`
}

func (c *CodeHubClient) BranchList(repoUUID string) ([]*Branch, error) {
	branchInfos := make([]*Branch, 0)

	branchList := new(BranchList)
	body, err := c.sendRequest("GET", fmt.Sprintf("/v1/repositories/%s/branches", repoUUID), []byte{})
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
