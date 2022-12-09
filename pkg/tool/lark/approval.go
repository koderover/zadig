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
	"strconv"
	"time"

	larkapproval "github.com/larksuite/oapi-sdk-go/v3/service/approval/v4"
	"github.com/pkg/errors"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/log"
)

type CreateApprovalDefinitionArgs struct {
	Name        string
	Description string
	Type        ApprovalType
}

func (client *Client) CreateApprovalDefinition(arg *CreateApprovalDefinitionArgs) (string, error) {
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
			NodeList([]*larkapproval.ApprovalNode{
				larkapproval.NewApprovalNodeBuilder().Id(`START`).Build(),
				larkapproval.NewApprovalNodeBuilder().Id(`APPROVE`).Name(approvalNodeApproveI18NKey).
					NodeType(string(arg.Type)).
					Approver([]*larkapproval.ApprovalApproverCcer{
						larkapproval.NewApprovalApproverCcerBuilder().Type(ApproverSelectionMethodFree).Build(),
					}).Build(),
				larkapproval.NewApprovalNodeBuilder().Id(`END`).Build(),
			}).
			I18nResources([]*larkapproval.I18nResource{
				larkapproval.NewI18nResourceBuilder().
					Locale(`zh-CN`).
					Texts([]*larkapproval.I18nResourceText{
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
							Key(approvalNodeApproveI18NKey).
							Value(defaultNodeApproveValue).
							Build(),
					}).
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

type CreateApprovalInstanceArgs struct {
	ApprovalCode   string
	UserOpenID     string
	ApproverIDList []string
	FormContent    string
}

func (client *Client) CreateApprovalInstance(args *CreateApprovalInstanceArgs) (string, error) {
	log.Infof("create approval instance: approver id list %v", args.ApproverIDList)
	formContent, err := json.Marshal([]formData{{
		ID:    "1",
		Type:  "textarea",
		Value: args.FormContent,
	}})
	if err != nil {
		return "", errors.Wrap(err, "marshal form data")
	}

	req := larkapproval.NewCreateInstanceReqBuilder().
		InstanceCreate(larkapproval.NewInstanceCreateBuilder().
			ApprovalCode(args.ApprovalCode).
			OpenId(args.UserOpenID).
			Form(string(formContent)).
			NodeApproverOpenIdList([]*larkapproval.NodeApprover{
				larkapproval.NewNodeApproverBuilder().
					Key(`APPROVE`).
					Value(args.ApproverIDList).
					Build(),
			}).
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

type ApprovalInstanceInfo struct {
	config.ApproveOrReject
	ApproverInfo *UserInfo
	Comment      string
	Time         int64
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

	for _, timeline := range resp.Data.Timeline {
		status := getStringFromPointer(timeline.Type)
		if status == "PASS" || status == "REJECT" {
			user, err := client.GetUserInfoByID(getStringFromPointer(timeline.OpenId))
			if err != nil {
				return nil, errors.Wrap(err, "get user")
			}
			ts, _ := strconv.ParseInt(getStringFromPointer(timeline.CreateTime), 10, 64)
			ts /= 1000
			if ts == 0 {
				ts = time.Now().Unix()
			}
			return &ApprovalInstanceInfo{
				ApproveOrReject: m[status],
				ApproverInfo:    user,
				Comment:         getStringFromPointer(timeline.Comment),
				Time:            ts,
			}, nil
		}
	}
	return nil, errors.New("not found timeline")
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
