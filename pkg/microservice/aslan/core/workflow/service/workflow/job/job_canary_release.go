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

type CanaryReleaseJob struct {
	job      *commonmodels.Job
	workflow *commonmodels.WorkflowV4
	spec     *commonmodels.CanaryReleaseJobSpec
}

func (j *CanaryReleaseJob) Instantiate() error {
	j.spec = &commonmodels.CanaryReleaseJobSpec{}
	if err := commonmodels.IToiYaml(j.job.Spec, j.spec); err != nil {
		return err
	}
	j.job.Spec = j.spec
	return nil
}

func (j *CanaryReleaseJob) SetPreset() error {
	j.spec = &commonmodels.CanaryReleaseJobSpec{}
	if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
		return err
	}
	j.job.Spec = j.spec
	return nil
}

func (j *CanaryReleaseJob) MergeArgs(args *commonmodels.Job) error {
	return nil
}

func (j *CanaryReleaseJob) ToJobs(taskID int64) ([]*commonmodels.JobTask, error) {
	resp := []*commonmodels.JobTask{}

	j.spec = &commonmodels.CanaryReleaseJobSpec{}
	if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
		return resp, err
	}

	var releaseJobSpec *commonmodels.CanaryDeployJobSpec
	for _, stage := range j.workflow.Stages {
		for _, job := range stage.Jobs {
			if job.JobType != config.JobCanaryRelease || job.Name != j.spec.FromJob {
				continue
			}
			if err := commonmodels.IToi(job.Spec, releaseJobSpec); err != nil {
				return resp, err
			}
			break
		}
	}
	if releaseJobSpec == nil {
		return resp, fmt.Errorf("no canary release job: %s found, please check workflow configuration", j.spec.FromJob)
	}
	for _, target := range releaseJobSpec.Targets {
		task := &commonmodels.JobTask{
			Name:    jobNameFormat(j.job.Name + "-" + target.K8sServiceName),
			JobType: string(config.JobCanaryRelease),
			Spec: &commonmodels.JobTaskCanaryReleaseSpec{
				Namespace:      releaseJobSpec.Namespace,
				ClusterID:      releaseJobSpec.ClusterID,
				ReleaseTimeout: j.spec.ReleaseTimeout,
				K8sServiceName: target.K8sServiceName,
				WorkloadType:   target.WorkloadType,
				WorkloadName:   target.WorkloadName,
				ContainerName:  target.ContainerName,
				Image:          target.Image,
			},
		}
		resp = append(resp, task)
	}
	j.job.Spec = j.spec
	return resp, nil
}
