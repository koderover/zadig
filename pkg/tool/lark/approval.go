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

	"github.com/koderover/zadig/pkg/setting"
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
		return "", err
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
