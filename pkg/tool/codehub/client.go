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
package codehub

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/koderover/zadig/pkg/util"
)

type CodeHubClient struct {
	AK string `json:"ak"`
	SK string `json:"sk"`
}

func NewCodeHubClient(ak, sk string) *CodeHubClient {
	return &CodeHubClient{
		AK: ak,
		SK: sk,
	}
}

// Just apply the signature and request the CodeHub interface
func (c *CodeHubClient) sendRequest(method, path, payload string) (io.ReadCloser, error) {
	r, _ := http.NewRequest(method, fmt.Sprintf("%s%s", "https://codehub-ext.cn-north-4.myhuaweicloud.com", path), ioutil.NopCloser(bytes.NewBuffer([]byte(payload))))
	r.Header.Add("content-type", "application/json")
	signer := &util.Signer{
		AK: c.AK,
		SK: c.SK,
	}
	signer.Sign(r)
	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}
