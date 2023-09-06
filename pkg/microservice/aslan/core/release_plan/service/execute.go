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
	"time"

	"github.com/pkg/errors"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/workflow/service/workflow"
	"github.com/koderover/zadig/pkg/shared/handler"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	"github.com/koderover/zadig/pkg/types"
)

type ReleaseJobExecutor interface {
	Execute(plan *models.ReleasePlan) error
}

func NewReleaseJobExecutor(c *handler.Context, args *ExecuteReleaseJobArgs) (ReleaseJobExecutor, error) {
	switch config.ReleasePlanJobType(args.Type) {
	case config.JobText:
		return NewTextReleaseJobExecutor(c.UserName, args)
	case config.JobWorkflow:
		return NewWorkflowReleaseJobExecutor(username, args)
	default:
		return nil, errors.Errorf("invalid release job type: %s", args.Type)
	}
}

type TextReleaseJobExecutor struct {
	ID         string
	ExecutedBy string
	Spec       TextReleaseJobSpec
}

type TextReleaseJobSpec struct {
	Remark string `json:"remark"`
}

func NewTextReleaseJobExecutor(c *handler.Context, args *ExecuteReleaseJobArgs) (ReleaseJobExecutor, error) {
	var executor TextReleaseJobExecutor
	if err := models.IToi(args.Spec, &executor.Spec); err != nil {
		return nil, errors.Wrap(err, "invalid spec")
	}
	executor.ID = args.ID
	executor.ExecutedBy = c.UserName
	return &executor, nil
}

func (e *TextReleaseJobExecutor) Execute(plan *models.ReleasePlan) error {
	spec := new(models.TextReleaseJobSpec)
	for _, job := range plan.Jobs {
		if job.ID != e.ID {
			continue
		}
		if err := models.IToi(job.Spec, spec); err != nil {
			return errors.Wrap(err, "invalid spec")
		}
		if job.Status != config.ReleasePlanJobStatusTodo {
			return errors.Errorf("job %s status is not todo", job.Name)
		}
		spec.Remark = e.Spec.Remark
		job.Spec = spec
		job.Status = config.ReleasePlanJobStatusDone
		job.ExecutedBy = e.ExecutedBy
		job.ExecutedTime = time.Now().Unix()
		return nil
	}
	return errors.Errorf("job %s not found", e.ID)
}

type WorkflowReleaseJobExecutor struct {
	ID   string
	Ctx  *handler.Context
	Spec WorkflowReleaseJobSpec
}

type WorkflowReleaseJobSpec struct {
}

func NewWorkflowReleaseJobExecutor(c *handler.Context, args *ExecuteReleaseJobArgs) (ReleaseJobExecutor, error) {
	var executor WorkflowReleaseJobExecutor
	if err := models.IToi(args.Spec, &executor.Spec); err != nil {
		return nil, errors.Wrap(err, "invalid spec")
	}
	executor.ID = args.ID
	executor.Ctx = c
	return &executor, nil
}

func (e *WorkflowReleaseJobExecutor) Execute(plan *models.ReleasePlan) error {
	spec := new(models.WorkflowReleaseJobSpec)
	for _, job := range plan.Jobs {
		if job.ID != e.ID {
			continue
		}
		if err := models.IToi(job.Spec, spec); err != nil {
			return errors.Wrap(err, "invalid spec")
		}
		if spec.Workflow == nil {
			return errors.Errorf("workflow is nil")
		}
		// workflow support retry after failed
		if job.Status != config.ReleasePlanJobStatusTodo && job.Status != config.ReleasePlanJobStatusFailed {
			return errors.Errorf("job %s status %s can't execute", job.Name, job.Status)
		}
		// check workflow execute permission
		ctx := e.Ctx
		if !ctx.Resources.IsSystemAdmin {
			if _, ok := ctx.Resources.ProjectAuthInfo[spec.Workflow.Project]; !ok {
				return ErrPermissionDenied
			}

			if !ctx.Resources.ProjectAuthInfo[spec.Workflow.Project].IsProjectAdmin &&
				!ctx.Resources.ProjectAuthInfo[spec.Workflow.Project].Workflow.Execute {
				// check if the permission is given by collaboration mode
				permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, spec.Workflow.Project, types.ResourceTypeWorkflow, spec.Workflow.Name, types.WorkflowActionRun)
				if err != nil || !permitted {
					return ErrPermissionDenied
				}
			}
		}

		result, err := workflow.CreateWorkflowTaskV4(&workflow.CreateWorkflowTaskV4Args{
			Name:    ctx.UserName,
			Account: ctx.Account,
			UserID:  ctx.UserID,
		}, spec.Workflow, ctx.Logger)
		if err != nil {
			return errors.Wrapf(err, "failed to create workflow task %s", spec.Workflow.Name)
		}

		spec.TaskID = result.TaskID
		spec.Status = config.StatusPrepare
		job.Spec = spec
		job.Status = config.ReleasePlanJobStatusRunning
		job.ExecutedBy = ctx.UserName
		job.ExecutedTime = time.Now().Unix()
		return nil
	}
	return errors.Errorf("job %s not found", e.ID)
}
