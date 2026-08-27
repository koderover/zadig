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
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

type headerTransport struct {
	base    http.RoundTripper
	headers map[string]string
}

type omitModelTransport struct {
	base http.RoundTripper
}

func newHTTPClient(proxy string, headers map[string]string, timeout time.Duration) (*http.Client, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if proxy != "" {
		proxyURL, err := url.Parse(proxy)
		if err != nil {
			return nil, err
		}
		transport.Proxy = http.ProxyURL(proxyURL)
	}
	var roundTripper http.RoundTripper = transport
	if len(headers) > 0 {
		roundTripper = &headerTransport{base: transport, headers: headers}
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: roundTripper,
	}, nil
}

func (t *headerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	cloned := req.Clone(req.Context())
	for key, value := range t.headers {
		cloned.Header.Set(key, value)
	}
	return t.base.RoundTrip(cloned)
}

func (t *omitModelTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, fmt.Errorf("read request body: %w", err)
	}
	_ = req.Body.Close()

	payload := make(map[string]json.RawMessage)
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("decode request body: %w", err)
	}
	delete(payload, "model")
	body, err = json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode request body: %w", err)
	}

	cloned := req.Clone(req.Context())
	cloned.Body = io.NopCloser(bytes.NewReader(body))
	cloned.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(body)), nil
	}
	cloned.ContentLength = int64(len(body))
	cloned.Header.Del("Content-Length")
	return t.base.RoundTrip(cloned)
}
