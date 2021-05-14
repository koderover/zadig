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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"
	"strconv"
)

// SprintService ...
type SprintService struct {
	client *Client
}

// ListSprints https://developer.atlassian.com/cloud/jira/software/rest/?_ga=2.61935276.1221874248.1518272104-1457687781.1449453107#api-board-boardId-sprint-get
// states valid values: future, active, closed. You can define multiple states separated by commas, e.g. state=active,closed
//
// TODO: Loop pages to find all sprints
//
func (s *SprintService) ListSprints(boardID int, state string) (*SprintsList, error) {

	resp := &SprintsList{}

	params := url.Values{}
	params.Add("state", state)

	url := fmt.Sprintf("%s/rest/agile/1.0/board/%d/sprint", s.client.Host, boardID)

	if len(state) != 0 {
		url = url + "?" + params.Encode()
	}

	err := s.client.Conn.CallWithJson(context.Background(), resp, "GET", url, "")

	return resp, err
}

// ListSprintIssues https://developer.atlassian.com/cloud/jira/software/rest/?_ga=2.61935276.1221874248.1518272104-1457687781.1449453107#api-sprint-sprintId-issue-get
//
// TODO: check and modify issue struct
//
func (s *SprintService) ListSprintIssues(sprintID int, startAt int, fields string) (*IssueResult, error) {
	ret := &IssueResult{}
	params := url.Values{}
	params.Add("fields", fields)
	params.Add("startAt", strconv.Itoa(startAt))
	params.Add("maxResults", "100") // 设置默认一次返回值为100个issue

	url := fmt.Sprintf("%s/rest/agile/1.0/sprint/%s/issue?%s", s.client.Host, strconv.Itoa(sprintID), params.Encode())

	resp, err := s.client.Conn.DoRequestWithJson(context.Background(), "GET", url, "")
	if err != nil {
		return ret, err
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return ret, err
	}
	if err := json.Unmarshal(body, ret); err != nil {
		return ret, err
	}
	return ret, err
}
