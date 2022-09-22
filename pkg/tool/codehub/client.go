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
	"net/http"
	"net/url"

	"github.com/koderover/zadig/pkg/tool/log"
)

type CodeHubClient struct {
	AK          string `json:"ak"`
	SK          string `json:"sk"`
	Region      string `json:"region"`
	ProxyAddr   string `json:"proxy_addr"`
	EnableProxy bool   `json:"enable_proxy"`
}

func NewCodeHubClient(ak, sk, region, proxyAddr string, enableProxy bool) *CodeHubClient {
	return &CodeHubClient{
		AK:          ak,
		SK:          sk,
		Region:      region,
		ProxyAddr:   proxyAddr,
		EnableProxy: enableProxy,
	}
}

// Just apply the signature and request the CodeHub interface
func (c *CodeHubClient) sendRequest(method, path string, payload []byte) (io.ReadCloser, error) {
	var client *http.Client
	r, err := http.NewRequest(method, fmt.Sprintf("%s.%s.%s%s", "https://codehub-ext", c.Region, "myhuaweicloud.com", path), io.NopCloser(bytes.NewBuffer(payload)))
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
	if c.EnableProxy {
		proxyURL, err := url.Parse(c.ProxyAddr)
		if err != nil {
			return nil, err
		}
		transport := &http.Transport{Proxy: http.ProxyURL(proxyURL)}
		client = &http.Client{Transport: transport}
	} else {
		client = http.DefaultClient
	}
	resp, err := client.Do(r)
	if err != nil || resp == nil {
		log.Errorf("http.DefaultClient.Do error:%s", err)
		return nil, err
	}
	return resp.Body, nil
}
