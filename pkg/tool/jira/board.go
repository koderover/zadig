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

// BoardService ...
type BoardService struct {
	client *Client
}

// ListBoards https://developer.atlassian.com/cloud/jira/software/rest/?_ga=2.61935276.1221874248.1518272104-1457687781.1449453107#api-board-get
func (s *BoardService) ListBoards(projectKeyOrID, boardType string) ([]*Board, error) {
	startAt := 0
	isLast := false
	boards := []*Board{}

	for !isLast {
		list := &BoardsList{}
		params := url.Values{}
		params.Add("startAt", fmt.Sprintf("%d", startAt))
		params.Add("maxResults", strconv.Itoa(50))

		url := s.client.Host + "/rest/agile/1.0/board"

		if len(projectKeyOrID) != 0 {
			params.Add("projectKeyOrId", projectKeyOrID)
		}
		if len(boardType) != 0 {
			params.Add("type", boardType)
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
		boards = append(boards, list.Values...)
	}

	return boards, nil
}
