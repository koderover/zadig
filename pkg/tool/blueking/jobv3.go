/*
 * Copyright 2024 The KodeRover Authors.
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

/*
	蓝鲸作业平台 v3 相关接口
*/

package blueking

import (
	"fmt"
	"strings"

	"github.com/koderover/zadig/v2/pkg/tool/log"
)

type ListExecutionPlanReq struct {
	BusinessID int64 `json:"bk_biz_id"`

	// General query field
	JobPlanId int64  `json:"job_plan_id,omitempty"`
	Name      string `json:"name,omitempty"`
	// Paging request
	Start  int64 `json:"start"`
	Length int64 `json:"length"`
}

func (c *Client) ListExecutionPlan(businessID int64, name string, start, perPage int64) ([]*ExecutionPlanBrief, error) {
	url := fmt.Sprintf("%s/%s", strings.TrimSuffix(c.Host, "/"), ListExecutionPlanAPI)
	request := &ListExecutionPlanReq{
		BusinessID: businessID,
		Start:      start,
		Length:     perPage,
		Name:       name,
	}

	resp := make([]*ExecutionPlanBrief, 0)

	err := c.Post(url, request, &resp)
	if err != nil {
		log.Errorf("failed to list execution plan from blueking, error: %s", err)
		return nil, err
	}

	return resp, nil
}

type GetExecutionPlanDetailReq struct {
	BusinessID      int64 `json:"bk_biz_id"`
	ExecutionPlanID int64 `json:"job_plan_id"`
}

func (c *Client) GetExecutionPlanDetail(businessID, executionPlanID int64) (*ExecutionPlanDetail, error) {
	url := fmt.Sprintf("%s/%s", strings.TrimSuffix(c.Host, "/"), GetExecutionPlanDetailAPI)
	request := &GetExecutionPlanDetailReq{
		BusinessID:      businessID,
		ExecutionPlanID: executionPlanID,
	}

	executionPlanDetail := new(ExecutionPlanDetail)

	err := c.Post(url, request, executionPlanDetail)
	if err != nil {
		log.Errorf("failed to get execution plan detail from blueking, error: %s", err)
		return nil, err
	}

	return executionPlanDetail, nil
}

func (c *Client) RunExecutionPlan(businessID, executionPlanID int64, params []*GlobalVariable) (*JobInstanceBrief, error) {
	url := fmt.Sprintf("%s/%s", strings.TrimPrefix(c.Host, "/"), RunExecutionPlanAPI)
	requestBody := &ExecutionPlanDetail{
		BusinessID:         businessID,
		ExecutionPlanID:    executionPlanID,
		GlobalVariableList: params,
	}

	jobInstance := new(JobInstanceBrief)

	err := c.Post(url, requestBody, jobInstance)
	if err != nil {
		log.Errorf("failed to run execution plan , error: %s", err)
		return nil, err
	}

	return jobInstance, nil
}

type GetExecutionPlanInstanceReq struct {
	BusinessID    int64 `json:"bk_biz_id"`
	JobInstanceId int64 `json:"job_instance_id"`
}

func (c *Client) GetExecutionPlanInstance(businessID, jobInstanceID int64) (*JobInstanceDetail, error) {
	url := fmt.Sprintf("%s/%s", strings.TrimPrefix(c.Host, "/"), GetJobInstanceAPI)
	requestBody := &GetExecutionPlanInstanceReq{
		BusinessID:    businessID,
		JobInstanceId: jobInstanceID,
	}

	jobInstance := new(JobInstanceDetail)

	err := c.Post(url, requestBody, jobInstance)
	if err != nil {
		log.Errorf("failed to run execution plan , error: %s", err)
		return nil, err
	}

	return jobInstance, nil
}
