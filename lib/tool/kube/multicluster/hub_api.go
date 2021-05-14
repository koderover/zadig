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

package multicluster

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/pkg/errors"
)

type HubClient struct {
	addr  *url.URL
	http  *http.Client
	proxy *httputil.ReverseProxy
}

func NewHubClient(hubServerAddr string) (*HubClient, error) {
	addr, err := url.Parse(hubServerAddr)
	if err != nil {
		return nil, errors.WithMessage(err, "invalid hub server addr")
	}

	return &HubClient{
		addr: addr,
		http: &http.Client{
			Transport: http.DefaultTransport,
		},
		proxy: httputil.NewSingleHostReverseProxy(addr),
	}, nil
}

func (c *HubClient) CreatePost(uri string) (*http.Request, error) {
	return http.NewRequest(
		"POST",
		fmt.Sprintf("%s%s", c.addr.String(), uri),
		nil,
	)
}

func (c *HubClient) Do(uri string) error {
	req, err := http.NewRequest(
		"POST",
		fmt.Sprintf("%s%s", c.addr.String(), uri),
		nil,
	)

	if err != nil {
		return err
	}

	resp, err := c.http.Do(req)

	if err != nil {
		return err
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != 200 {
		return errors.Errorf("got %d response", resp.StatusCode)
	}

	return nil
}

func (c *HubClient) Disconnect(id string) error {
	return c.Do("/disconnect/" + id)
}

func (c *HubClient) Restore(id string) error {
	return c.Do("/restore/" + id)
}

func (c *HubClient) AgentProxy(w http.ResponseWriter, r *http.Request) {
	c.proxy.ServeHTTP(w, r)
}

func (c *HubClient) HasSession(id string) error {
	return c.Do("/hasSession/" + id)
}
