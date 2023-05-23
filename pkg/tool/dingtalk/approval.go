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

import "github.com/tidwall/gjson"

const (
	defaultApprovalFormName  = "Zadig 审批表单模板"
	defaultApprovalFormLabel = "详情"
)

var (
	defaultApprovalFormDefinition = ApprovalFormDefinition{
		Name:        defaultApprovalFormName,
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

func (c *Client) CreateApproval() (string, error) {
	//type result struct {
	//	Result struct {
	//		ProcessCode string `json:"processCode"`
	//	} `json:"result"`
	//}
	//var re *result
	resp, err := c.R().SetBodyJsonMarshal(defaultApprovalFormDefinition).
		//SetSuccessResult(&re).
		Post("https://api.dingtalk.com/v1.0/workflow/forms")
	if err != nil {
		return "", err
	}
	return gjson.Get(resp.String(), "result.processCode").String(), nil
}

type ApprovalInstance struct {
	ProcessCode         string               `json:"processCode"`
	Originator          string               `json:"originatorUserId"`
	Approvers           []ApprovalNode       `json:"approvers"`
	FormComponentValues []FormComponentValue `json:"formComponentValues"`
	MicroAgentID        int                  `json:"microappAgentId,omitempty"`
}

type FormComponentValue struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type ApprovalNode struct {
	ActionType string   `json:"actionType"`
	UserIDs    []string `json:"userIds"`
}

type CreateApprovalInstanceArgs struct {
	ProcessCode      string
	OriginatorUserID string
	ApproverIDList   []string
	FormContent      string
}

func (c *Client) CreateApprovalInstance(args *CreateApprovalInstanceArgs) (string, error) {
	type result struct {
		Result struct {
			InstanceID string `json:"instanceId"`
		} `json:"result"`
	}
	var re *result
	_, err := c.R().SetBodyJsonMarshal(ApprovalInstance{
		ProcessCode: args.ProcessCode,
		Originator:  args.OriginatorUserID,
		Approvers: []ApprovalNode{
			//{
			//	ActionType: "NONE",
			//	UserIDs:    args.ApproverIDList,
			//},
			{
				ActionType: "OR",
				UserIDs:    args.ApproverIDList,
			}},
		FormComponentValues: []FormComponentValue{
			{
				Name:  defaultApprovalFormLabel,
				Value: args.FormContent,
			},
		},
		MicroAgentID: 2594089750,
	}).
		SetSuccessResult(&re).
		Post("https://api.dingtalk.com/v1.0/workflow/processInstances")
	if err != nil {
		return "", err
	}
	return re.Result.InstanceID, nil
}
