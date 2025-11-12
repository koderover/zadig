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

package meego

import (
	"errors"
	"fmt"

	"github.com/koderover/zadig/v2/pkg/tool/log"

	"github.com/koderover/zadig/v2/pkg/tool/httpclient"
)

type GetWorkItemListReq struct {
	WorkItemName     string   `json:"work_item_name,omitempty"`
	WorkItemTypeKeys []string `json:"work_item_type_keys"`
	WorkItemIDs      []int    `json:"work_item_ids,omitempty"`
	PageSize         int      `json:"page_size"`
	PageNum          int      `json:"page_num"`
}

type GetWorkItemListResp struct {
	Data         []*WorkItem `json:"data"`
	ErrorMessage string      `json:"err_msg"`
	ErrorCode    int         `json:"err_code"`
	Error        interface{} `json:"error"`
}

type WorkItem struct {
	ID             int               `json:"id"`
	Name           string            `json:"name"`
	SimpleName     string            `json:"simple_name"`
	TemplateType   string            `json:"template_type"`
	Pattern        WorkItemPattern   `json:"pattern"`
	WorkItemStatus *WorkItemStatus   `json:"work_item_status"`
	WorkflowInfos  *NodesConnections `json:"workflow_infos"`
}

type WorkItemStatus struct {
	StateKey string `json:"state_key"`
}

type GetWorkflowReq struct {
	FlowType int64    `json:"flow_type"`
	Fields   []string `json:"fields,omitempty"`
}

type GetWorkflowResp struct {
	Data         *NodesConnections `json:"data"`
	ErrorMessage string            `json:"err_msg"`
	ErrorCode    int               `json:"err_code"`
	Error        interface{}       `json:"error"`
}

type NodesConnections struct {
	WorkflowNodes  []*WorkflowNode  `json:"workflow_nodes"`
	Connections    []*Connection    `json:"connections"`
	StateflowNodes []*StateFlowNode `json:"state_flow_nodes"`
}

type WorkflowNode struct {
	ID       string `json:"id"`
	StateKey string `json:"state_key"`
	Status   int    `json:"status"`
	Name     string `json:"name"`
}

type Connection struct {
	SourceStateKey string `json:"source_state_key"`
	TargetStateKey string `json:"target_state_key"`
	TransitionID   int64  `json:"transition_id"`
}

type StateFlowNode struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status int    `json:"status"`
}

func (c *Client) GetWorkItemList(projectKey, workItemTypeKey, nameQuery string, pageNum, pageSize int) ([]*WorkItem, error) {
	api := fmt.Sprintf("%s/open_api/%s/work_item/filter", c.Host, projectKey)

	result := new(GetWorkItemListResp)

	pageNumber := pageNum
	if pageNumber == 0 {
		pageNumber = 1
	}
	realPageSize := pageSize
	if pageSize == 0 {
		realPageSize = 200
	}

	req := &GetWorkItemListReq{
		WorkItemTypeKeys: []string{workItemTypeKey},
		PageSize:         realPageSize,
		PageNum:          pageNumber,
	}

	if nameQuery != "" {
		req.WorkItemName = nameQuery
	}

	_, err := httpclient.Post(api,
		httpclient.SetBody(req),
		httpclient.SetHeader(PluginTokenHeader, c.PluginToken),
		httpclient.SetHeader(UserKeyHeader, c.UserKey),
		httpclient.SetResult(result),
	)

	if err != nil {
		log.Errorf("error occurred when getting meego workload item list, error: %s", err)
		return nil, err
	}

	if result.ErrorCode != 0 {
		errMsg := fmt.Sprintf("error response when getting meego item list, error code: %d, error message: %s, error: %+v", result.ErrorCode, result.ErrorMessage, result.Error)
		log.Error(errMsg)
		return nil, errors.New(errMsg)
	}

	return result.Data, nil
}

func (c *Client) GetWorkItem(projectKey, workItemTypeKey string, workItemID int) (*WorkItem, error) {
	api := fmt.Sprintf("%s/open_api/%s/work_item/filter", c.Host, projectKey)

	result := new(GetWorkItemListResp)

	pageNum := 1
	pageSize := 200

	req := &GetWorkItemListReq{
		WorkItemTypeKeys: []string{workItemTypeKey},
		WorkItemIDs:      []int{workItemID},
		PageSize:         pageSize,
		PageNum:          pageNum,
	}

	_, err := httpclient.Post(api,
		httpclient.SetBody(req),
		httpclient.SetHeader(PluginTokenHeader, c.PluginToken),
		httpclient.SetHeader(UserKeyHeader, c.UserKey),
		httpclient.SetResult(result),
	)

	if err != nil {
		log.Errorf("error occurred when getting meego workload item list, error: %s", err)
		return nil, err
	}

	if result.ErrorCode != 0 {
		errMsg := fmt.Sprintf("error response when getting meego item list, error code: %d, error message: %s, error: %+v", result.ErrorCode, result.ErrorMessage, result.Error)
		log.Error(errMsg)
		return nil, errors.New(errMsg)
	}

	for i, workItem := range result.Data {
		log.Infof("workitem[%d] ID: %d", i, workItem.ID)
		if workItem.ID == workItemID {
			return workItem, nil
		}
	}

	return nil, errors.New("no work item found")
}

