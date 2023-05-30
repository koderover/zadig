/*
 * Copyright 2022 The KodeRover Authors.
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

package lark

import (
	"context"
	"encoding/json"
	"fmt"

	larkapproval "github.com/larksuite/oapi-sdk-go/v3/service/approval/v4"
	"github.com/pkg/errors"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/lark"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/log"
)

type CreateApprovalDefinitionArgs struct {
	Name        string
	Description string
	Nodes       []*ApprovalNode
}

type ApprovalNode struct {
	ApproverIDList []string
	Type           ApproveType
}

func (client *Client) CreateApprovalDefinition(arg *CreateApprovalDefinitionArgs) (string, error) {
	i18nTextList := []*larkapproval.I18nResourceText{
		larkapproval.NewI18nResourceTextBuilder().
			Key(approvalNameI18NKey).
			Value(arg.Name).
			Build(),
		larkapproval.NewI18nResourceTextBuilder().
			Key(approvalDescriptionI18NKey).
			Value(arg.Description).
			Build(),
		larkapproval.NewI18nResourceTextBuilder().
			Key(approvalFormNameI18NKey).
			Value(approvalFormNameI18NValue).
			Build(),
		larkapproval.NewI18nResourceTextBuilder().
			Key(approvalFormValueI18NKey).
			Value(defaultFormValueI18NValue).
			Build(),
		larkapproval.NewI18nResourceTextBuilder().
			Key(approvalNodeApproveI18NKeyTmpl).
			Value(approvalNodeNameValueTmpl).
			Build(),
	}
	larkApprovalNodeList := make([]*larkapproval.ApprovalNode, 0)
	larkApprovalNodeList = append(larkApprovalNodeList, larkapproval.NewApprovalNodeBuilder().Id(`START`).Build())
	for i, node := range arg.Nodes {
		larkApprovalNodeList = append(larkApprovalNodeList, larkapproval.NewApprovalNodeBuilder().
			Id(ApprovalNodeIDKey(i)).
			Name(approvalNodeApproveI18NKey(i)).
			NodeType(string(node.Type)).
			Approver([]*larkapproval.ApprovalApproverCcer{
				larkapproval.NewApprovalApproverCcerBuilder().Type(ApproverSelectionMethodFree).Build(),
			}).Build())
		i18nTextList = append(i18nTextList, larkapproval.NewI18nResourceTextBuilder().
			Key(approvalNodeApproveI18NKey(i)).
			Value(approvalNodeNameValue(i)).
			Build())
	}
	larkApprovalNodeList = append(larkApprovalNodeList, larkapproval.NewApprovalNodeBuilder().Id(`END`).Build())

	req := larkapproval.NewCreateApprovalReqBuilder().
		UserIdType(setting.LarkUserOpenID).
		ApprovalCreate(larkapproval.NewApprovalCreateBuilder().
			ApprovalName(approvalNameI18NKey).
			ApprovalCode(``).
			Description(approvalDescriptionI18NKey).
			Viewers([]*larkapproval.ApprovalCreateViewers{
				larkapproval.NewApprovalCreateViewersBuilder().
					ViewerType(`NONE`).
					Build(),
			}).
			Form(larkapproval.NewApprovalFormBuilder().
				FormContent(fmt.Sprintf(`[{"id":"1","name":"%s","type":"textarea","required":false,"value":"%s"}]`, approvalFormNameI18NKey, approvalFormValueI18NKey)).
				Build()).
			NodeList(larkApprovalNodeList).
			I18nResources([]*larkapproval.I18nResource{
				larkapproval.NewI18nResourceBuilder().
					Locale(`zh-CN`).
					Texts(i18nTextList).
					IsDefault(true).
					Build(),
			}).
			Build()).
		Build()

	resp, err := client.Approval.Approval.Create(context.Background(), req)
	if err != nil {
		return "", errors.Wrap(err, "lark client")
	}

	if !resp.Success() {
		return "", resp.CodeError
	}
	if resp.Data.ApprovalCode == nil {
		return "", errors.New("get nil approval code")
	}

	return *resp.Data.ApprovalCode, nil
}

func ApprovalNodeIDKey(id int) string {
	return fmt.Sprintf(approvalNodeIDKeyTmpl, id)
}

func approvalNodeApproveI18NKey(id int) string {
	return fmt.Sprintf(approvalNodeApproveI18NKeyTmpl, id)
}

func approvalNodeNameValue(id int) string {
	return fmt.Sprintf(approvalNodeNameValueTmpl, id)
}

func (client *Client) GetApprovalDefinition(approvalCode string) (*larkapproval.GetApprovalRespData, error) {
	req := larkapproval.NewGetApprovalReqBuilder().
		ApprovalCode(approvalCode).
		Build()
	resp, err := client.Approval.Approval.Get(context.Background(), req)
	if err != nil {
		return nil, errors.Wrap(err, "lark client")
	}

	if !resp.Success() {
		return nil, resp.CodeError
	}
	return resp.Data, nil
}

func (client *Client) GetApprovalDefinitionNodeKeyMap(approvalCode string) (map[string]string, error) {
	resp, err := client.GetApprovalDefinition(approvalCode)
	if err != nil {
		return nil, err
	}

	nodeKeyMap := make(map[string]string)
	for _, node := range resp.NodeList {
		nodeKeyMap[getStringFromPointer(node.CustomNodeId)] = getStringFromPointer(node.NodeId)
	}

	//todo debug
	log.Infof("node key map: %v", nodeKeyMap)
	return nodeKeyMap, nil
}

type CreateApprovalInstanceArgs struct {
	ApprovalCode string
	UserOpenID   string
	Nodes        []*ApprovalNode
	FormContent  string
}

func (client *Client) CreateApprovalInstance(args *CreateApprovalInstanceArgs) (string, error) {
	log.Infof("create approval instance: approver node num %d", len(args.Nodes))
	formContent, err := json.Marshal([]formData{{
		ID:    "1",
		Type:  "textarea",
		Value: args.FormContent,
	}})
	if err != nil {
		return "", errors.Wrap(err, "marshal form data")
	}

	nodeList := make([]*larkapproval.NodeApprover, 0)
	for i, node := range args.Nodes {
		nodeList = append(nodeList, larkapproval.NewNodeApproverBuilder().
			Key(ApprovalNodeIDKey(i)).
			Value(node.ApproverIDList).
			Build())
	}

	req := larkapproval.NewCreateInstanceReqBuilder().
		InstanceCreate(larkapproval.NewInstanceCreateBuilder().
			ApprovalCode(args.ApprovalCode).
			OpenId(args.UserOpenID).
			Form(string(formContent)).
			NodeApproverOpenIdList(nodeList).
			Build()).
		Build()

	resp, err := client.Approval.Instance.Create(context.Background(), req)
	if err != nil {
		return "", err
	}

	if !resp.Success() {
		return "", resp.CodeError
	}
	if resp.Data.InstanceCode == nil {
		return "", errors.New("failed to create approval instance")
	}

	return *resp.Data.InstanceCode, nil
}

type GetApprovalInstanceArgs struct {
	InstanceID string
}

type UserApprovalResult struct {
	Comment string
}

type ApprovalInstanceInfo struct {
	// key1 is node id, key2 is user open id
	ApproverInfoWithNode map[string]map[string]*UserApprovalResult
	ApproveOrReject      config.ApproveOrReject
}

func (client *Client) GetApprovalInstance(args *GetApprovalInstanceArgs) (*ApprovalInstanceInfo, error) {
	req := larkapproval.NewGetInstanceReqBuilder().
		InstanceId(args.InstanceID).
		Build()

	resp, err := client.Approval.Instance.Get(context.Background(), req)
	if err != nil {
		return nil, errors.Wrap(err, "send request")
	}

	if !resp.Success() {
		return nil, resp.CodeError
	}

	m := map[string]config.ApproveOrReject{
		"PASS":   config.Approve,
		"REJECT": config.Reject,
	}
	resultMap := make(map[string]map[string]*UserApprovalResult)
	// todo debug
	b, _ := json.MarshalIndent(resp.Data.Timeline, "", "  ")
	log.Infof("timeline: %s", string(b))

	for _, timeline := range resp.Data.Timeline {
		status := getStringFromPointer(timeline.Type)
		if status == "PASS" || status == "REJECT" {
			nodeKey, userID := getStringFromPointer(timeline.NodeKey), getStringFromPointer(timeline.OpenId)
			if nodeKey == "" {
				log.Warn("node key is empty")
				continue
			}
			if resultMap[nodeKey] == nil {
				resultMap[nodeKey] = make(map[string]*UserApprovalResult)
			}
			resultMap[nodeKey][userID] = &UserApprovalResult{
				Comment: getStringFromPointer(timeline.Comment),
			}
		}
	}
	return &ApprovalInstanceInfo{
		ApproverInfoWithNode: resultMap,
		ApproveOrReject: map[string]config.ApproveOrReject{
			lark.ApprovalStatusApproved: config.Approve,
			lark.ApprovalStatusRejected: config.Reject}[getStringFromPointer(resp.Data.Status)],
	}, nil
}

type CancelApprovalInstanceArgs struct {
	ApprovalID string
	InstanceID string
	UserID     string
}

func (client *Client) CancelApprovalInstance(args *CancelApprovalInstanceArgs) error {
	req := larkapproval.NewCancelInstanceReqBuilder().
		UserIdType(setting.LarkUserOpenID).
		InstanceCancel(larkapproval.NewInstanceCancelBuilder().
			ApprovalCode(args.ApprovalID).
			InstanceCode(args.InstanceID).
			UserId(args.UserID).
			Build()).
		Build()

	resp, err := client.Approval.Instance.Cancel(context.Background(), req)
	if err != nil {
		return err
	}

	if !resp.Success() {
		return resp.CodeError
	}
	return nil
}

type SubscribeApprovalDefinitionArgs struct {
	ApprovalID string
}

func (client *Client) SubscribeApprovalDefinition(args *SubscribeApprovalDefinitionArgs) error {
	req := larkapproval.NewSubscribeApprovalReqBuilder().
		ApprovalCode(args.ApprovalID).
		Build()

	resp, err := client.Approval.Approval.Subscribe(context.Background(), req)
	if err != nil {
		return errors.Wrap(err, "send request")
	}

	if !resp.Success() {
		return resp.CodeError
	}
	return nil
}
