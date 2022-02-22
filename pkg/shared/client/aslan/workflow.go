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

	"github.com/koderover/zadig/pkg/tool/httpclient"
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
