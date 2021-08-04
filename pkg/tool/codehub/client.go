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

	"github.com/koderover/zadig/pkg/tool/log"
)

type CodeHubClient struct {
	AK     string `json:"ak"`
	SK     string `json:"sk"`
	Region string `json:"region"`
}

func NewCodeHubClient(ak, sk, region string) *CodeHubClient {
	return &CodeHubClient{
		AK:     ak,
		SK:     sk,
		Region: region,
	}
}

// Just apply the signature and request the CodeHub interface
func (c *CodeHubClient) sendRequest(method, path string, payload []byte) (io.ReadCloser, error) {
	r, err := http.NewRequest(method, fmt.Sprintf("%s.%s.%s%s", "https://codehub-ext", c.Region, "myhuaweicloud.com", path), ioutil.NopCloser(bytes.NewBuffer(payload)))
	if r == nil || err != nil {
		log.Errorf("http.NewRequest error:%s", err)
		return nil, err
	}
	r.Header.Add("content-type", "application/json")
	signer := &Signer{
		AK: c.AK,
		SK: c.SK,
	}
	err = signer.Sign(r)
	if err != nil {
		log.Errorf("signer.Sign error:%s", err)
		return nil, err
	}
	resp, err := http.DefaultClient.Do(r)
	if err != nil || resp == nil {
		log.Errorf("http.DefaultClient.Do error:%s", err)
		return nil, err
	}
	return resp.Body, nil
}
