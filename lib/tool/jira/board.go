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
	"net/url"
)

// BoardService ...
type BoardService struct {
	client *Client
}

// ListBoards https://developer.atlassian.com/cloud/jira/software/rest/?_ga=2.61935276.1221874248.1518272104-1457687781.1449453107#api-board-get
//
// TODO: Loop pages to find all boards
//
func (s *BoardService) ListBoards(projectKeyOrID string) (*BoardsList, error) {

	resp := &BoardsList{}

	params := url.Values{}
	params.Add("projectKeyOrId", projectKeyOrID)

	url := s.client.Host + "/rest/agile/1.0/board"

	if len(projectKeyOrID) != 0 {
		url = url + "?" + params.Encode()
	}

	err := s.client.Conn.CallWithJson(context.Background(), resp, "GET", url, "")

	return resp, err
}
