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
	"strings"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/types"
)

type CustomDeployJobController struct {
	*BasicInfo

	jobSpec *commonmodels.CustomDeployJobSpec
}

func CreateCustomDeployJobController(job *commonmodels.Job, workflow *commonmodels.WorkflowV4) (Job, error) {
	spec := new(commonmodels.CustomDeployJobSpec)
	if err := commonmodels.IToi(job.Spec, spec); err != nil {
		return nil, fmt.Errorf("failed to create apollo job controller, error: %s", err)
	}

	basicInfo := &BasicInfo{
		name:        job.Name,
		jobType:     job.JobType,
		errorPolicy: job.ErrorPolicy,
		workflow:    workflow,
	}

	return CustomDeployJobController{
		BasicInfo: basicInfo,
		jobSpec:   spec,
	}, nil
}

func (j CustomDeployJobController) SetWorkflow(wf *commonmodels.WorkflowV4) {
	j.workflow = wf
}

func (j CustomDeployJobController) GetSpec() interface{} {
	return j.jobSpec
}

func (j CustomDeployJobController) Validate(isExecution bool) error {
	return nil
}

func (j CustomDeployJobController) Update(useUserInput bool, ticket *commonmodels.ApprovalTicket) error {
	currJob, err := j.workflow.FindJob(j.name, j.jobType)
	if err != nil {
		return err
	}

	currJobSpec := new(commonmodels.CustomDeployJobSpec)
	if err := commonmodels.IToi(currJob.Spec, currJobSpec); err != nil {
		return fmt.Errorf("failed to decode custom deploy job spec, error: %s", err)
	}

	j.jobSpec.ClusterID = currJobSpec.ClusterID
	j.jobSpec.Namespace = currJobSpec.Namespace
	j.jobSpec.SkipCheckRunStatus = currJobSpec.SkipCheckRunStatus
	j.jobSpec.Timeout = currJobSpec.Timeout
	j.jobSpec.DockerRegistryID = currJobSpec.DockerRegistryID
	j.jobSpec.TargetOptions = currJobSpec.TargetOptions

	return nil
}

func (j CustomDeployJobController) SetOptions(ticket *commonmodels.ApprovalTicket) error {
	return nil
}

// ClearOptions does nothing since options is a configuration
func (j CustomDeployJobController) ClearOptions() {
	return
}

func (j CustomDeployJobController) ClearSelection() {
	j.jobSpec.Targets = make([]*commonmodels.DeployTargets, 0)
}

func (j CustomDeployJobController) ToTask(taskID int64) ([]*commonmodels.JobTask, error) {
	resp := make([]*commonmodels.JobTask, 0)

	for jobSubTaskID, target := range j.jobSpec.Targets {
		t := strings.Split(target.Target, "/")
		if len(t) != 3 {
			log.Errorf("target string: %s wrong format", target.Target)
			continue
		}
		workloadType := t[0]
		workloadName := t[1]
		containerName := t[2]
		jobTaskSpec := &commonmodels.JobTaskCustomDeploySpec{
			Namespace:          j.jobSpec.Namespace,
			ClusterID:          j.jobSpec.ClusterID,
			Timeout:            j.jobSpec.Timeout,
			WorkloadType:       workloadType,
			WorkloadName:       workloadName,
			ContainerName:      containerName,
			Image:              target.Image,
			SkipCheckRunStatus: j.jobSpec.SkipCheckRunStatus,
		}
		jobTask := &commonmodels.JobTask{
			Name:        GenJobName(j.workflow, j.name, jobSubTaskID),
			Key:         genJobKey(j.name, workloadType, workloadName, containerName),
			DisplayName: genJobDisplayName(j.name, workloadType, workloadName, containerName),
			OriginName:  j.name,
			JobInfo: map[string]string{
				JobNameKey:       j.name,
				"workload_type":  workloadType,
				"workload_name":  workloadName,
				"container_name": containerName,
			},
			JobType:     string(config.JobCustomDeploy),
			Spec:        jobTaskSpec,
			ErrorPolicy: j.errorPolicy,
		}
		resp = append(resp, jobTask)
	}

	return resp, nil
}

func (j CustomDeployJobController) SetRepo(repo *types.Repository) error {
	return nil
}

func (j CustomDeployJobController) SetRepoCommitInfo() error {
	return nil
}

func (j CustomDeployJobController) GetVariableList(jobName string, getAggregatedVariables, getRuntimeVariables, getPlaceHolderVariables, getServiceSpecificVariables, useUserInputValue bool) ([]*commonmodels.KeyVal, error) {
	return make([]*commonmodels.KeyVal, 0), nil
}

func (j CustomDeployJobController) GetUsedRepos() ([]*types.Repository, error) {
	return make([]*types.Repository, 0), nil
}

func (j CustomDeployJobController) RenderDynamicVariableOptions(key string, option *RenderDynamicVariableValue) ([]string, error) {
	return nil, fmt.Errorf("invalid job type: %s to render dynamic variable", j.name)
}

func (j CustomDeployJobController) IsServiceTypeJob() bool {
	return false
}
