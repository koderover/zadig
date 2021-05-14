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
	"encoding/base64"
	"net/http"

	"github.com/koderover/zadig/lib/tool/httpclient"
)

// Client is jira RPC client
type Client struct {
	Host    string
	Conn    *httpclient.Client
	Issue   *IssueService
	Project *ProjectService
	Board   *BoardService
	Sprint  *SprintService
}

// BasicTransport a http round tripper
type BasicTransport struct {
	Username  string
	Password  string
	transport http.RoundTripper
}

// RoundTrip : The Request's URL and Header fields must be initialized.
func (t *BasicTransport) RoundTrip(req *http.Request) (resp *http.Response, err error) {
	auth := "Basic " + base64.URLEncoding.EncodeToString([]byte(t.Username+":"+t.Password))
	req.Header.Set("Authorization", auth)
	return t.transport.RoundTrip(req)
}

// NewBasicTransport return a BasicTransport
func NewBasicTransport(username, password string, transport http.RoundTripper) *BasicTransport {
	if transport == nil {
		transport = http.DefaultTransport
	}

	return &BasicTransport{
		Username:  username,
		Password:  password,
		transport: transport,
	}
}

// NewJiraClient is to get jira client func
func NewJiraClient(username, password, host string) *Client {
	tr := NewBasicTransport(username, password, nil)
	httpClient := &http.Client{Transport: tr}

	c := &Client{
		Host: host,
		Conn: &httpclient.Client{Client: httpClient},
	}

	c.Issue = &IssueService{client: c}
	c.Project = &ProjectService{client: c}
	c.Board = &BoardService{client: c}
	c.Sprint = &SprintService{client: c}

	return c
}
