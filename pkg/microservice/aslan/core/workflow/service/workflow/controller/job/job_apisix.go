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
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/types"
)

type ApisixJobController struct {
	*BasicInfo

	jobSpec *commonmodels.ApisixJobSpec
}

func CreateApisixJobController(job *commonmodels.Job, workflow *commonmodels.WorkflowV4) (Job, error) {
	spec := new(commonmodels.ApisixJobSpec)
	if err := commonmodels.IToi(job.Spec, spec); err != nil {
		return nil, fmt.Errorf("failed to create apisix job controller, error: %s", err)
	}

	basicInfo := &BasicInfo{
		name:          job.Name,
		jobType:       job.JobType,
		errorPolicy:   job.ErrorPolicy,
		executePolicy: job.ExecutePolicy,
		workflow:      workflow,
	}

	return ApisixJobController{
		BasicInfo: basicInfo,
		jobSpec:   spec,
	}, nil
}

func (j ApisixJobController) SetWorkflow(wf *commonmodels.WorkflowV4) {
	j.workflow = wf
}

func (j ApisixJobController) GetSpec() interface{} {
	return j.jobSpec
}

func (j ApisixJobController) Validate(isExecution bool) error {
	if err := util.CheckZadigProfessionalLicense(); err != nil {
		return e.ErrLicenseInvalid.AddDesc("")
	}

	currJob, err := j.workflow.FindJob(j.name, j.jobType)
	if err != nil {
		return err
	}

	currJobSpec := new(commonmodels.ApisixJobSpec)
	if err := commonmodels.IToi(currJob.Spec, currJobSpec); err != nil {
		return fmt.Errorf("failed to decode apollo job spec, error: %s", err)
	}

	if j.jobSpec.ApisixID != currJobSpec.ApisixID {
		return fmt.Errorf("given apisix job spec (id: %s) does not match current apisix job (id: %s)", j.jobSpec.ApisixID, currJobSpec.ApisixID)
	}

	return nil
}

func (j ApisixJobController) Update(useUserInput bool, ticket *commonmodels.ApprovalTicket) error {
	return nil
}

func (j ApisixJobController) SetOptions(ticket *commonmodels.ApprovalTicket) error {
	return nil
}

func (j ApisixJobController) ClearOptions() {
}

func (j ApisixJobController) ClearSelection() {
	j.jobSpec.Tasks = make([]*commonmodels.ApisixItemSpec, 0)
}

func (j ApisixJobController) ToTask(taskID int64) ([]*commonmodels.JobTask, error) {
	resp := make([]*commonmodels.JobTask, 0)

	itemList := make([]*commonmodels.ApisixItemUpdateSpec, 0)
	for _, updateItem := range j.jobSpec.Tasks {
		itemList = append(itemList, &commonmodels.ApisixItemUpdateSpec{
			Action: updateItem.Action,
			Type: updateItem.Type,
			UserSpec: updateItem.Spec,
		})
	}

	jobTask := &commonmodels.JobTask{
		Name:        GenJobName(j.workflow, j.name, 0),
		Key:         genJobKey(j.name),
		DisplayName: genJobDisplayName(j.name),
		OriginName:  j.name,
		JobInfo: map[string]string{
			JobNameKey: j.name,
		},
		JobType: string(config.JobApisix),
		Spec: commonmodels.JobTaskApisixSpec{
			ApisixID: j.jobSpec.ApisixID,
			Tasks:    itemList,
		},
		ErrorPolicy:   j.errorPolicy,
		ExecutePolicy: j.executePolicy,
	}
	resp = append(resp, jobTask)

	return resp, nil
}

func (j ApisixJobController) SetRepo(repo *types.Repository) error {
	return nil
}

func (j ApisixJobController) SetRepoCommitInfo() error {
	return nil
}

func (j ApisixJobController) GetVariableList(jobName string, getAggregatedVariables, getRuntimeVariables, getPlaceHolderVariables, getServiceSpecificVariables, useUserInputValue bool) ([]*commonmodels.KeyVal, error) {
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

func (j ApisixJobController) GetUsedRepos() ([]*types.Repository, error) {
	return make([]*types.Repository, 0), nil
}

func (j ApisixJobController) RenderDynamicVariableOptions(key string, option *RenderDynamicVariableValue) ([]string, error) {
	return nil, fmt.Errorf("invalid job type: %s to render dynamic variable", j.name)
}

func (j ApisixJobController) IsServiceTypeJob() bool {
	return false
}
