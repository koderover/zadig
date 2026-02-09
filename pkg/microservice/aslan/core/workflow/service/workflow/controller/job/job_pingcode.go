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
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/types"
)

type PingCodeJobController struct {
	*BasicInfo

	jobSpec *commonmodels.PingCodeJobSpec
}

func CreatePingCodeJobController(job *commonmodels.Job, workflow *commonmodels.WorkflowV4) (Job, error) {
	spec := new(commonmodels.PingCodeJobSpec)
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

	return PingCodeJobController{
		BasicInfo: basicInfo,
		jobSpec:   spec,
	}, nil
}

func (j PingCodeJobController) SetWorkflow(wf *commonmodels.WorkflowV4) {
	j.workflow = wf
}

func (j PingCodeJobController) GetSpec() interface{} {
	return j.jobSpec
}

func (j PingCodeJobController) Validate(isExecution bool) error {
	if err := util.CheckZadigEnterpriseLicense(); err != nil {
		return e.ErrLicenseInvalid.AddDesc("")
	}

	return nil
}

func (j PingCodeJobController) Update(useUserInput bool, ticket *commonmodels.ApprovalTicket) error {
	currJob, err := j.workflow.FindJob(j.name, j.jobType)
	if err != nil {
		return err
	}

	currJobSpec := new(commonmodels.PingCodeJobSpec)
	if err := commonmodels.IToi(currJob.Spec, currJobSpec); err != nil {
		return fmt.Errorf("failed to decode apollo job spec, error: %s", err)
	}
	j.errorPolicy = currJob.ErrorPolicy
	j.executePolicy = currJob.ExecutePolicy

	j.jobSpec.PingCodeID = currJobSpec.PingCodeID
	j.jobSpec.ProjectID = currJobSpec.ProjectID
	j.jobSpec.BoardID = currJobSpec.BoardID
	j.jobSpec.SprintID = currJobSpec.SprintID
	j.jobSpec.WorkItemTypeID = currJobSpec.WorkItemTypeID
	j.jobSpec.WorkItemStateIDs = currJobSpec.WorkItemStateIDs

	return nil
}

func (j PingCodeJobController) SetOptions(ticket *commonmodels.ApprovalTicket) error {
	return nil
}

func (j PingCodeJobController) ClearOptions() {
}

func (j PingCodeJobController) ClearSelection() {
}

func (j PingCodeJobController) ToTask(taskID int64) ([]*commonmodels.JobTask, error) {
	resp := make([]*commonmodels.JobTask, 0)

	spec, err := mongodb.NewProjectManagementColl().GetPingCodeSpec(j.jobSpec.PingCodeID)
	if err != nil {
		return nil, fmt.Errorf("failed to get pingcode info, error: %s", err)
	}

	jobTask := &commonmodels.JobTask{
		Name:        GenJobName(j.workflow, j.name, 0),
		Key:         genJobKey(j.name),
		DisplayName: genJobDisplayName(j.name),
		OriginName:  j.name,
		JobInfo: map[string]string{
			JobNameKey: j.name,
		},
		JobType: string(config.JobPingCode),
		Spec: commonmodels.JobTaskPingCodeSpec{
			PingCodeAddress:      spec.PingCodeAddress,
			PingCodeClientID:     spec.PingCodeClientID,
			PingCodeClientSecret: spec.PingCodeClientSecret,
			ProjectID:            j.jobSpec.ProjectID,
			ProjectName:          j.jobSpec.ProjectName,
			BoardID:              j.jobSpec.BoardID,
			SprintID:             j.jobSpec.SprintID,
			WorkItemTypeID:       j.jobSpec.WorkItemTypeID,
			WorkItemStateIDs:     j.jobSpec.WorkItemStateIDs,
			WorkItems:            j.jobSpec.WorkItems,
		},
		ErrorPolicy:   j.errorPolicy,
		ExecutePolicy: j.executePolicy,
	}
	resp = append(resp, jobTask)

	return resp, nil
}

func (j PingCodeJobController) SetRepo(repo *types.Repository) error {
	return nil
}

func (j PingCodeJobController) SetRepoCommitInfo() error {
	return nil
}

func (j PingCodeJobController) GetVariableList(jobName string, getAggregatedVariables, getRuntimeVariables, getPlaceHolderVariables, getServiceSpecificVariables, useUserInputValue bool) ([]*commonmodels.KeyVal, error) {
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

func (j PingCodeJobController) GetUsedRepos() ([]*types.Repository, error) {
	return make([]*types.Repository, 0), nil
}

func (j PingCodeJobController) RenderDynamicVariableOptions(key string, option *RenderDynamicVariableValue) ([]string, error) {
	return nil, fmt.Errorf("invalid job type: %s to render dynamic variable", j.name)
}

func (j PingCodeJobController) IsServiceTypeJob() bool {
	return false
}
