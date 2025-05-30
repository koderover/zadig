/*
Copyright 2024 The KodeRover Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package workwx

import (
	"fmt"

	"github.com/koderover/zadig/v2/pkg/tool/httpclient"
)

type createApprovalTemplateReq struct {
	TemplateName    []*GeneralText           `json:"template_name"`
	TemplateContent *ApprovalTemplateContent `json:"template_content"`
}

func (c *Client) CreateApprovalTemplate(templateName []*GeneralText, controls []*ApprovalControl) (string, error) {
	url := fmt.Sprintf("%s/%s", c.Host, createApprovalTemplateDetailAPI)

	accessToken, err := c.getAccessToken(false)
	if err != nil {
		return "", err
	}

	requestQuery := map[string]string{
		"access_token": accessToken,
	}

	requestBody := &createApprovalTemplateReq{
		TemplateName:    templateName,
		TemplateContent: &ApprovalTemplateContent{Controls: controls},
	}

	resp := new(createApprovalTemplateResponse)

	_, err = httpclient.Post(
		url,
		httpclient.SetQueryParams(requestQuery),
		httpclient.SetBody(requestBody),
		httpclient.SetResult(&resp),
	)

	if err != nil {
		return "", err
	}

	if resp.ToError() != nil {
		return "", resp.ToError()
	}

	return resp.TemplateID, nil
}

// TODO: Add chooseDepartment param support, for now it is useless for us.
func (c *Client) CreateApprovalInstance(templateID, applicant string, useTemplateApprover bool, input []*ApplyDataContent, approveNodes []*ApprovalNode, summary []*ApprovalSummary) (string, error) {
	if useTemplateApprover && len(approveNodes) > 0 {
		return "", fmt.Errorf("cannot pass approval node while not using it")
	}

	if len(summary) > 3 {
		return "", fmt.Errorf("maximum summary length supported is 3")
	}

	url := fmt.Sprintf("%s/%s", c.Host, createApprovalInstanceAPI)

	accessToken, err := c.getAccessToken(false)
	if err != nil {
		return "", err
	}

	useTemplateApproverInt := 0
	if useTemplateApprover {
		useTemplateApproverInt = 1
	}

	req := &createApprovalInstanceReq{
		CreatorUserID:       applicant,
		TemplateID:          templateID,
		UseTemplateApprover: useTemplateApproverInt,
		ChooseDepartment:    0,
		ApplyData:           &ApprovalApplyData{input},
		Process:             &ApprovalNodes{NodeList: approveNodes},
		SummaryList:         summary,
	}

	requestQuery := map[string]string{
		"access_token": accessToken,
	}

	resp := new(createApprovalInstanceResp)

	_, err = httpclient.Post(
		url,
		httpclient.SetQueryParams(requestQuery),
		httpclient.SetBody(req),
		httpclient.SetResult(&resp),
	)

	if err != nil {
		return "", err
	}

	if wxErr := resp.ToError(); wxErr != nil {
		return "", wxErr
	}

	return resp.ApprovalInstanceID, nil
}
