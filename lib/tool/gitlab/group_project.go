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
	"strings"

	"github.com/xanzy/go-gitlab"
)

func (c *Client) ListGroupProjects(namespace, keyword string) ([]*Project, error) {
	respProjs := make([]*Project, 0)

	opts := &gitlab.ListGroupProjectsOptions{
		ListOptions: gitlab.ListOptions{
			PerPage: 100,
			Page:    1,
		},
	}

	orderBy := "name"
	sort := "asc"
	opts.OrderBy = &orderBy
	opts.Sort = &sort

	// gitlab search works only when character length > 2
	if keyword != "" && len(keyword) > 2 {
		opts.Search = &keyword
	}

	projs, _, err := c.Groups.ListGroupProjects(namespace, opts)
	if err != nil {
		return nil, err
	}

	for _, proj := range projs {
		respProj := &Project{
			ID:            proj.ID,
			Name:          proj.Path,
			Description:   proj.Description,
			DefaultBranch: proj.DefaultBranch,
			Namespace:     proj.Namespace.FullPath,
		}

		if len(keyword) > 0 && !strings.Contains(strings.ToLower(proj.Path), strings.ToLower(keyword)) {
			// filter
		} else {
			respProjs = append(respProjs, respProj)
		}
	}

	return filterProjectsByNamespace(respProjs, namespace), nil
}

func filterProjectsByNamespace(pros []*Project, namespace string) []*Project {
	r := make([]*Project, 0)
	for _, project := range pros {
		if project.Namespace == namespace {
			r = append(r, project)
		}
	}
	return r
}
