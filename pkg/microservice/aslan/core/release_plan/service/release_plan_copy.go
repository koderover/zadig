/*
 * Copyright 2026 The KodeRover Authors.
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
	"context"
	"strings"

	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/shared/handler"
	"github.com/koderover/zadig/v2/pkg/util"
)

type CopyReleasePlanArgs struct {
	Name string `json:"name"`
}

func CopyReleasePlan(c *handler.Context, planID string, args *CopyReleasePlanArgs) error {
	if args == nil {
		return errors.New("copy release plan args is nil")
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	sourcePlan, err := mongodb.NewReleasePlanColl().GetByID(ctx, planID)
	if err != nil {
		return errors.Wrap(err, "get source release plan")
	}

	copiedPlan, err := prepareCopiedReleasePlan(sourcePlan, args.Name)
	if err != nil {
		return errors.Wrap(err, "prepare copied release plan")
	}

	return CreateReleasePlan(c, copiedPlan)
}

func prepareCopiedReleasePlan(source *models.ReleasePlan, name string) (*models.ReleasePlan, error) {
	if source == nil {
		return nil, errors.New("source release plan is nil")
	}

	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("name is required")
	}

	copied := new(models.ReleasePlan)
	if err := util.DeepCopy(copied, source); err != nil {
		return nil, errors.Wrap(err, "deep copy release plan")
	}

	copied.ID = primitive.NilObjectID
	copied.Index = 0
	copied.Name = name
	copied.CreatedBy = ""
	copied.CreateTime = 0
	copied.UpdatedBy = ""
	copied.UpdateTime = 0
	copied.Status = ""
	copied.PlanningTime = 0
	copied.FinishPlanningTime = 0
	copied.ApprovalTime = 0
	copied.ExecutingTime = 0
	copied.SuccessTime = 0
	copied.InstanceCode = ""
	copied.WaitForFinishPlanningExternalCheckTime = 0
	copied.WaitForApproveExternalCheckTime = 0
	copied.WaitForExecuteExternalCheckTime = 0
	copied.WaitForAllDoneExternalCheckTime = 0
	copied.ExternalCheckFailedReason = ""
	copied.CallbackDescription = ""

	resetCopiedReleasePlanApproval(copied.Approval)
	for _, job := range copied.Jobs {
		if err := resetCopiedReleaseJob(job); err != nil {
			return nil, err
		}
	}

	return copied, nil
}

func resetCopiedReleaseJob(job *models.ReleaseJob) error {
	if job == nil {
		return nil
	}

	job.ID = ""
	job.ReleaseJobRuntime = models.ReleaseJobRuntime{}

	if job.Type != config.JobWorkflow {
		return nil
	}

	spec := new(models.WorkflowReleaseJobSpec)
	if err := models.IToi(job.Spec, spec); err != nil {
		return errors.Wrap(err, "convert workflow release job spec")
	}

	spec.TaskID = 0
	spec.Status = config.StatusPrepare
	job.Spec = spec

	return nil
}

func resetCopiedReleasePlanApproval(approval *models.Approval) {
	if approval == nil {
		return
	}

	approval.Status = ""
	approval.StartTime = 0
	approval.EndTime = 0

	if approval.NativeApproval != nil {
		approval.NativeApproval.RejectOrApprove = ""
		approval.NativeApproval.InstanceCode = ""
		for _, user := range approval.NativeApproval.ApproveUsers {
			resetApprovalUserRuntime(user)
		}
		approval.NativeApproval.FloatApproveUsers = nil
	}

	if approval.LarkApproval != nil {
		approval.LarkApproval.ApprovalInitiator = nil
		approval.LarkApproval.InstanceCode = ""
		approval.LarkApproval.ApprovalInstance = nil
		for _, user := range approval.LarkApproval.ApproveUsers {
			resetLarkApprovalUserRuntime(user)
		}
		for _, node := range approval.LarkApproval.ApprovalNodes {
			node.RejectOrApprove = ""
			for _, user := range node.ApproveUsers {
				resetLarkApprovalUserRuntime(user)
			}
		}
	}

	if approval.DingTalkApproval != nil {
		approval.DingTalkApproval.InstanceCode = ""
		for _, node := range approval.DingTalkApproval.ApprovalNodes {
			node.RejectOrApprove = ""
			for _, user := range node.ApproveUsers {
				user.RejectOrApprove = ""
				user.Comment = ""
				user.OperationTime = 0
			}
		}
	}

	if approval.WorkWXApproval != nil {
		approval.WorkWXApproval.CreatorUser = nil
		approval.WorkWXApproval.ApprovalNodeDetails = nil
		approval.WorkWXApproval.InstanceID = ""
	}
}

func resetApprovalUserRuntime(user *models.User) {
	if user == nil {
		return
	}

	user.RejectOrApprove = ""
	user.Comment = ""
	user.OperationTime = 0
}

func resetLarkApprovalUserRuntime(user *models.LarkApprovalUser) {
	if user == nil {
		return
	}

	user.RejectOrApprove = ""
	user.Comment = ""
	user.OperationTime = 0
}
