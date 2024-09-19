/*
Copyright 2024 The KodeRover Authors.

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
	"github.com/pkg/errors"
)

// ProjectService ...
type PlatformService struct {
	client *Client
}

// Get all status https://developer.atlassian.com/server/jira/platform/rest/v10000/api-group-status/#api-group-status
func (s *PlatformService) ListAllStatues() ([]*Status, error) {
	list := make([]*Status, 0)
	url := s.client.Host + "/rest/api/2/status"

	resp, err := s.client.R().Get(url)
	if err != nil {
		return nil, err
	}
	if resp.GetStatusCode()/100 != 2 {
		return nil, errors.Errorf("get unexpected status code %d, body: %s", resp.GetStatusCode(), resp.String())
	}
	if err = resp.UnmarshalJson(&list); err != nil {
		return nil, errors.Wrap(err, "unmarshal")
	}

	return list, nil
}
