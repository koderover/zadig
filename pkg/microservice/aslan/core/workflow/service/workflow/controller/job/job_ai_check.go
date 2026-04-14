/*
Copyright 2026 The KodeRover Authors.

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
	"github.com/koderover/zadig/v2/pkg/types"
)

type AICheckJobController struct {
	*BasicInfo

	jobSpec *commonmodels.AICheckJobSpec
}

func CreateAICheckJobController(job *commonmodels.Job, workflow *commonmodels.WorkflowV4) (Job, error) {
	spec := new(commonmodels.AICheckJobSpec)
	if err := commonmodels.IToi(job.Spec, spec); err != nil {
		return nil, fmt.Errorf("failed to create ai-check job controller, error: %s", err)
	}

	basicInfo := &BasicInfo{
		name:          job.Name,
		jobType:       job.JobType,
		errorPolicy:   job.ErrorPolicy,
		executePolicy: job.ExecutePolicy,
		workflow:      workflow,
	}

	return AICheckJobController{
		BasicInfo: basicInfo,
		jobSpec:   spec,
	}, nil
}

func (j AICheckJobController) SetWorkflow(wf *commonmodels.WorkflowV4) {
	j.workflow = wf
}

func (j AICheckJobController) GetSpec() interface{} {
	return j.jobSpec
}

func (j AICheckJobController) Validate(isExecution bool) error {
	currJob, err := j.workflow.FindJob(j.name, j.jobType)
	if err != nil {
		return err
	}

	currJobSpec := new(commonmodels.AICheckJobSpec)
	if err := commonmodels.IToi(currJob.Spec, currJobSpec); err != nil {
		return fmt.Errorf("failed to decode ai-check job spec, error: %s", err)
	}

	return nil
}

func (j AICheckJobController) Update(useUserInput bool, ticket *commonmodels.ApprovalTicket) error {
	latestJob, err := j.workflow.FindJob(j.name, j.jobType)
	if err != nil {
		return err
	}

	latestJobSpec := new(commonmodels.AICheckJobSpec)
	if err := commonmodels.IToi(latestJob.Spec, latestJobSpec); err != nil {
		return fmt.Errorf("failed to decode ai-check job spec, error: %s", err)
	}

	j.errorPolicy = latestJob.ErrorPolicy
	j.executePolicy = latestJob.ExecutePolicy

	j.jobSpec.Timeout = latestJobSpec.Timeout
	j.jobSpec.CheckContent = latestJobSpec.CheckContent
	j.jobSpec.ManualConfirm = latestJobSpec.ManualConfirm
	j.jobSpec.ManualConfirmUsers = latestJobSpec.ManualConfirmUsers
	return nil
}

func (j AICheckJobController) SetOptions(ticket *commonmodels.ApprovalTicket) error {
	return nil
}

func (j AICheckJobController) ClearOptions() {}

func (j AICheckJobController) ClearSelection() {}

func (j AICheckJobController) ToTask(taskID int64) ([]*commonmodels.JobTask, error) {
	jobSpec := &commonmodels.JobTaskAICheckSpec{
		Timeout:            j.jobSpec.Timeout,
		CheckContent:       j.jobSpec.CheckContent,
		ManualConfirm:      j.jobSpec.ManualConfirm,
		ManualConfirmUsers: j.jobSpec.ManualConfirmUsers,
	}

	jobTask := &commonmodels.JobTask{
		Name:        GenJobName(j.workflow, j.name, 0),
		Key:         genJobKey(j.name),
		DisplayName: genJobDisplayName(j.name),
		OriginName:  j.name,
		JobInfo: map[string]string{
			JobNameKey: j.name,
		},
		JobType:       string(config.JobAICheck),
		Spec:          jobSpec,
		Timeout:       j.jobSpec.Timeout,
		ErrorPolicy:   j.errorPolicy,
		ExecutePolicy: j.executePolicy,
	}

	return []*commonmodels.JobTask{jobTask}, nil
}

func (j AICheckJobController) LintJob() error {
	return nil
}

func (j AICheckJobController) SetRepo(repo *types.Repository) error {
	return nil
}

func (j AICheckJobController) SetRepoCommitInfo() error {
	return nil
}

func (j AICheckJobController) GetVariableList(jobName string, getAggregatedVariables, getRuntimeVariables, getPlaceHolderVariables, getServiceSpecificVariables, useUserInputValue bool) ([]*commonmodels.KeyVal, error) {
	resp := make([]*commonmodels.KeyVal, 0)
	if getRuntimeVariables {
		resp = append(resp, &commonmodels.KeyVal{
			Key:          strings.Join([]string{"job", j.name, "status"}, "."),
			Value:        "",
			Type:         "string",
			IsCredential: false,
		})
	}
	return resp, nil
}

func (j AICheckJobController) GetUsedRepos() ([]*types.Repository, error) {
	return make([]*types.Repository, 0), nil
}

func (j AICheckJobController) RenderDynamicVariableOptions(key string, option *RenderDynamicVariableValue) ([]string, error) {
	return nil, fmt.Errorf("invalid job type: %s to render dynamic variable", j.name)
}

func (j AICheckJobController) IsServiceTypeJob() bool {
	return false
}