func (c *Client) GetWorkFlowInfo(projectKey, workItemTypeKey string, pattern WorkItemPattern, workItemID int) ([]*Connection, []*WorkflowNode, []*StateFlowNode, error) {
	getWorkflowInfoAPI := fmt.Sprintf("%s/open_api/%s/work_item/%s/%d/workflow/query", c.Host, projectKey, workItemTypeKey, workItemID)

	result := new(GetWorkflowResp)

	flowType := NodeFlowType
	if pattern == WorkItemPatternNode {
		flowType = NodeFlowType
	} else if pattern == WorkItemPatternState {
		flowType = StatusFlowType
	}

	req := &GetWorkflowReq{
		FlowType: int64(flowType),
	}

	_, err := httpclient.Post(getWorkflowInfoAPI,
		httpclient.SetBody(req),
		httpclient.SetHeader(PluginTokenHeader, c.PluginToken),
		httpclient.SetHeader(UserKeyHeader, c.UserKey),
		httpclient.SetResult(result),
	)

	if err != nil {
		log.Errorf("error occurred when getting meego workflow status list, error: %s", err)
		return nil, nil, nil, err
	}

	if result.ErrorCode != 0 {
		errMsg := fmt.Sprintf("error response when getting meego workflow status list, error code: %d, error message: %s, error: %+v", result.ErrorCode, result.ErrorMessage, result.Error)
		log.Error(errMsg)
		return nil, nil, nil, errors.New(errMsg)
	}

	return result.Data.Connections, result.Data.WorkflowNodes, result.Data.StateflowNodes, nil
}

type StatusTransitionReq struct {
	TransitionID int64 `json:"transition_id"`
}

type StatusTransitionResp struct {
	ErrorMessage string      `json:"err_msg"`
	ErrorCode    int         `json:"err_code"`
	Error        interface{} `json:"error"`
}

func (c *Client) StatusTransition(projectKey, workItemTypeKey string, workItemID int, transitionID int64) error {
	statusTransitionAPI := fmt.Sprintf("%s/open_api/%s/workflow/%s/%d/node/state_change", c.Host, projectKey, workItemTypeKey, workItemID)

	result := new(StatusTransitionResp)

	req := &StatusTransitionReq{
		TransitionID: transitionID,
	}

	_, err := httpclient.Post(statusTransitionAPI,
		httpclient.SetBody(req),
		httpclient.SetHeader(PluginTokenHeader, c.PluginToken),
		httpclient.SetHeader(UserKeyHeader, c.UserKey),
		httpclient.SetResult(result),
	)

	if err != nil {
		log.Errorf("error occurred when updating work item status, error: %s", err)
		return err
	}

	if result.ErrorCode != 0 {
		errMsg := fmt.Sprintf("error response when updating work item status, error code: %d, error message: %s, error: %+v", result.ErrorCode, result.ErrorMessage, result.Error)
		log.Error(errMsg)
		return errors.New(errMsg)
	}

	return nil
}

type NodeOperateReq struct {
	Action string `json:"action"`
}

type NodeOperateResp struct {
	ErrorMessage string      `json:"err_msg"`
	ErrorCode    int         `json:"err_code"`
	Error        interface{} `json:"error"`
}

func (c *Client) NodeOperate(projectKey, workItemTypeKey string, workItemID string, nodeID string) error {
	nodeOperateAPI := fmt.Sprintf("%s/open_api/%s/workflow/%s/%s/node/%s/operate", c.Host, projectKey, workItemTypeKey, workItemID, nodeID)

	req := &NodeOperateReq{
		Action: "confirm",
	}

	result := new(NodeOperateResp)
	_, err := httpclient.Post(nodeOperateAPI,
		httpclient.SetBody(req),
		httpclient.SetHeader(PluginTokenHeader, c.PluginToken),
		httpclient.SetHeader(UserKeyHeader, c.UserKey),
	)

	if err != nil {
		log.Errorf("error occurred when operate node, error: %s", err)
		return err
	}

	if result.ErrorCode != 0 {
		errMsg := fmt.Sprintf("error response when operate node, error code: %d, error message: %s, error: %+v", result.ErrorCode, result.ErrorMessage, result.Error)
		log.Error(errMsg)
		return errors.New(errMsg)
	}

	return nil
}
