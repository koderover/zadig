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
	"net/http"
	"net/http/cookiejar"
	"net/url"

	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/shared/servicetoken"
)

type Client struct {
	APIBase string
	Conn    *http.Client
}

// NewAslanClient is to get aslan client func
func NewAslanClient(host string) *Client {
	jar, _ := cookiejar.New(nil)

	aslanURL, _ := url.Parse(host)

	c := &Client{
		APIBase: host,
		Conn: &http.Client{
			Transport: &internalServiceTokenTransport{base: http.DefaultTransport, host: aslanURL.Host},
			Jar:       jar,
		},
	}

	return c
}

// internalServiceTokenTransport injects the internal service token into every request so the
// receiving service can validate it instead of granting admin on an empty user id.
type internalServiceTokenTransport struct {
	base http.RoundTripper
	host string
}

func (t *internalServiceTokenTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Host != t.host {
		return t.base.RoundTrip(req)
	}
	token, err := servicetoken.CurrentInternalToken()
	if err != nil {
		return nil, err
	}
	clone := req.Clone(req.Context())
	clone.Header = req.Header.Clone()
	clone.Header.Set(setting.InternalServiceTokenHeader, token)
	return t.base.RoundTrip(clone)
}
