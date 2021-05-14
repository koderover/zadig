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

package nsqcli

import (
	"net/http"

	"github.com/koderover/zadig/lib/tool/httpclient"
)

// Client is nsq RPC client
type Client struct {
	lookupdAddr []string
	nsqdAddr    string
	Conn        *httpclient.Client
}

// NewNsqClient is to get nsq client func
func NewNsqClient(lookupdAddr []string, nsqdAddr string) *Client {
	httpClient := &http.Client{}

	c := &Client{
		lookupdAddr: lookupdAddr,
		nsqdAddr:    nsqdAddr,
		Conn:        &httpclient.Client{Client: httpClient},
	}

	return c
}
