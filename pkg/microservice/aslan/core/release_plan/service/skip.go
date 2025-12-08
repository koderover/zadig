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

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/shared/client/user"
)

type ReleaseJobSkipper interface {
	Skip(plan *models.ReleasePlan) error
}

type SkipReleaseJobContext struct {
	AuthResources *user.AuthorizedResources
	UserID        string
	Account       string
	UserName      string
}

func NewReleaseJobSkipper(c *SkipReleaseJobContext, args *SkipReleaseJobArgs) (ReleaseJobSkipper, error) {
	switch config.ReleasePlanJobType(args.Type) {
	case config.JobText:
		return NewTextReleaseJobSkipper(c, args)
	case config.JobWorkflow:
		return NewWorkflowReleaseJobSkipper(c, args)
	default:
		return nil, errors.Errorf("invalid release job type: %s", args.Type)
	}
}

type WorkflowReleaseJobSkipper struct {
	ID   string
	Ctx  *SkipReleaseJobContext
	Spec WorkflowReleaseJobSpec
}

func NewWorkflowReleaseJobSkipper(c *SkipReleaseJobContext, args *SkipReleaseJobArgs) (ReleaseJobSkipper, error) {
	var skipper WorkflowReleaseJobSkipper
	if err := models.IToi(args.Spec, &skipper.Spec); err != nil {
		return nil, errors.Wrap(err, "invalid spec")
	}
	skipper.ID = args.ID
	skipper.Ctx = c
	return &skipper, nil
}

func (e *WorkflowReleaseJobSkipper) Skip(plan *models.ReleasePlan) error {
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
		if job.Status != config.ReleasePlanJobStatusTodo {
			return errors.Errorf("job %s status %s can't skip", job.Name, job.Status)
		}

		err := jobManagerAuth(plan.Name, plan.ManagerID, job, e.Ctx.UserName, e.Ctx.UserID, e.Ctx.AuthResources)
		if err != nil {
			return err
		}

		job.Status = config.ReleasePlanJobStatusSkipped
		job.ExecutedBy = e.Ctx.Account
		job.ExecutedTime = time.Now().Unix()
		return nil
	}
	return errors.Errorf("job %s not found", e.ID)
}

type TextReleaseJobSkipper struct {
	ID        string
	SkippedBy string
	Ctx       *SkipReleaseJobContext
	Spec      TextReleaseJobSpec
}

func NewTextReleaseJobSkipper(c *SkipReleaseJobContext, args *SkipReleaseJobArgs) (ReleaseJobSkipper, error) {
	var executor TextReleaseJobSkipper
	if err := models.IToi(args.Spec, &executor.Spec); err != nil {
		return nil, errors.Wrap(err, "invalid spec")
	}
	executor.ID = args.ID
	executor.SkippedBy = c.UserName
	executor.Ctx = c
	return &executor, nil
}

func (e *TextReleaseJobSkipper) Skip(plan *models.ReleasePlan) error {
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

		err := jobManagerAuth(plan.Name, plan.ManagerID, job, e.Ctx.UserName, e.Ctx.UserID, e.Ctx.AuthResources)
		if err != nil {
			return err
		}

		spec.Remark = e.Spec.Remark
		job.Spec = spec
		job.Status = config.ReleasePlanJobStatusSkipped
		job.ExecutedBy = e.SkippedBy
		job.ExecutedTime = time.Now().Unix()
		return nil
	}
	return errors.Errorf("job %s not found", e.ID)
}
