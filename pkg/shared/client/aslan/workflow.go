/*
Copyright 2022 The KodeRover Authors.

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

package aslan

import (
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/tool/httpclient"
	"github.com/koderover/zadig/pkg/tool/log"
)

type workflowConcurrencySettingResp struct {
	WorkflowConcurrency int64 `json:"workflow_concurrency"`
	BuildConcurrency    int64 `json:"build_concurrency"`
}

func (c *Client) GetWorkflowConcurrencySetting() (*workflowConcurrencySettingResp, error) {
	url := "/system/concurrency/workflow"

	resp := &workflowConcurrencySettingResp{}
	res, err := c.Get(url, httpclient.SetResult(resp))
	if err != nil {
		errorMessage := new(ErrorMessage)
		err := json.Unmarshal(res.Body(), errorMessage)
		if err != nil {
			return nil, fmt.Errorf("failed to get workflow concurrency settings, error: %s", err)
		}
	}
	return resp, nil
}

type CreateWorkflowTaskV4Req struct {
	Workflow *models.WorkflowV4
	UserName string
}

type CreateTaskV4Resp struct {
	ProjectName  string `json:"project_name"`
	WorkflowName string `json:"workflow_name"`
	TaskID       int64  `json:"task_id"`
}

func (c *Client) CreateWorkflowTaskV4(req *CreateWorkflowTaskV4Req) (*CreateTaskV4Resp, error) {
	url := "/workflow/v4/workflowtask/trigger"

	resp := &CreateTaskV4Resp{}
	res, err := c.Post(url, httpclient.SetBody(req.Workflow), httpclient.SetQueryParam("triggerName", req.UserName), httpclient.SetResult(resp))
	if err != nil {
		return nil, errors.Wrap(err, "request failed")
	}
	if res.IsSuccess() {
		log.Debugf("AslanClient: create workflow task %s success", req.Workflow.Name)
		return resp, nil
	}
	log.Debugf("AslanClient: create workflow task %s failed, response: %s", req.Workflow.Name, res.String())
	return nil, fmt.Errorf("failed to create workflow task, response: %s", res.String())
}

func (c *Client) CancelWorkflowTaskV4(userName, workflowName string, taskID int64) error {
	url := fmt.Sprintf("/workflow/%s/task/%d", workflowName, taskID)

	res, err := c.Delete(url, httpclient.SetQueryParam("username", userName))
	if err != nil {
		return errors.Wrap(err, "request failed")
	}
	if res.IsSuccess() {
		log.Debugf("AslanClient: cancel workflow task %d success", taskID)
		return nil
	}
	log.Debugf("AslanClient: cancel workflow task %d failed, response: %s", taskID, res.String())
	return fmt.Errorf("failed to cancel workflow task, response: %s", res.String())
}
