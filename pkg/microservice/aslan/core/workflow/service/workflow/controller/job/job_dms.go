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
	"github.com/koderover/zadig/v2/pkg/types"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

type DMSJobController struct {
	*BasicInfo

	jobSpec *commonmodels.DMSJobSpec
}

func CreateDMSJobController(job *commonmodels.Job, workflow *commonmodels.WorkflowV4) (Job, error) {
	spec := new(commonmodels.DMSJobSpec)
	if err := commonmodels.IToi(job.Spec, spec); err != nil {
		return nil, fmt.Errorf("failed to create dms job controller, error: %s", err)
	}

	basicInfo := &BasicInfo{
		name:          job.Name,
		jobType:       job.JobType,
		errorPolicy:   job.ErrorPolicy,
		executePolicy: job.ExecutePolicy,
		workflow:      workflow,
	}

	return DMSJobController{
		BasicInfo: basicInfo,
		jobSpec:   spec,
	}, nil
}

func (j DMSJobController) SetWorkflow(wf *commonmodels.WorkflowV4) {
	j.workflow = wf
}

func (j DMSJobController) GetSpec() interface{} {
	return j.jobSpec
}

func (j DMSJobController) Validate(isExecution bool) error {
	if err := util.CheckZadigProfessionalLicense(); err != nil {
		return e.ErrLicenseInvalid.AddDesc("")
	}

	currJob, err := j.workflow.FindJob(j.name, j.jobType)
	if err != nil {
		return err
	}

	currJobSpec := new(commonmodels.DMSJobSpec)
	if err := commonmodels.IToi(currJob.Spec, currJobSpec); err != nil {
		return fmt.Errorf("failed to decode apollo job spec, error: %s", err)
	}

	if j.jobSpec.ID != currJobSpec.ID {
		return fmt.Errorf("given apollo job spec does not match current apollo job")
	}

	if isExecution {
	}

	return nil
}

// Update does 2 things:
// 1. ALWAYS use the configured apollo system and options.
// 2. if there is a given selection and the configured system changed, clear it.
func (j DMSJobController) Update(useUserInput bool, ticket *commonmodels.ApprovalTicket) error {
	currJob, err := j.workflow.FindJob(j.name, j.jobType)
	if err != nil {
		return err
	}

	currJobSpec := new(commonmodels.DMSJobSpec)
	if err := commonmodels.IToi(currJob.Spec, currJobSpec); err != nil {
		return fmt.Errorf("failed to decode apollo job spec, error: %s", err)
	}

	if j.jobSpec.ID != currJobSpec.ID {
		j.jobSpec.Orders = make([]*commonmodels.DMSOrder, 0)
	}

	j.jobSpec.ID = currJobSpec.ID
	j.jobSpec.RemarkTemplate = currJobSpec.RemarkTemplate

	return nil
}

// SetOptions sets the actual kv for each configured apollo namespace for users to select and edit
func (j DMSJobController) SetOptions(ticket *commonmodels.ApprovalTicket) error {
	return nil
}

// ClearOptions does nothing since the option field happens to be the user configured field, clear it would cause problems
func (j DMSJobController) ClearOptions() {
	return
}

func (j DMSJobController) ClearSelection() {
	// j.jobSpec.NamespaceList = make([]*commonmodels.ApolloNamespace, 0)
}

func (j DMSJobController) ToTask(taskID int64) ([]*commonmodels.JobTask, error) {
	jobTask := &commonmodels.JobTask{
		Name:        GenJobName(j.workflow, j.name, 0),
		Key:         genJobKey(j.name),
		DisplayName: genJobDisplayName(j.name),
		OriginName:  j.name,
		JobInfo: map[string]string{
			JobNameKey: j.name,
		},
		JobType: string(config.JobDMS),
		Spec: &commonmodels.JobTaskDMSSpec{
			ID: j.jobSpec.ID,
			Orders: func() (list []*commonmodels.DMSTaskOrder) {
				for _, order := range j.jobSpec.Orders {
					list = append(list, &commonmodels.DMSTaskOrder{
						DMSOrder: *order,
					})
				}
				return list
			}(),
		},
		Timeout:       0,
		ErrorPolicy:   j.errorPolicy,
		ExecutePolicy: j.executePolicy,
	}

	return []*commonmodels.JobTask{jobTask}, nil
}

func (j DMSJobController) SetRepo(repo *types.Repository) error {
	return nil
}

func (j DMSJobController) SetRepoCommitInfo() error {
	return nil
}

func (j DMSJobController) GetVariableList(jobName string, getAggregatedVariables, getRuntimeVariables, getPlaceHolderVariables, getServiceSpecificVariables, useUserInputValue bool) ([]*commonmodels.KeyVal, error) {
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

func (j DMSJobController) GetUsedRepos() ([]*types.Repository, error) {
	return make([]*types.Repository, 0), nil
}

func (j DMSJobController) RenderDynamicVariableOptions(key string, option *RenderDynamicVariableValue) ([]string, error) {
	return nil, fmt.Errorf("invalid job type: %s to render dynamic variable", j.name)
}

func (j DMSJobController) IsServiceTypeJob() bool {
	return false
}
