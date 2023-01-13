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

package httpclient

import (
	"crypto/tls"
	"time"
)

type ClientFunc func(*Client)

func SetBasicAuth(username, password string) ClientFunc {
	return func(c *Client) {
		c.Client.SetBasicAuth(username, password)
	}
}

func SetAuthScheme(scheme string) ClientFunc {
	return func(c *Client) {
		c.Client.SetAuthScheme(scheme)
	}
}

func SetAuthToken(token string) ClientFunc {
	return func(c *Client) {
		c.Client.SetAuthToken(token)
	}
}

func SetBaseURI(uri string) ClientFunc {
	return func(c *Client) {
		c.BaseURI = uri
	}
}

func SetHostURL(url string) ClientFunc {
	return func(c *Client) {
		c.Client.SetHostURL(url)
		c.Host = url
	}
}

func SetProxy(proxyURL string) ClientFunc {
	return func(c *Client) {
		c.Client.SetProxy(proxyURL)
	}
}

func UnsetTimeout() ClientFunc {
	return func(c *Client) {
		c.Client.SetTimeout(0)
	}
}

func SetIgnoreCodes(codes ...int) ClientFunc {
	return func(c *Client) {
		c.IgnoreCodes.Insert(codes...)
	}
}

func SetRetryCount(count int) ClientFunc {
	return func(c *Client) {
		c.Client.SetRetryCount(count)
	}
}

func SetRetryWaitTime(waitTime time.Duration) ClientFunc {
	return func(c *Client) {
		c.Client.SetRetryWaitTime(waitTime)
	}
}

func SetTLSClientConfig(config *tls.Config) ClientFunc {
	return func(c *Client) {
		c.Client.SetTLSClientConfig(config)
	}
}

func SetClientHeader(header, value string) ClientFunc {
	return func(c *Client) {
		c.Client.SetHeader(header, value)
	}
}
