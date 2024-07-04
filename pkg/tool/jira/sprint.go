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
	"fmt"
	"net/url"
	"strconv"

	"github.com/pkg/errors"
)

// SprintService ...
type SprintService struct {
	client *Client
}

// ListSprints https://developer.atlassian.com/cloud/jira/software/rest/?_ga=2.61935276.1221874248.1518272104-1457687781.1449453107#api-board-boardId-sprint-get
// states valid values: future, active, closed. You can define multiple states separated by commas, e.g. state=active,closed
func (s *SprintService) ListSprints(boardID int, state string) ([]*Sprint, error) {
	startAt := 0
	isLast := false
	sprints := []*Sprint{}

	for !isLast {
		list := &SprintsList{}
		params := url.Values{}
		params.Add("startAt", fmt.Sprintf("%d", startAt))
		params.Add("maxResults", strconv.Itoa(50))

		url := fmt.Sprintf("%s/rest/agile/1.0/board/%d/sprint", s.client.Host, boardID)

		if len(state) != 0 {
			params.Add("state", state)
		}
		url = url + "?" + params.Encode()

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

		isLast = list.IsLast
		startAt += len(list.Values)
		sprints = append(sprints, list.Values...)
	}

	return sprints, nil
}

func (s *SprintService) GetSrpint(sprintID int) (*Sprint, error) {

	list := &Sprint{}

	url := fmt.Sprintf("%s/rest/agile/1.0/sprint/%d", s.client.Host, sprintID)

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

	return list, err
}

// // ListSprintIssues https://developer.atlassian.com/cloud/jira/software/rest/?_ga=2.61935276.1221874248.1518272104-1457687781.1449453107#api-sprint-sprintId-issue-get
func (s *SprintService) ListSprintIssues(sprintID int, fields string) ([]*Issue, error) {
	startAt := 0
	isLast := false
	issues := []*Issue{}

	for !isLast {
		list := &IssueResult{}
		params := url.Values{}
		params.Add("fields", fields)
		params.Add("startAt", strconv.Itoa(startAt))
		params.Add("maxResults", strconv.Itoa(50))

		url := fmt.Sprintf("%s/rest/agile/1.0/sprint/%s/issue", s.client.Host, strconv.Itoa(sprintID))
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

		startAt += len(list.Issues)
		if startAt >= list.Total {
			isLast = true
		}
		issues = append(issues, list.Issues...)
	}

	return issues, nil
}
