/*
Copyright 2024 The KodeRover Authors.

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

package job

import (
	"fmt"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

type ApprovalJob struct {
	job      *commonmodels.Job
	workflow *commonmodels.WorkflowV4
	spec     *commonmodels.ApprovalJobSpec
}

func (j *ApprovalJob) Instantiate() error {
	j.spec = &commonmodels.ApprovalJobSpec{}
	if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
		return err
	}
	j.job.Spec = j.spec
	return nil
}

func (j *ApprovalJob) SetPreset() error {
	j.spec = &commonmodels.ApprovalJobSpec{}
	if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
		return err
	}
	j.job.Spec = j.spec
	return nil
}

func (j *ApprovalJob) SetOptions() error {
	return nil
}

func (j *ApprovalJob) ClearSelectionField() error {
	return nil
}

func (j *ApprovalJob) UpdateWithLatestSetting() error {
	j.spec = &commonmodels.ApprovalJobSpec{}
	if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
		return err
	}

	latestWorkflow, err := mongodb.NewWorkflowV4Coll().Find(j.workflow.Name)
	if err != nil {
		log.Errorf("Failed to find original workflow to set options, error: %s", err)
		return err
	}

	latestSpec := new(commonmodels.ApprovalJobSpec)
	found := false
	for _, stage := range latestWorkflow.Stages {
		if !found {
			for _, job := range stage.Jobs {
				if job.Name == j.job.Name && job.JobType == j.job.JobType {
					if err := commonmodels.IToi(job.Spec, latestSpec); err != nil {
						return err
					}
					found = true
					break
				}
			}
		} else {
			break
		}
	}

	if !found {
		return fmt.Errorf("failed to find the original workflow: %s", j.workflow.Name)
	}
	// just use the latest config
	j.spec = latestSpec

	j.job.Spec = j.spec
	return nil
}

func (j *ApprovalJob) MergeArgs(args *commonmodels.Job) error {
	j.spec = &commonmodels.ApprovalJobSpec{}
	if err := commonmodels.IToi(args.Spec, j.spec); err != nil {
		return err
	}
	j.job.Spec = j.spec
	return nil
}

func (j *ApprovalJob) ToJobs(taskID int64) ([]*commonmodels.JobTask, error) {
	j.spec = &commonmodels.ApprovalJobSpec{}
	if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
		return nil, err
	}

	resp := make([]*commonmodels.JobTask, 0)
	resp = append(resp, &commonmodels.JobTask{
		Name: j.job.Name,
		JobInfo: map[string]string{
			JobNameKey: j.job.Name,
		},
		Key:     j.job.Name,
		JobType: string(config.JobApproval),
		Spec: &commonmodels.JobTaskApprovalSpec{
			Type:             j.spec.Type,
			Description:      j.spec.Description,
			NativeApproval:   j.spec.NativeApproval,
			LarkApproval:     j.spec.LarkApproval,
			DingTalkApproval: j.spec.DingTalkApproval,
			WorkWXApproval:   j.spec.WorkWXApproval,
		},
		Timeout: j.spec.Timeout,
	})

	return resp, nil
}

func (j *ApprovalJob) LintJob() error {
	return nil
}
