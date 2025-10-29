/*
Copyright 2025 The KodeRover Authors.

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
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/types"
)

type CanaryReleaseJobController struct {
	*BasicInfo

	jobSpec *commonmodels.CanaryReleaseJobSpec
}

func CreateCanaryReleaseJobController(job *commonmodels.Job, workflow *commonmodels.WorkflowV4) (Job, error) {
	spec := new(commonmodels.CanaryReleaseJobSpec)
	if err := commonmodels.IToi(job.Spec, spec); err != nil {
		return nil, fmt.Errorf("failed to create apollo job controller, error: %s", err)
	}

	basicInfo := &BasicInfo{
		name:          job.Name,
		jobType:       job.JobType,
		errorPolicy:   job.ErrorPolicy,
		executePolicy: job.ExecutePolicy,
		workflow:      workflow,
	}

	return CanaryReleaseJobController{
		BasicInfo: basicInfo,
		jobSpec:   spec,
	}, nil
}

func (j CanaryReleaseJobController) SetWorkflow(wf *commonmodels.WorkflowV4) {
	j.workflow = wf
}

func (j CanaryReleaseJobController) GetSpec() interface{} {
	return j.jobSpec
}

func (j CanaryReleaseJobController) Validate(isExecution bool) error {
	if err := util.CheckZadigProfessionalLicense(); err != nil {
		return e.ErrLicenseInvalid.AddDesc("")
	}

	currJob, err := j.workflow.FindJob(j.name, j.jobType)
	if err != nil {
		return err
	}

	currJobSpec := new(commonmodels.CanaryReleaseJobSpec)
	if err := commonmodels.IToi(currJob.Spec, currJobSpec); err != nil {
		return fmt.Errorf("failed to decode apollo job spec, error: %s", err)
	}

	if isExecution {
		if j.jobSpec.FromJob != currJobSpec.FromJob {
			return fmt.Errorf("from job [%s] is different from configuration in the workflow: [%s]", j.jobSpec.FromJob, currJobSpec.FromJob)
		}
	}

	_, err = j.workflow.FindJob(j.jobSpec.FromJob, config.JobK8sCanaryDeploy)
	if err != nil {
		return fmt.Errorf("failed to find referred job: %s, error: %s", j.jobSpec.FromJob, err)
	}

	return nil
}

func (j CanaryReleaseJobController) Update(useUserInput bool, ticket *commonmodels.ApprovalTicket) error {
	currJob, err := j.workflow.FindJob(j.name, j.jobType)
	if err != nil {
		return err
	}

	currJobSpec := new(commonmodels.CanaryReleaseJobSpec)
	if err := commonmodels.IToi(currJob.Spec, currJobSpec); err != nil {
		return fmt.Errorf("failed to decode apollo job spec, error: %s", err)
	}

	j.jobSpec.FromJob = currJobSpec.FromJob
	j.jobSpec.ReleaseTimeout = currJobSpec.ReleaseTimeout

	return nil
}

func (j CanaryReleaseJobController) SetOptions(ticket *commonmodels.ApprovalTicket) error {
	return nil
}

func (j CanaryReleaseJobController) ClearOptions() {
	return
}

func (j CanaryReleaseJobController) ClearSelection() {
	return
}

func (j CanaryReleaseJobController) ToTask(taskID int64) ([]*commonmodels.JobTask, error) {
	resp := make([]*commonmodels.JobTask, 0)

	deployJob, err := j.workflow.FindJob(j.jobSpec.FromJob, config.JobK8sCanaryDeploy)
	if err != nil {
		return nil, err
	}

	deployJobSpec := &commonmodels.CanaryDeployJobSpec{}
	if err := commonmodels.IToi(deployJob.Spec, deployJobSpec); err != nil {
		return resp, err
	}

	for jobSubTaskID, target := range deployJobSpec.Targets {
		if target.WorkloadName == "" {
			continue
		}
		task := &commonmodels.JobTask{
			Name:        GenJobName(j.workflow, j.name, jobSubTaskID),
			Key:         genJobKey(j.name, target.K8sServiceName),
			DisplayName: genJobDisplayName(j.name, target.K8sServiceName),
			OriginName:  j.name,
			JobInfo: map[string]string{
				JobNameKey:         j.name,
				"k8s_service_name": target.K8sServiceName,
			},
			JobType: string(config.JobK8sCanaryRelease),
			Spec: &commonmodels.JobTaskCanaryReleaseSpec{
				Namespace:      deployJobSpec.Namespace,
				ClusterID:      deployJobSpec.ClusterID,
				ReleaseTimeout: j.jobSpec.ReleaseTimeout,
				K8sServiceName: target.K8sServiceName,
				WorkloadType:   target.WorkloadType,
				WorkloadName:   target.WorkloadName,
				ContainerName:  target.ContainerName,
				Image:          target.Image,
			},
			ErrorPolicy:   j.errorPolicy,
			ExecutePolicy: j.executePolicy,
		}
		resp = append(resp, task)
	}

	return resp, nil
}

func (j CanaryReleaseJobController) SetRepo(repo *types.Repository) error {
	return nil
}

func (j CanaryReleaseJobController) SetRepoCommitInfo() error {
	return nil
}

func (j CanaryReleaseJobController) GetVariableList(jobName string, getAggregatedVariables, getRuntimeVariables, getPlaceHolderVariables, getServiceSpecificVariables, useUserInputValue bool) ([]*commonmodels.KeyVal, error) {
	return make([]*commonmodels.KeyVal, 0), nil
}

func (j CanaryReleaseJobController) GetUsedRepos() ([]*types.Repository, error) {
	return make([]*types.Repository, 0), nil
}

func (j CanaryReleaseJobController) RenderDynamicVariableOptions(key string, option *RenderDynamicVariableValue) ([]string, error) {
	return nil, fmt.Errorf("invalid job type: %s to render dynamic variable", j.name)
}

func (j CanaryReleaseJobController) IsServiceTypeJob() bool {
	return false
}
