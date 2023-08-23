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
)

const ()

type ReleaseJobExecutor interface {
	Execute(plan *models.ReleasePlan) error
}

func NewReleaseJobExecutor(username string, args *ExecuteReleaseJobArgs) (ReleaseJobExecutor, error) {
	switch config.ReleasePlanJobType(args.Type) {
	case config.JobText:
		return NewTextReleaseJobExecutor(username, args)
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

func NewTextReleaseJobExecutor(username string, args *ExecuteReleaseJobArgs) (ReleaseJobExecutor, error) {
	var executor TextReleaseJobExecutor
	if err := models.IToi(args.Spec, &executor.Spec); err != nil {
		return nil, errors.Wrap(err, "invalid spec")
	}
	executor.ID = args.ID
	executor.ExecutedBy = username
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
			return errors.Errorf("job %s status is not todo", e.ID)
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
	ID         string
	ExecutedBy string
	Spec       WorkflowReleaseJobSpec
}

type WorkflowReleaseJobSpec struct {
	TaskID int64 `json:"task_id"`
}

func NewWorkflowReleaseJobExecutor(username string, args *ExecuteReleaseJobArgs) (ReleaseJobExecutor, error) {
	var executor WorkflowReleaseJobExecutor
	if err := models.IToi(args.Spec, &executor.Spec); err != nil {
		return nil, errors.Wrap(err, "invalid spec")
	}
	executor.ID = args.ID
	executor.ExecutedBy = username
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
		if job.Status != config.ReleasePlanJobStatusTodo {
			return errors.Errorf("job %s status is not todo", e.ID)
		}
		spec.TaskID = e.Spec.TaskID
		job.Spec = spec
		job.Status = config.ReleasePlanJobStatusRunning
		return nil
	}
	return errors.Errorf("job %s not found", e.ID)
}
