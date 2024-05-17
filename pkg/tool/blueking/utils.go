/*
 * Copyright 2024 The KodeRover Authors.
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

package blueking

import (
	"encoding/json"
	"fmt"

	"github.com/koderover/zadig/v2/pkg/tool/httpclient"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

func (c *Client) generateAuthHeader() (string, error) {
	header := &AuthorizationHeader{
		AppCode:   c.AppCode,
		AppSecret: c.AppSecret,
		Username:  c.UserName,
	}

	serializedData, err := json.Marshal(header)
	if err != nil {
		return "", err
	}

	return string(serializedData), nil
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

	fmt.Println("!!!!!!!!!", resp)

	if err != nil {
		return fmt.Errorf("failed to do blueking request, error: %s", err)
	}

	fmt.Println(resp.Data)

	err = resp.DecodeResponseData(response)
	if err != nil {
		return err
	}

	return nil
}

// HasError returns if the request is success, if not, return the error message in the second response
func (resp *GeneralResponse) HasError() (bool, string) {
	if !resp.Result {
		return true, resp.Message
	}
	return false, ""
}

// DecodeResponseData decode the data in the response's Data field and try to unmarshal it into the target
// USE POINTER TYPE OR IT WILL FAIL
func (resp *GeneralResponse) DecodeResponseData(target interface{}) error {
	dt, err := json.Marshal(resp.Data)
	if err != nil {
		log.Errorf("failed to decode blueking system's response, error: %s", err)
		return fmt.Errorf("failed to decode blueking system's response, error: %s", err)
	}

	err = json.Unmarshal(dt, target)
	if err != nil {
		log.Errorf("failed to unmarshal blueking system's response into target, error: %s", err)
		return fmt.Errorf("failed to unmarshal blueking system's response into target, error: %s", err)
	}

	return nil
}
