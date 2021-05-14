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

type Tag struct {
	Name       string `json:"name"`
	ZipballURL string `json:"zipball_url"`
	TarballURL string `json:"tarball_url"`
	Message    string `json:"message"`
}

// List branches by projectID <- urlEncode(namespace/projectName)
func (c *Client) ListTags(projectID string) ([]*Tag, error) {
	opts := &gitlab.ListTagsOptions{
		ListOptions: gitlab.ListOptions{
			PerPage: 100,
			Page:    1,
		},
	}

	respTags := make([]*Tag, 0)

	for opts.Page > 0 {
		tags, resp, err := c.Tags.ListTags(projectID, opts)
		if err != nil {
			return nil, err
		}

		for _, branch := range tags {
			respB := &Tag{
				Name:    branch.Name,
				Message: branch.Message,
			}
			respTags = append(respTags, respB)
		}
		opts.Page = resp.NextPage
	}
	return respTags, nil
}
