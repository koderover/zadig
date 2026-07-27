/*
Copyright 2026 The KodeRover Authors.

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

package llm

import (
	"net/http"
	"net/url"
	"time"
)

type headerTransport struct {
	base        http.RoundTripper
	headers     map[string]string
	disableAuth bool
}

func newHTTPClient(proxy string, headers map[string]string, disableAuth bool) (*http.Client, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if proxy != "" {
		proxyURL, err := url.Parse(proxy)
		if err != nil {
			return nil, err
		}
		transport.Proxy = http.ProxyURL(proxyURL)
	}
	return &http.Client{
		Timeout:   5 * time.Minute,
		Transport: &headerTransport{base: transport, headers: headers, disableAuth: disableAuth},
	}, nil
}

func (t *headerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	cloned := req.Clone(req.Context())
	if t.disableAuth {
		cloned.Header.Del("Authorization")
		cloned.Header.Del("x-api-key")
		cloned.Header.Del("api-key")
	}
	for key, value := range t.headers {
		cloned.Header.Set(key, value)
	}
	return t.base.RoundTrip(cloned)
}
