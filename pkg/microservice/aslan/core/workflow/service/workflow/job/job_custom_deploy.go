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
	"strings"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/tool/log"
)

type CustomDeployJob struct {
	job      *commonmodels.Job
	workflow *commonmodels.WorkflowV4
	spec     *commonmodels.CustomDeployJobSpec
}

func (j *CustomDeployJob) Instantiate() error {
	j.spec = &commonmodels.CustomDeployJobSpec{}
	if err := commonmodels.IToiYaml(j.job.Spec, j.spec); err != nil {
		return err
	}
	j.job.Spec = j.spec
	return nil
}

func (j *CustomDeployJob) SetPreset() error {
	j.spec = &commonmodels.CustomDeployJobSpec{}
	if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
		return err
	}
	j.job.Spec = j.spec
	return nil
}

func (j *CustomDeployJob) MergeArgs(args *commonmodels.Job) error {
	if j.job.Name == args.Name && j.job.JobType == args.JobType {
		j.spec = &commonmodels.CustomDeployJobSpec{}
		if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
			return err
		}
		j.job.Spec = j.spec
		argsSpec := &commonmodels.CustomDeployJobSpec{}
		if err := commonmodels.IToi(args.Spec, argsSpec); err != nil {
			return err
		}
		j.spec.Source = argsSpec.Source
		j.spec.Targets = argsSpec.Targets
		j.job.Spec = j.spec
	}
	return nil
}

func (j *CustomDeployJob) ToJobs(taskID int64) ([]*commonmodels.JobTask, error) {
	resp := []*commonmodels.JobTask{}

	j.spec = &commonmodels.CustomDeployJobSpec{}
	if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
		return resp, err
	}
	for _, target := range j.spec.Targets {
		t := strings.Split(target.Target, "/")
		if len(t) != 3 {
			log.Errorf("target string: %s wrong format", target.Target)
			continue
		}
		workloadType := t[0]
		workloadName := t[1]
		containerName := t[2]
		jobTaskSpec := &commonmodels.JobTaskCustomDeploySpec{
			Namespace:          j.spec.Namespace,
			ClusterID:          j.spec.ClusterID,
			Timeout:            j.spec.Timeout,
			WorkloadType:       workloadType,
			WorkloadName:       workloadName,
			ContainerName:      containerName,
			Image:              target.Image,
			SkipCheckRunStatus: j.spec.SkipCheckRunStatus,
		}
		jobTask := &commonmodels.JobTask{
			Name:    jobNameFormat(j.job.Name + "-" + workloadType + "-" + workloadName + "-" + containerName),
			JobType: string(config.JobCustomDeploy),
			Spec:    jobTaskSpec,
		}
		resp = append(resp, jobTask)
	}
	j.job.Spec = j.spec
	return resp, nil
}

func (j *CustomDeployJob) LintJob() error {
	return nil
}
