// /*
// * Copyright 2023 The KodeRover Authors.
// *
// * Licensed under the Apache License, Version 2.0 (the "License");
// * you may not use this file except in compliance with the License.
// * You may obtain a copy of the License at
// *
// *     http://www.apache.org/licenses/LICENSE-2.0
// *
// * Unless required by applicable law or agreed to in writing, software
// * distributed under the License is distributed on an "AS IS" BASIS,
// * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// * See the License for the specific language governing permissions and
// * limitations under the License.
// */
package dingtalk

import (
	"strings"

	"github.com/pkg/errors"
)

const (
	DefaultApprovalFormName  = "Zadig 审批表单模板"
	defaultApprovalFormLabel = "详情"
)

type ApprovalAction string

const (
	AND  = "AND"
	OR   = "OR"
	NONE = "NONE"
)

type ApprovalFormDefinition struct {
	ProcessCode    string           `json:"processCode"`
	Name           string           `json:"name"`
	Description    string           `json:"description"`
	FormComponents []FormComponents `json:"formComponents"`
	TemplateConfig *TemplateConfig  `json:"templateConfig"`
}

type FormComponents struct {
	ComponentType string `json:"componentType"`
	Props         Props  `json:"props,omitempty"`
}

type Props struct {
	Label       string `json:"label"`
	Placeholder string `json:"placeholder"`
	ComponentID string `json:"componentId"`
	Required    bool   `json:"required"`
}

type TemplateConfig struct {
	DisableFormEdit bool `json:"disableFormEdit"`
}

type CreateApprovalResponse struct {
	ProcessCode string `json:"processCode"`
}

func (c *Client) CreateApproval() (resp *CreateApprovalResponse, err error) {
	_, err = c.R().SetBodyJsonMarshal(getRandNameDefaultApprovalFormDefinition()).
		SetSuccessResult(&resp).
		Post("https://api.dingtalk.com/v1.0/workflow/forms")
	if err != nil && strings.Contains(err.Error(), "已有相同名称表单") {
		return nil, ErrApprovalFormNameExists
	}
	return
}

func getRandNameDefaultApprovalFormDefinition() ApprovalFormDefinition {
	return ApprovalFormDefinition{
		Name:        DefaultApprovalFormName,
		Description: "用于 Zadig Workflow 审批",
		FormComponents: []FormComponents{
			{
				ComponentType: "TextareaField",
				Props: Props{
					Label:       defaultApprovalFormLabel,
					Placeholder: "请输入详情",
					ComponentID: "TextareaField-1",
					Required:    true,
				},
			},
		},
	}
}

type GetAllApprovalFormDefinitionResponse struct {
	ProcessList []ApprovalForm `json:"processList"`
	NextToken   int            `json:"nextToken"`
}

type ApprovalForm struct {
	Name        string `json:"name"`
	URL         string `json:"url"`
	IconURL     string `json:"iconUrl"`
	ProcessCode string `json:"processCode"`
}

func (c *Client) GetApprovalFormDefinitionList(next int) (resp *GetAllApprovalFormDefinitionResponse, err error) {
	_, err = c.R().SetQueryParamsAnyType(map[string]interface{}{
		"maxResults": 100,
		"nextToken":  next,
	}).
		SetSuccessResult(&resp).Get("https://api.dingtalk.com/v1.0/workflow/processes/userVisibilities/templates")
	return
}

func (c *Client) GetAllApprovalFormDefinitionList() (resp []ApprovalForm, err error) {
	next := 0
	for {
		result, err := c.GetApprovalFormDefinitionList(next)
		if err != nil {
			return nil, errors.Wrapf(err, "get %d list", next)
		}
		resp = append(resp, result.ProcessList...)
		next = result.NextToken
		if next == 0 {
			return resp, nil
		}
	}
}

type ApprovalInstance struct {
	ProcessCode         string               `json:"processCode"`
	Originator          string               `json:"originatorUserId"`
	Approvers           []*ApprovalNode      `json:"approvers"`
	FormComponentValues []FormComponentValue `json:"formComponentValues"`
	MicroAgentID        int                  `json:"microappAgentId,omitempty"`
}

type FormComponentValue struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type ApprovalNode struct {
	ActionType ApprovalAction `json:"actionType"`
	UserIDs    []string       `json:"userIds"`
}

type CreateApprovalInstanceArgs struct {
	ProcessCode      string
	OriginatorUserID string
	ApproverNodeList []*ApprovalNode
	FormContent      string
}

type CreateApprovalInstanceResponse struct {
	InstanceID string `json:"instanceId"`
}

func (c *Client) CreateApprovalInstance(args *CreateApprovalInstanceArgs) (resp *CreateApprovalInstanceResponse, err error) {
	for _, node := range args.ApproverNodeList {
		if len(node.UserIDs) == 1 {
			node.ActionType = NONE
		}
	}
	_, err = c.R().
		SetBodyJsonMarshal(ApprovalInstance{
			ProcessCode: args.ProcessCode,
			Originator:  args.OriginatorUserID,
			Approvers:   args.ApproverNodeList,
			FormComponentValues: []FormComponentValue{
				{
					Name:  defaultApprovalFormLabel,
					Value: args.FormContent,
				},
			},
		}).
		SetSuccessResult(&resp).
		Post("https://api.dingtalk.com/v1.0/workflow/processInstances")
	return
}

type ApprovalInstanceInfo struct {
	Title            string                  `json:"title"`
	Status           string                  `json:"status"`
	Result           string                  `json:"result"`
	ApproverUserIds  []string                `json:"approverUserIds"`
	OperationRecords []*OperationRecord      `json:"operationRecords"`
	Tasks            []*ApprovalInstanceTask `json:"tasks"`
}

type OperationRecord struct {
	UserID string `json:"userid"`
	Date   string `json:"date"`
	Result string `json:"result"`
	Remark string `json:"remark"`
}

type ApprovalInstanceTask struct {
	UserID     string `json:"userid"`
	Result     string `json:"result"`
	ActivityID string `json:"activityId"`
}

func (c *Client) GetApprovalInstance(id string) (resp *ApprovalInstanceInfo, err error) {
	_, err = c.R().SetQueryParam("processInstanceId", id).
		SetSuccessResult(&resp).
		Get("https://api.dingtalk.com/v1.0/workflow/processInstances")
	return
}
