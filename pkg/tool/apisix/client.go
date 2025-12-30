/*
Copyright 2025 The KodeRover Authors.

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

package apisix

import (
	"fmt"
	"strings"

	"github.com/koderover/zadig/v2/pkg/tool/httpclient"
)

type Client struct {
	Host   string
	ApiKey string
}

func NewClient(host, apiKey string) *Client {
	return &Client{
		Host:   strings.TrimSuffix(host, "/"),
		ApiKey: apiKey,
	}
}

// Get do a get request with the given queries to the url and set the response into the response param
func (c *Client) Get(url string, queries map[string]string, response interface{}) error {
	opts := []httpclient.RequestFunc{
		httpclient.SetHeader("X-API-KEY", c.ApiKey),
		httpclient.SetHeader("Content-Type", "application/json"),
		httpclient.SetQueryParams(queries),
	}
	if response != nil {
		opts = append(opts, httpclient.SetResult(response))
	}

	_, err := httpclient.Get(url, opts...)
	if err != nil {
		return fmt.Errorf("failed to do apisix request, error: %s", err)
	}

	return nil
}

// Post do a post request with the given request to the url and set the response into the response param
func (c *Client) Post(url string, requestBody interface{}, response interface{}) error {
	opts := []httpclient.RequestFunc{
		httpclient.SetHeader("X-API-KEY", c.ApiKey),
		httpclient.SetHeader("Content-Type", "application/json"),
		httpclient.SetBody(requestBody),
	}
	if response != nil {
		opts = append(opts, httpclient.SetResult(response))
	}

	_, err := httpclient.Post(url, opts...)
	if err != nil {
		return fmt.Errorf("failed to do apisix request, error: %s", err)
	}

	return nil
}

// Put do a put request with the given request to the url and set the response into the response param
func (c *Client) Put(url string, requestBody interface{}, response interface{}) error {
	opts := []httpclient.RequestFunc{
		httpclient.SetHeader("X-API-KEY", c.ApiKey),
		httpclient.SetHeader("Content-Type", "application/json"),
		httpclient.SetBody(requestBody),
	}
	if response != nil {
		opts = append(opts, httpclient.SetResult(response))
	}

	_, err := httpclient.Put(url, opts...)
	if err != nil {
		return fmt.Errorf("failed to do apisix request, error: %s", err)
	}

	return nil
}

// Delete do a delete request to the url and set the response into the response param
// Pass nil for response if you don't need the response body
func (c *Client) Delete(url string, response interface{}) error {
	opts := []httpclient.RequestFunc{
		httpclient.SetHeader("X-API-KEY", c.ApiKey),
		httpclient.SetHeader("Content-Type", "application/json"),
	}
	if response != nil {
		opts = append(opts, httpclient.SetResult(response))
	}

	_, err := httpclient.Delete(url, opts...)
	if err != nil {
		return fmt.Errorf("failed to do apisix request, error: %s", err)
	}

	return nil
}

