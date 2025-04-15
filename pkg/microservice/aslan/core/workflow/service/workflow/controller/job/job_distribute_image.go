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

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/types"
)

type DistributeImageJobController struct {
	*BasicInfo

	jobSpec *commonmodels.ZadigDistributeImageJobSpec
}

func CreateDistributeImageJobController(job *commonmodels.Job, workflow *commonmodels.WorkflowV4) (Job, error) {
	spec := new(commonmodels.ZadigDistributeImageJobSpec)
	if err := commonmodels.IToi(job.Spec, spec); err != nil {
		return nil, fmt.Errorf("failed to create apollo job controller, error: %s", err)
	}

	basicInfo := &BasicInfo{
		name:        job.Name,
		jobType:     job.JobType,
		errorPolicy: job.ErrorPolicy,
		workflow:    workflow,
	}

	return DistributeImageJobController{
		BasicInfo: basicInfo,
		jobSpec:   spec,
	}, nil
}

func (j DistributeImageJobController) SetWorkflow(wf *commonmodels.WorkflowV4) {
	j.workflow = wf
}

func (j DistributeImageJobController) GetSpec() interface{} {
	return j.jobSpec
}

func (j DistributeImageJobController) Validate(isExecution bool) error {
	return nil
}

func (j DistributeImageJobController) Update(useUserInput bool, ticket *commonmodels.ApprovalTicket) error {

	return nil
}

func (j DistributeImageJobController) SetOptions(ticket *commonmodels.ApprovalTicket) error {
	return nil
}

func (j DistributeImageJobController) ClearOptions() {
	return
}

func (j DistributeImageJobController) ClearSelection() {
	return
}

func (j DistributeImageJobController) ToTask(taskID int64) ([]*commonmodels.JobTask, error) {

	return make([]*commonmodels.JobTask, 0), nil
}

func (j DistributeImageJobController) SetRepo(repo *types.Repository) error {
	return nil
}

func (j DistributeImageJobController) SetRepoCommitInfo() error {
	return nil
}

func (j DistributeImageJobController) GetVariableList(jobName string, getAggregatedVariables, getRuntimeVariables, getPlaceHolderVariables, getServiceSpecificVariables, getReferredKeyValVariables bool) ([]*commonmodels.KeyVal, error) {
	return make([]*commonmodels.KeyVal, 0), nil
}

func (j DistributeImageJobController) GetUsedRepos() ([]*types.Repository, error) {
	return make([]*types.Repository, 0), nil
}
