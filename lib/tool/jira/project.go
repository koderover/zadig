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

package jira

import (
	"context"
)

// Project ...
type Project struct {
	ID   string `json:"id,omitempty"`
	Key  string `json:"key,omitempty"`
	Name string `json:"name,omitempty"`
}

// ProjectService ...
type ProjectService struct {
	client *Client
}

// ListProjects https://developer.atlassian.com/cloud/jira/platform/rest/#api-api-2-project-get
func (s *ProjectService) ListProjects() ([]*Project, error) {

	resp := make([]*Project, 0)

	url := s.client.Host + "/rest/api/2/project"

	err := s.client.Conn.CallWithJson(context.Background(), &resp, "GET", url, "")

	return resp, err
}

// ListComponents ...
func (s *ProjectService) ListComponents(projectKey string) ([]*Component, error) {

	resp := make([]*Component, 0)

	url := s.client.Host + "/rest/api/2/project/" + projectKey + "/components"

	err := s.client.Conn.CallWithJson(context.Background(), &resp, "GET", url, "")

	return resp, err
}
