/*
 * Copyright 2023 The KodeRover Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package apollo

import (
	"github.com/imroc/req/v3"
	"github.com/pkg/errors"
)

type Client struct {
	*req.Client
	BaseURL string
}

func NewClient(url, token string) *Client {
	return &Client{
		Client: req.C().
			SetCommonHeader("Authorization", token).
			OnAfterResponse(func(client *req.Client, resp *req.Response) error {
				if resp.Err != nil {
					resp.Err = errors.Wrapf(resp.Err, "body: %s", resp.String())
					return nil
				}
				if !resp.IsSuccessState() {
					resp.Err = errors.Errorf("unexpected status code %d, body: %s", resp.GetStatusCode(), resp.String())
					return nil
				}
				return nil
			}),
		BaseURL: url,
	}
}
