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

package workwx

import (
	"fmt"
	"strconv"

	"github.com/koderover/zadig/v2/pkg/tool/httpclient"
)

func (c *Client) ListDepartmentUsers(departmentID int) (*ListDepartmentUserResp, error) {
	url := fmt.Sprintf("%s/%s", c.Host, listDepartmentUserAPI)

	accessToken, err := c.getAccessToken()
	if err != nil {
		return nil, err
	}

	requestQuery := map[string]string{
		"access_token": accessToken,
		"id":           strconv.Itoa(departmentID),
	}

	resp := new(ListDepartmentUserResp)

	_, err = httpclient.Get(
		url,
		httpclient.SetQueryParams(requestQuery),
		httpclient.SetResult(&resp),
	)

	if err != nil {
		return nil, err
	}

	if wxErr := resp.ToError(); wxErr != nil {
		return nil, wxErr
	}

	return resp, nil
}

func (c *Client) FindUserByPhone(phone int) (interface{}, error) {
	url := fmt.Sprintf("%s/%s", c.Host, getUserIDByPhoneAPI)

	accessToken, err := c.getAccessToken()
	if err != nil {
		return nil, err
	}

	requestQuery := map[string]string{
		"access_token": accessToken,
		"mobile":       strconv.Itoa(phone),
	}

	resp := new(FindUserByPhoneResp)

	_, err = httpclient.Get(
		url,
		httpclient.SetQueryParams(requestQuery),
		httpclient.SetResult(&resp),
	)

	if err != nil {
		return nil, err
	}

	if wxErr := resp.ToError(); wxErr != nil {
		return nil, wxErr
	}

	return resp, nil
}
