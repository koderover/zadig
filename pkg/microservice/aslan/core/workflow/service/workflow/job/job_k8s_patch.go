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
	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/setting"
)

type K8sPacthJob struct {
	job      *commonmodels.Job
	workflow *commonmodels.WorkflowV4
	spec     *commonmodels.K8sPatchJobSpec
}

func (j *K8sPacthJob) Instantiate() error {
	j.spec = &commonmodels.K8sPatchJobSpec{}
	if err := commonmodels.IToiYaml(j.job.Spec, j.spec); err != nil {
		return err
	}
	j.job.Spec = j.spec
	return nil
}

func (j *K8sPacthJob) SetPreset() error {
	j.spec = &commonmodels.K8sPatchJobSpec{}
	if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
		return err
	}
	j.job.Spec = j.spec
	return nil
}

func (j *K8sPacthJob) MergeArgs(args *commonmodels.Job) error {
	if j.job.Name == args.Name && j.job.JobType == args.JobType {
		j.spec = &commonmodels.K8sPatchJobSpec{}
		if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
			return err
		}
		j.job.Spec = j.spec
		argsSpec := &commonmodels.K8sPatchJobSpec{}
		if err := commonmodels.IToi(args.Spec, argsSpec); err != nil {
			return err
		}
		j.spec.PatchItems = argsSpec.PatchItems
		j.job.Spec = j.spec
	}
	return nil
}

func (j *K8sPacthJob) ToJobs(taskID int64) ([]*commonmodels.JobTask, error) {
	// logger := log.SugaredLogger()
	resp := []*commonmodels.JobTask{}
	j.spec = &commonmodels.K8sPatchJobSpec{}
	if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
		return resp, err
	}
	jobTask := &commonmodels.JobTask{
		Name:    j.job.Name,
		JobType: string(config.JobK8sPatch),
		Spec:    patchJobToTaskJob(j.spec),
	}
	resp = append(resp, jobTask)
	j.job.Spec = j.spec
	return resp, nil
}

func (j *K8sPacthJob) LintJob() error {
	return nil
}

func patchJobToTaskJob(job *commonmodels.K8sPatchJobSpec) *commonmodels.JobTasK8sPatchSpec {
	resp := &commonmodels.JobTasK8sPatchSpec{
		ClusterID: job.ClusterID,
		Namespace: job.Namespace,
	}
	for _, patch := range job.PatchItems {
		patchTaskItem := &commonmodels.PatchTaskItem{
			ResourceName:    patch.ResourceName,
			ResourceKind:    patch.ResourceKind,
			ResourceGroup:   patch.ResourceGroup,
			ResourceVersion: patch.ResourceVersion,
			PatchContent:    renderString(patch.PatchContent, setting.RenderValueTemplate, patch.Params),
			PatchStrategy:   patch.PatchStrategy,
			Params:          patch.Params,
		}
		resp.PatchItems = append(resp.PatchItems, patchTaskItem)
	}
	return resp
}
