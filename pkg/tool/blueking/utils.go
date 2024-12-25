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
	// if nothing is required, then just do nothing
	if target == nil {
		return nil
	}

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
