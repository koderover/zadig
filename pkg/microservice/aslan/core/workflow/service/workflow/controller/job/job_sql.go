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
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/types"
)

type SQLJobController struct {
	*BasicInfo

	jobSpec *commonmodels.SQLJobSpec
}

func CreateSQLJobController(job *commonmodels.Job, workflow *commonmodels.WorkflowV4) (Job, error) {
	spec := new(commonmodels.SQLJobSpec)
	if err := commonmodels.IToi(job.Spec, spec); err != nil {
		return nil, fmt.Errorf("failed to create apollo job controller, error: %s", err)
	}

	basicInfo := &BasicInfo{
		name:        job.Name,
		jobType:     job.JobType,
		errorPolicy: job.ErrorPolicy,
		workflow:    workflow,
	}

	return SQLJobController{
		BasicInfo: basicInfo,
		jobSpec:   spec,
	}, nil
}

func (j SQLJobController) SetWorkflow(wf *commonmodels.WorkflowV4) {
	j.workflow = wf
}

func (j SQLJobController) GetSpec() interface{} {
	return j.jobSpec
}

func (j SQLJobController) Validate(isExecution bool) error {
	if _, err := mongodb.NewDBInstanceColl().Find(&mongodb.DBInstanceCollFindOption{Id: j.jobSpec.ID}); err != nil {
		return fmt.Errorf("not found db instance in mongo, err: %v", err)
	}

	return nil
}

func (j SQLJobController) Update(useUserInput bool, ticket *commonmodels.ApprovalTicket) error {
	currJob, err := j.workflow.FindJob(j.name, j.jobType)
	if err != nil {
		return err
	}

	currJobSpec := new(commonmodels.SQLJobSpec)
	if err := commonmodels.IToi(currJob.Spec, currJobSpec); err != nil {
		return fmt.Errorf("failed to decode apollo job spec, error: %s", err)
	}

	if j.jobSpec.Source == "fixed" {
		j.jobSpec.ID = currJobSpec.ID
		j.jobSpec.Type = currJobSpec.Type
	}
	j.jobSpec.Source = currJobSpec.Source

	return nil
}

func (j SQLJobController) SetOptions(ticket *commonmodels.ApprovalTicket) error {
	return nil
}

func (j SQLJobController) ClearOptions() {
	return
}

func (j SQLJobController) ClearSelection() {
	return
}

func (j SQLJobController) ToTask(taskID int64) ([]*commonmodels.JobTask, error) {
	resp := make([]*commonmodels.JobTask, 0)

	jobTask := &commonmodels.JobTask{
		Key:         genJobKey(j.name),
		DisplayName: genJobDisplayName(j.name),
		Name:        GenJobName(j.workflow, j.name, 0),
		OriginName:  j.name,
		JobInfo: map[string]string{
			JobNameKey: j.name,
		},
		JobType: string(config.JobSQL),
		Spec: &commonmodels.JobTaskSQLSpec{
			ID:   j.jobSpec.ID,
			Type: j.jobSpec.Type,
			SQL:  j.jobSpec.SQL,
		},
		Timeout:     0,
		ErrorPolicy: j.errorPolicy,
	}
	resp = append(resp, jobTask)

	return resp, nil
}

func (j SQLJobController) SetRepo(repo *types.Repository) error {
	return nil
}

func (j SQLJobController) SetRepoCommitInfo() error {
	return nil
}

func (j SQLJobController) GetVariableList(jobName string, getAggregatedVariables, getRuntimeVariables, getPlaceHolderVariables, getServiceSpecificVariables, getReferredKeyValVariables bool) ([]*commonmodels.KeyVal, error) {
	return make([]*commonmodels.KeyVal, 0), nil
}

func (j SQLJobController) GetUsedRepos() ([]*types.Repository, error) {
	return make([]*types.Repository, 0), nil
}

func (j SQLJobController) RenderDynamicVariableOptions(key string, option *RenderDynamicVariableValue) ([]string, error) {
	return nil, fmt.Errorf("invalid job type: %s to render dynamic variable", j.name)
}

func (j SQLJobController) IsServiceTypeJob() bool {
	return false
}
