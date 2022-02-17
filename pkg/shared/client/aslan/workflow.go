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
