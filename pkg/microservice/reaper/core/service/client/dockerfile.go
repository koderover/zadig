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

package client

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/pkg/errors"

	"github.com/koderover/zadig/pkg/tool/log"
)

type Dockerfile struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Content string `json:"content"`
}

func (c *Client) GetDockerfile(id string) (*Dockerfile, error) {
	resp := new(Dockerfile)
	var err error

	url := fmt.Sprintf("%s/api/template/dockerfile/%s", c.APIBase, id)
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Errorf("new http request error: %v", err)
		return nil, err
	}
	// No auth required
	var ret *http.Response
	if ret, err = c.Conn.Do(request); err == nil {
		defer func() { _ = ret.Body.Close() }()
		var body []byte
		body, err = ioutil.ReadAll(ret.Body)
		if err == nil {
			if err = json.Unmarshal(body, &resp); err == nil {
				return resp, nil
			}
		}
	}

	return resp, errors.WithMessage(err, "failed to get dockerfile template")
}
