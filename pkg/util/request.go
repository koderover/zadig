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

package util

import (
	"bytes"
	"io/ioutil"
	"net/http"
)

// Deprecated: do not use it any more, use http client in tool/httpclient instead.
func SendRequest(url, method string, header http.Header, body []byte) ([]byte, error) {
	request, err := http.NewRequest(method, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	request.Header = header

	var ret *http.Response
	client := &http.Client{}
	ret, err = client.Do(request)
	if err != nil {
		return nil, err
	}

	defer func() { _ = ret.Body.Close() }()

	var resp []byte
	resp, err = ioutil.ReadAll(ret.Body)
	if err != nil {
		return nil, err
	}

	return resp, nil
}
