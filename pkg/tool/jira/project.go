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

import "github.com/pkg/errors"

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
func (s *ProjectService) ListProjects() ([]string, error) {
	list := make([]*Project, 0)
	url := s.client.Host + "/rest/api/2/project"

	resp, err := s.client.R().Get(url)
	if err != nil {
		return nil, err
	}
	if resp.GetStatusCode()/100 != 2 {
		return nil, errors.Errorf("unexpected status code %d", resp.GetStatusCode())
	}
	if err = resp.UnmarshalJson(&list); err != nil {
		return nil, errors.Wrap(err, "unmarshal")
	}

	var name []string
	for _, project := range list {
		name = append(name, project.Key)
	}
	return name, nil
}

//// ListComponents ...
//func (s *ProjectService) ListComponents(projectKey string) ([]*Component, error) {
//
//	resp := make([]*Component, 0)
//
//	url := s.client.Host + "/rest/api/2/project/" + projectKey + "/components"
//
//	err := s.client.Conn.CallWithJson(context.Background(), &resp, "GET", url, "")
//
//	return resp, err
//}
