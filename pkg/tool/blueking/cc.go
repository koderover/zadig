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
	"strconv"

	"github.com/koderover/zadig/v2/pkg/tool/log"
)

type SearchBusinessReq struct {
	Page *PagingReq `json:"page"`
}

// SearchBusiness 获取业务列表（有搜索分页）
func (c *Client) SearchBusiness(start, limit int64) (*BusinessList, error) {
	url := fmt.Sprintf("%s/%s", c.Host, ListBusinessAPI)
	// TODO: hard code to get the maximum number of business
	request := &SearchBusinessReq{Page: &PagingReq{
		Start: start,
		Limit: limit,
	}}
	businessList := new(BusinessList)

	err := c.Post(url, request, businessList)
	if err != nil {
		log.Errorf("failed to list business from blueking, error: %s", err)
		return nil, err
	}

	return businessList, nil
}

func (c *Client) GetTopology(businessID int64) ([]*TopologyNode, error) {
	url := fmt.Sprintf("%s/%s", c.Host, GetTopologyAPI)
	query := make(map[string]string)
	query["bk_biz_id"] = strconv.FormatInt(businessID, 10)

	topology := make([]*TopologyNode, 0)

	err := c.Get(url, query, &topology)
	if err != nil {
		log.Errorf("failed to list topology from blueking, error: %s", err)
		return nil, err
	}

	return topology, nil
}

type GetHostByTopologyNodeReq struct {
	BusinessID int64      `json:"bk_biz_id"`
	ObjectID   string     `json:"bk_obj_id"`
	InstanceID int64      `json:"bk_inst_id"`
	Fields     []string   `json:"fields"`
	Page       *PagingReq `json:"page"`
}

func (c *Client) GetHostByTopologyNode(businessID, instanceID, start, perPage int64, objectID string) (*HostList, error) {
	url := fmt.Sprintf("%s/%s", c.Host, GetTopologyNodeHostAPI)
	request := &GetHostByTopologyNodeReq{
		BusinessID: businessID,
		ObjectID:   objectID,
		InstanceID: instanceID,
		// TODO: hard code the responding fields, if we need more, add it here
		Fields: []string{"bk_host_id", "bk_cloud_id", "bk_host_innerip"},
		// TODO: hard code to get the maximum number of data in one req
		Page: &PagingReq{
			Start: start,
			Limit: perPage,
		},
	}

	serverList := new(HostList)

	err := c.Post(url, request, serverList)
	if err != nil {
		log.Errorf("failed to find hosts by instanceID: %d, objectID: %s from blueking business: %d, error: %s", instanceID, objectID, businessID, err)
		return nil, err
	}

	return serverList, nil
}

func (c *Client) GetHostByBusiness(businessID, start, perPage int64) (*HostList, error) {
	url := fmt.Sprintf("%s/%s", c.Host, GetBusinessHostAPI)
	request := &GetHostByTopologyNodeReq{
		BusinessID: businessID,
		// TODO: hard code the responding fields, if we need more, add it here
		Fields: []string{"bk_host_id", "bk_cloud_id", "bk_host_innerip", "bk_os_type"},
		// TODO: hard code to get the maximum number of data in one req
		Page: &PagingReq{
			Start: start,
			Limit: perPage,
		},
	}

	serverList := new(HostList)

	err := c.Post(url, request, serverList)
	if err != nil {
		log.Errorf("failed to find hosts by business: %d, error: %s", businessID, err)
		return nil, err
	}

	return serverList, nil
}
