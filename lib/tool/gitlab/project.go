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
	"fmt"
	"strings"

	"github.com/xanzy/go-gitlab"
)

type Project struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	DefaultBranch string `json:"defaultBranch"`
	Namespace     string `json:"namespace"`
}

const DefaultOrgID = 1

func (c *Client) ListUserProjects(owner, keyword string) ([]*Project, error) {
	encodeOwner := strings.Replace(owner, ".", "%2e", -1)
	opts := &gitlab.ListProjectsOptions{
		ListOptions: gitlab.ListOptions{
			PerPage: 100,
			Page:    1,
		},
	}

	if keyword == "" {
		opts.Search = &keyword
	}

	respProjs := make([]*Project, 0)

	projs, _, err := c.Projects.ListUserProjects(encodeOwner, opts)
	if err != nil {
		return nil, err
	}

	for _, proj := range projs {
		respProj := &Project{
			ID:            proj.ID,
			Name:          proj.Path,
			Namespace:     proj.Namespace.FullPath,
			Description:   proj.Description,
			DefaultBranch: proj.DefaultBranch,
		}
		if len(keyword) > 0 && !strings.Contains(strings.ToLower(proj.Path), strings.ToLower(keyword)) {
			// filter
		} else {
			respProjs = append(respProjs, respProj)
		}
	}

	return filterProjectsByNamespace(respProjs, owner), nil
}

func (c *Client) GetProject(owner, repo string) (*Project, error) {
	proj, _, err := c.Projects.GetProject(fmt.Sprintf("%s/%s", owner, repo), nil)
	if err != nil {
		return nil, err
	}
	resp := &Project{
		ID:            proj.ID,
		Name:          proj.Name,
		Description:   proj.Description,
		DefaultBranch: proj.DefaultBranch,
	}
	return resp, nil
}
