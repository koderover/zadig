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

package testutils

import (
	"bytes"
	"io/ioutil"
	"net/http"
)

const (
	codeInvalidPath = 404
)

type HttpConfig struct {
	path   string
	status int
	body   []byte
}

func NewHttpConfig(path string, status int, body []byte) *HttpConfig {
	return &HttpConfig{
		path:   path,
		status: status,
		body:   body,
	}
}

func NewHttpClientForTest(config *HttpConfig) *http.Client {
	transport := func(req *http.Request) *http.Response {
		if req.URL.Path == config.path {
			return &http.Response{
				StatusCode: config.status,
				Body:       ioutil.NopCloser(bytes.NewBuffer(config.body)),
			}
		} else {
			return &http.Response{
				StatusCode: codeInvalidPath,
			}
		}
	}

	return &http.Client{
		Transport: roundTripFunc(transport),
	}
}

type roundTripFunc func(req *http.Request) *http.Response

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req), nil
}
