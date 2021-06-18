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
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/httpclient"
)

type CollieClient struct {
	*httpclient.Client

	host  string
	token string
}

func NewCollieClient(host, token string) *CollieClient {
	c := httpclient.New(
		httpclient.SetAuthScheme(setting.RootAPIKey),
		httpclient.SetAuthToken(token),
		httpclient.SetHostURL(host),
	)

	return &CollieClient{
		Client: c,
		host:   host,
		token:  token,
	}
}
