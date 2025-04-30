/*
 * Copyright 2025 The KodeRover Authors.
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

package job

import (
	"fmt"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/types"
)

type OfflineServiceJobController struct {
	*BasicInfo

	jobSpec *commonmodels.OfflineServiceJobSpec
}

func CreateOfflineServiceJobController(job *commonmodels.Job, workflow *commonmodels.WorkflowV4) (Job, error) {
	spec := new(commonmodels.OfflineServiceJobSpec)
	if err := commonmodels.IToi(job.Spec, spec); err != nil {
		return nil, fmt.Errorf("failed to create apollo job controller, error: %s", err)
	}

	basicInfo := &BasicInfo{
		name:        job.Name,
		jobType:     job.JobType,
		errorPolicy: job.ErrorPolicy,
		workflow:    workflow,
	}

	return OfflineServiceJobController{
		BasicInfo: basicInfo,
		jobSpec:   spec,
	}, nil
}

func (j OfflineServiceJobController) SetWorkflow(wf *commonmodels.WorkflowV4) {
	j.workflow = wf
}

func (j OfflineServiceJobController) GetSpec() interface{} {
	return j.jobSpec
}

func (j OfflineServiceJobController) Validate(isExecution bool) error {
	if err := util.CheckZadigProfessionalLicense(); err != nil {
		return e.ErrLicenseInvalid.AddDesc("")
	}

	return nil
}

func (j OfflineServiceJobController) Update(useUserInput bool, ticket *commonmodels.ApprovalTicket) error {
	currJob, err := j.workflow.FindJob(j.name, j.jobType)
	if err != nil {
		return err
	}

	currJobSpec := new(commonmodels.OfflineServiceJobSpec)
	if err := commonmodels.IToi(currJob.Spec, currJobSpec); err != nil {
		return fmt.Errorf("failed to decode apollo job spec, error: %s", err)
	}

	j.jobSpec.EnvType = currJobSpec.EnvType
	if currJobSpec.Source == "fixed" {
		if j.jobSpec.EnvName != currJobSpec.EnvName {
			j.ClearSelection()
		}
		j.jobSpec.EnvName = currJobSpec.EnvName
	}
	j.jobSpec.Source = currJobSpec.Source

	return nil
}

func (j OfflineServiceJobController) SetOptions(ticket *commonmodels.ApprovalTicket) error {
	return nil
}

func (j OfflineServiceJobController) ClearOptions() {
	return
}

func (j OfflineServiceJobController) ClearSelection() {
	j.jobSpec.Services = make([]string, 0)
}

func (j OfflineServiceJobController) ToTask(taskID int64) ([]*commonmodels.JobTask, error) {
	resp := make([]*commonmodels.JobTask, 0)
	jobTask := &commonmodels.JobTask{
		Name:        GenJobName(j.workflow, j.name, 0),
		Key:         genJobKey(j.name),
		DisplayName: genJobDisplayName(j.name),
		OriginName:  j.name,
		JobType:     string(config.JobOfflineService),
		JobInfo: map[string]string{
			JobNameKey: j.name,
		},
		Spec: &commonmodels.JobTaskOfflineServiceSpec{
			EnvType: j.jobSpec.EnvType,
			EnvName: j.jobSpec.EnvName,
			ServiceEvents: func() (resp []*commonmodels.JobTaskOfflineServiceEvent) {
				for _, serviceName := range j.jobSpec.Services {
					resp = append(resp, &commonmodels.JobTaskOfflineServiceEvent{
						ServiceName: serviceName,
					})
				}
				return
			}(),
		},
		Timeout:     0,
		ErrorPolicy: j.errorPolicy,
	}

	resp = append(resp, jobTask)

	return resp, nil
}

func (j OfflineServiceJobController) SetRepo(repo *types.Repository) error {
	return nil
}

func (j OfflineServiceJobController) SetRepoCommitInfo() error {
	return nil
}

func (j OfflineServiceJobController) GetVariableList(jobName string, getAggregatedVariables, getRuntimeVariables, getPlaceHolderVariables, getServiceSpecificVariables, useUserInputValue bool) ([]*commonmodels.KeyVal, error) {
	return make([]*commonmodels.KeyVal, 0), nil
}

func (j OfflineServiceJobController) GetUsedRepos() ([]*types.Repository, error) {
	return make([]*types.Repository, 0), nil
}

func (j OfflineServiceJobController) RenderDynamicVariableOptions(key string, option *RenderDynamicVariableValue) ([]string, error) {
	return nil, fmt.Errorf("invalid job type: %s to render dynamic variable", j.name)
}

func (j OfflineServiceJobController) IsServiceTypeJob() bool {
	return false
}
