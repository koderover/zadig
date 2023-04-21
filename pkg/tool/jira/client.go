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
	"github.com/imroc/req/v3"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/tool/log"
)

// Client is jira RPC client
type Client struct {
	Host string
	*req.Client
	Issue   *IssueService
	Project *ProjectService
	Board   *BoardService
	Sprint  *SprintService
}

func NewJiraClientWithAuthType(host, username, password, token string, _type config.JiraAuthType) *Client {
	switch _type {
	case config.JiraPersonalAccessToken:
		return NewJiraClientWithBearerToken(host, token)
	case config.JiraBasicAuth:
		return NewJiraClientWithBasicAuth(host, username, password)
	default:
		log.Error("NewJiraClientWithAuthType: invalid type %s", _type)
		return NewJiraClientWithBasicAuth(host, username, password)
	}
}

func NewJiraClientWithBasicAuth(host, username, password string) *Client {
	c := &Client{
		Host: host,
		Client: req.C().SetCommonBasicAuth(username, password).SetCommonHeaders(map[string]string{
			"X-Force-Accept-Language": "true",
			"Accept-Language":         "en-US,en",
		}),
	}

	c.Issue = &IssueService{client: c}
	c.Project = &ProjectService{client: c}
	c.Board = &BoardService{client: c}
	c.Sprint = &SprintService{client: c}

	return c
}

func NewJiraClientWithBearerToken(host, token string) *Client {
	c := &Client{
		Host: host,
		Client: req.C().SetCommonBearerAuthToken(token).SetCommonHeaders(map[string]string{
			"X-Force-Accept-Language": "true",
			"Accept-Language":         "en-US,en",
		}),
	}

	c.Issue = &IssueService{client: c}
	c.Project = &ProjectService{client: c}
	c.Board = &BoardService{client: c}
	c.Sprint = &SprintService{client: c}

	return c
}
