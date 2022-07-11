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

package aslan

import (
	"crypto/tls"

	"github.com/koderover/zadig/pkg/tool/httpclient"
)

type Client struct {
	*httpclient.Client

	host     string
	token    string
	external bool
}

func New(host string) *Client {
	c := httpclient.New(
		httpclient.SetHostURL(host + "/api"),
	)

	return &Client{
		Client: c,
		host:   host,
	}
}

func NewExternal(host, token string) *Client {
	c := httpclient.New(
		httpclient.SetAuthToken(token),
		httpclient.SetHostURL(host+"/api/aslan"),
	)

	c.SetTLSClientConfig(&tls.Config{InsecureSkipVerify: true})

	return &Client{
		Client:   c,
		host:     host,
		token:    token,
		external: true,
	}
}
