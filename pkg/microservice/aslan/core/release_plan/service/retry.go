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
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/workflow/service/workflow"
	"github.com/koderover/zadig/v2/pkg/shared/client/user"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

type ReleaseJobRetryer interface {
	Retry(plan *models.ReleasePlan) error
}

type RetryReleaseJobContext struct {
	AuthResources *user.AuthorizedResources
	UserID        string
	Account       string
	UserName      string
}

func NewReleaseJobRetryer(c *RetryReleaseJobContext, args *RetryReleaseJobArgs) (ReleaseJobRetryer, error) {
	switch config.ReleasePlanJobType(args.Type) {
	case config.JobText:
		return nil, errors.Errorf("text release job does not support retry")
	case config.JobWorkflow:
		return NewWorkflowReleaseJobRetryer(c, args)
	default:
		return nil, errors.Errorf("invalid release job type: %s", args.Type)
	}
}

type WorkflowReleaseJobRetryer struct {
	ID  string
	Ctx *RetryReleaseJobContext
}

func NewWorkflowReleaseJobRetryer(c *RetryReleaseJobContext, args *RetryReleaseJobArgs) (ReleaseJobRetryer, error) {
	var retryer WorkflowReleaseJobRetryer
	retryer.ID = args.ID
	retryer.Ctx = c
	return &retryer, nil
}

func (e *WorkflowReleaseJobRetryer) Retry(plan *models.ReleasePlan) error {
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
		if spec.TaskID == 0 {
			return errors.Errorf("workflow task not found")
		}

		err := jobManagerAuth(plan.Name, plan.ManagerID, job, e.Ctx.UserName, e.Ctx.UserID, e.Ctx.AuthResources)
		if err != nil {
			return err
		}

		// workflow support retry after failed
		if job.Status != config.ReleasePlanJobStatusFailed {
			return errors.Errorf("job %s status %s can't retry", job.Name, job.Status)
		}

		err = workflow.RetryWorkflowTaskV4(spec.Workflow.Name, spec.TaskID, log.SugaredLogger().With("source", "release plan"))
		if err != nil {
			return errors.Wrapf(err, "failed to retry workflow task %s", spec.Workflow.Name)
		}

		spec.Status = config.StatusPrepare
		job.Status = config.ReleasePlanJobStatusRunning
		job.ExecutedBy = e.Ctx.UserName
		job.ExecutedTime = time.Now().Unix()
		return nil
	}
	return errors.Errorf("job %s not found", e.ID)
}
