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

/*
	蓝鲸配置平台相关接口
*/

package blueking

import (
	"fmt"
	"strings"

	"github.com/koderover/zadig/v2/pkg/tool/log"
)

type SearchBusinessReq struct {
	Page *PagingReq `json:"page"`
}

// SearchBusiness 获取业务列表（有搜索分页）
func (c *Client) SearchBusiness() (*BusinessList, error) {
	url := fmt.Sprintf("%s/%s", strings.TrimSuffix(c.Host, "/"), ListBusinessAPI)
	// TODO: hard code to get the maximum number of business
	request := &SearchBusinessReq{Page: &PagingReq{
		Start: 0,
		Limit: 2000,
	}}
	businessList := new(BusinessList)

	err := c.Post(url, request, businessList)
	if err != nil {
		log.Errorf("failed to list business from blueking, error: %s", err)
		return nil, err
	}

	return businessList, nil
}
