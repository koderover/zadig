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

type Branch struct {
	Name      string `json:"name"`
	Protected bool   `json:"protected"`
	Merged    bool   `json:"merged"`
}

// List branches by projectID <- urlEncode(namespace/projectName)
func (c *Client) ListBranches(projectID string) ([]*Branch, error) {
	opts := &gitlab.ListBranchesOptions{
		ListOptions: gitlab.ListOptions{
			PerPage: 100,
			Page:    1,
		},
	}

	branches, _, err := c.Branches.ListBranches(projectID, opts)
	if err != nil {
		return nil, err
	}

	var respBs []*Branch
	for _, branch := range branches {
		respBs = append(respBs, &Branch{
			Name:      branch.Name,
			Protected: branch.Protected,
			Merged:    branch.Merged,
		})
	}
	return respBs, nil
}
