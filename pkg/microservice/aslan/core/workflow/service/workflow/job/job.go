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

package job

import (
	"fmt"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
)

type JobCtl interface {
	Instantiate() error
	SetPreset() error
	ToJobs(taskID int64) ([]*commonmodels.JobTask, error)
}

func initJobCtl(job *commonmodels.Job, workflow *commonmodels.WorkflowV4) (JobCtl, error) {
	var resp JobCtl
	switch job.JobType {
	case config.JobZadigBuild:
		resp = &BuildJob{job: job, workflow: workflow}

	case config.JobZadigDeploy:
		resp = &DeployJob{job: job, workflow: workflow}
	default:
		return resp, fmt.Errorf("job type not found %s", job.JobType)
	}
	return resp, nil
}

func Instantiate(job *commonmodels.Job, workflow *commonmodels.WorkflowV4) error {
	ctl, err := initJobCtl(job, workflow)
	if err != nil {
		return err
	}
	return ctl.Instantiate()
}

func SetPreset(job *commonmodels.Job, workflow *commonmodels.WorkflowV4) error {
	jobCtl, err := initJobCtl(job, workflow)
	if err != nil {
		return err
	}
	return jobCtl.SetPreset()
}

func ToJobs(job *commonmodels.Job, workflow *commonmodels.WorkflowV4, taskID int64) ([]*commonmodels.JobTask, error) {
	jobCtl, err := initJobCtl(job, workflow)
	if err != nil {
		return []*commonmodels.JobTask{}, err
	}
	return jobCtl.ToJobs(taskID)
}

func jobNameFormat(jobName string) string {
	if len(jobName) > 50 {
		return jobName[:50]
	}
	return jobName
}
