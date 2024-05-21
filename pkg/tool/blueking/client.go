/*
Copyright 2024 The KodeRover Authors.

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

package blueking

import (
	"fmt"
	"strings"

	"github.com/koderover/zadig/v2/pkg/tool/httpclient"
)

type Client struct {
	Host      string
	AppCode   string
	AppSecret string
	UserName  string
}

func NewClient(host, appCode, appSecret, userName string) *Client {
	return &Client{
		Host:      strings.TrimSuffix(host, "/"),
		AppCode:   appCode,
		AppSecret: appSecret,
		UserName:  userName,
	}
}

// Post do a post request with the given request to the url and set the response into the response param
func (c *Client) Post(url string, requestBody interface{}, response interface{}) error {
	resp := new(GeneralResponse)

	authHeader, err := c.generateAuthHeader()
	if err != nil {
		return fmt.Errorf("failed to generate auth header for blueking request, err: %s", err)
	}

	_, err = httpclient.Post(
		url,
		httpclient.SetHeader("X-Bkapi-Authorization", authHeader),
		httpclient.SetHeader("Content-Type", "application/json"),
		httpclient.SetResult(resp),
		httpclient.SetBody(requestBody),
	)

	if err != nil {
		return fmt.Errorf("failed to do blueking request, error: %s", err)
	}

	if hasErr, errMsg := resp.HasError(); hasErr {
		return fmt.Errorf(errMsg)
	}

	err = resp.DecodeResponseData(response)
	if err != nil {
		return err
	}

	return nil
}

func (c *Client) Get(url string, queries map[string]string, response interface{}) error {
	resp := new(GeneralResponse)

	authHeader, err := c.generateAuthHeader()
	if err != nil {
		return fmt.Errorf("failed to generate auth header for blueking request, err: %s", err)
	}

	_, err = httpclient.Get(
		url,
		httpclient.SetHeader("X-Bkapi-Authorization", authHeader),
		httpclient.SetHeader("Content-Type", "application/json"),
		httpclient.SetResult(resp),
		httpclient.SetQueryParams(queries),
	)

	if err != nil {
		return fmt.Errorf("failed to do blueking request, error: %s", err)
	}

	if hasErr, errMsg := resp.HasError(); hasErr {
		return fmt.Errorf(errMsg)
	}

	err = resp.DecodeResponseData(response)
	if err != nil {
		return err
	}

	return nil
}
