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

import "github.com/xanzy/go-gitlab"

type MergeRequest struct {
	MRID           int    `json:"id"`
	TargetBranch   string `json:"targetBranch"`
	SourceBranch   string `json:"sourceBranch"`
	ProjectID      int    `json:"projectId"`
	Title          string `json:"title"`
	State          string `json:"state"`
	CreatedAt      int64  `json:"createdAt"`
	UpdatedAt      int64  `json:"updatedAt"`
	AuthorUsername string `json:"authorUsername"`
	Number         int    `json:"number"`
	User           string `json:"user"`
	Base           string `json:"base,omitempty"`
}

func (c *Client) ListOpened(projectID, targetBranch string) ([]*MergeRequest, error) {
	state := "opened"

	opts := &gitlab.ListProjectMergeRequestsOptions{
		State: &state,
		ListOptions: gitlab.ListOptions{
			PerPage: 100,
			Page:    1,
		},
	}
	if targetBranch != "" {
		opts.TargetBranch = &targetBranch
	}

	var respMRs []*MergeRequest

	for opts.ListOptions.Page > 0 {

		mrs, resp, err := c.MergeRequests.ListProjectMergeRequests(projectID, opts)
		if err != nil {
			return nil, err
		}

		for _, mr := range mrs {
			req := &MergeRequest{
				MRID:           mr.IID,
				TargetBranch:   mr.TargetBranch,
				SourceBranch:   mr.SourceBranch,
				ProjectID:      mr.ProjectID,
				Title:          mr.Title,
				State:          mr.State,
				CreatedAt:      mr.CreatedAt.Unix(),
				UpdatedAt:      mr.UpdatedAt.Unix(),
				AuthorUsername: mr.Author.Username,
			}
			respMRs = append(respMRs, req)
		}

		opts.ListOptions.Page = resp.NextPage
	}
	return respMRs, nil
}

func (c *Client) ListChangedFiles(address, token string, event *gitlab.MergeEvent) ([]string, error) {
	files := make([]string, 0)
	mergeRequest, _, err := c.MergeRequests.GetMergeRequestChanges(event.ObjectAttributes.TargetProjectID, event.ObjectAttributes.IID, nil)
	if err != nil || mergeRequest == nil {
		return nil, err
	}
	for _, change := range mergeRequest.Changes {
		files = append(files, change.NewPath)
		files = append(files, change.OldPath)
	}

	return files, nil
}
