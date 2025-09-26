/*
 * Copyright 2023 The KodeRover Authors.
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

package service

import (
	"fmt"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/tool/lark"
	"github.com/pkg/errors"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
)

func lintReleaseJob(_type config.ReleasePlanJobType, spec interface{}) error {
	switch _type {
	case config.JobText:
		t := new(models.TextReleaseJobSpec)
		if err := models.IToi(spec, t); err != nil {
			return fmt.Errorf("invalid text spec: %v", err)
		}
		return nil
	case config.JobWorkflow:
		w := new(models.WorkflowReleaseJobSpec)
		if err := models.IToi(spec, w); err != nil {
			return fmt.Errorf("invalid workflow spec: %v", err)
		}
		return lintWorkflow(w.Workflow)
	default:
		return fmt.Errorf("invalid release job type: %s", _type)
	}
}

func lintReleaseTimeRange(start, end int64) error {
	if start == 0 && end == 0 {
		return nil
	}
	if start > end {
		return errors.New("start time should not be greater than end time")
	}
	return nil
}

func lintScheduleExecuteTime(ScheduleExecuteTime, startTime, endTime int64) error {
	if ScheduleExecuteTime == 0 {
		return nil
	}
	if startTime == 0 && endTime == 0 {
		return nil
	}

	if startTime <= ScheduleExecuteTime && ScheduleExecuteTime <= endTime {
		return nil
	}
	return errors.New("schedule execute time should be in the range of start time and end time")
}

func lintWorkflow(workflow *models.WorkflowV4) error {
	if workflow == nil {
		return fmt.Errorf("workflow cannot be empty")
	}
	return nil
}

func lintApproval(approval *models.Approval) error {
	if approval == nil {
		return nil
	}
	if !approval.Enabled {
		return nil
	}
	switch approval.Type {
	case config.NativeApproval:
		if approval.NativeApproval == nil {
			return errors.New("approval not found")
		}
		allApproveUsers, _ := util.GeneFlatUsers(approval.NativeApproval.ApproveUsers)
		if len(allApproveUsers) < approval.NativeApproval.NeededApprovers {
			return errors.New("all approve users should not less than needed approvers")
		}
	case config.LarkApproval, config.LarkApprovalIntl:
		if approval.LarkApproval == nil {
			return errors.New("approval not found")
		}
		if len(approval.LarkApproval.ApprovalNodes) == 0 {
			return errors.New("num of approval-node is 0")
		}
		for i, node := range approval.LarkApproval.ApprovalNodes {
			if node.Type == lark.ApproveTypeStart || node.Type == lark.ApproveTypeEnd {
				continue
			}
			if len(node.ApproveUsers) == 0 {
				return errors.Errorf("num of approval-node %d approver is 0", i)
			}
			if !lo.Contains([]string{"AND", "OR"}, string(node.Type)) {
				return errors.Errorf("approval-node %d type should be AND or OR", i)
			}
		}
	case config.DingTalkApproval:
		if approval.DingTalkApproval == nil {
			return errors.New("approval not found")
		}
		userIDSets := sets.NewString()
		if len(approval.DingTalkApproval.ApprovalNodes) > 20 {
			return errors.New("num of approval-node should not exceed 20")
		}
		if len(approval.DingTalkApproval.ApprovalNodes) == 0 {
			return errors.New("num of approval-node is 0")
		}
		for i, node := range approval.DingTalkApproval.ApprovalNodes {
			if len(node.ApproveUsers) == 0 {
				return errors.Errorf("num of approval-node %d approver is 0", i)
			}
			for _, user := range node.ApproveUsers {
				if userIDSets.Has(user.ID) {
					return errors.Errorf("Duplicate approvers %s should not appear in a complete approval process", user.Name)
				}
				userIDSets.Insert(user.ID)
			}
			if !lo.Contains([]string{"AND", "OR"}, string(node.Type)) {
				return errors.Errorf("approval-node %d type should be AND or OR", i)
			}
		}
	case config.WorkWXApproval:
		// TODO: add some linting here
		return nil
	default:
		return errors.Errorf("invalid approval type %s", approval.Type)
	}

	return nil
}
