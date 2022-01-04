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
	"fmt"
	"net/http"
	"net/url"

	"github.com/koderover/zadig/pkg/tool/httpclient"
)

func (c *Client) DeleteProject(header http.Header, qs url.Values, productName string) ([]byte, error) {
	url := fmt.Sprintf("/project/products/%s", productName)

	res, err := c.Delete(url, httpclient.SetHeadersFromHTTPHeader(header), httpclient.SetQueryParamsFromValues(qs))
	if err != nil {
		return nil, err
	}

	return res.Body(), nil
}

func (c *Client) ListProjects(header http.Header, qs url.Values) ([]byte, error) {
	url := "/project/projects"

	res, err := c.Get(url, httpclient.SetHeadersFromHTTPHeader(header), httpclient.SetQueryParamsFromValues(qs))
	if err != nil {
		return nil, err
	}

	return res.Body(), nil
}

func (c *Client) CreateProject(header http.Header, qs url.Values, body []byte) ([]byte, error) {
	url := "/project/products"
	res, err := c.Post(url, httpclient.SetHeadersFromHTTPHeader(header), httpclient.SetQueryParamsFromValues(qs), httpclient.SetBody(body))
	if err != nil {
		return nil, err
	}

	return res.Body(), nil
}

func (c *Client) UpdateProject(header http.Header, qs url.Values, body []byte) ([]byte, error) {
	url := "/project/products"
	res, err := c.Put(url, httpclient.SetHeadersFromHTTPHeader(header), httpclient.SetQueryParamsFromValues(qs), httpclient.SetBody(body))
	if err != nil {
		return nil, err
	}

	return res.Body(), nil
}
