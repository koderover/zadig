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

package poetry

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/koderover/zadig/lib/setting"
	"github.com/koderover/zadig/lib/util"
)

const (
	GitLabProvider = "gitlab"
	GitHubProvider = "github"
	GerritProvider = "gerrit"
)

// PoetryClient ...
type PoetryClient struct {
	PoetryAPIServer string
	ApiRootKey      string
}

// NewPoetryClient constructor
func NewPoetryServer(poetryApiServer, ApiRootKey string) *PoetryClient {
	return &PoetryClient{
		PoetryAPIServer: poetryApiServer,
		ApiRootKey:      ApiRootKey,
	}
}

func (p *PoetryClient) GetRootTokenHeader() http.Header {
	header := http.Header{}
	header.Set(setting.Auth, fmt.Sprintf("%s%s", setting.AuthPrefix, os.Getenv("POETRY_API_ROOT_KEY")))
	return header
}

func (p *PoetryClient) Do(url, method string, reader io.Reader, header http.Header) ([]byte, error) {
	if !strings.HasPrefix(url, p.PoetryAPIServer) {
		url = p.PoetryAPIServer + url
	}

	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		return nil, err
	}

	req.Header = header

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer func() { _ = resp.Body.Close() }()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode/100 == 2 {
		return body, nil
	}

	return nil, fmt.Errorf("response status error: %s %s %v %d", url, string(body), resp.Header, resp.StatusCode)
}

func (p *PoetryClient) SendRequest(url, method string, data interface{}, header http.Header) (string, error) {
	if body, err := p.Do(url, method, util.GetRequestBody(data), header); err != nil {
		return "", err
	} else {
		return string(body), nil
	}
}

func (p *PoetryClient) Deserialize(data []byte, v interface{}) error {
	err := json.Unmarshal(data, v)
	if err != nil {
		return err
	}
	return nil
}
