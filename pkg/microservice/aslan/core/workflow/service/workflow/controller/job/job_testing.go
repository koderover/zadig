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
	"fmt"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/types"
)

type TestingJobController struct {
	*BasicInfo

	jobSpec *commonmodels.ZadigTestingJobSpec
}

func CreateTestingJobController(job *commonmodels.Job, workflow *commonmodels.WorkflowV4) (Job, error) {
	spec := new(commonmodels.ZadigTestingJobSpec)
	if err := commonmodels.IToi(job.Spec, spec); err != nil {
		return nil, fmt.Errorf("failed to create apollo job controller, error: %s", err)
	}

	basicInfo := &BasicInfo{
		name:        job.Name,
		jobType:     job.JobType,
		errorPolicy: job.ErrorPolicy,
		workflow:    workflow,
	}

	return TestingJobController{
		BasicInfo: basicInfo,
		jobSpec:   spec,
	}, nil
}

func (j TestingJobController) SetWorkflow(wf *commonmodels.WorkflowV4) {
	j.workflow = wf
}

func (j TestingJobController) GetSpec() interface{} {
	return j.jobSpec
}

func (j TestingJobController) Validate(isExecution bool) error {
	if j.jobSpec.Source != config.SourceFromJob {
		return nil
	}
	jobRankMap := GetJobRankMap(j.workflow.Stages)
	buildJobRank, ok := jobRankMap[j.jobSpec.JobName]
	if !ok || buildJobRank >= jobRankMap[j.name] {
		return fmt.Errorf("can not quote job %s in job %s", j.jobSpec.JobName, j.name)
	}
	return nil
}

func (j TestingJobController) Update(useUserInput bool, ticket *commonmodels.ApprovalTicket) error {
	currJob, err := j.workflow.FindJob(j.name, j.jobType)
	if err != nil {
		return err
	}

	currJobSpec := new(commonmodels.ZadigTestingJobSpec)
	if err := commonmodels.IToi(currJob.Spec, currJobSpec); err != nil {
		return fmt.Errorf("failed to decode apollo job spec, error: %s", err)
	}

	j.jobSpec.TestType = currJobSpec.TestType
	j.jobSpec.Source = currJobSpec.Source
	j.jobSpec.JobName = currJobSpec.JobName
	j.jobSpec.OriginJobName = currJobSpec.OriginJobName
	j.jobSpec.RefRepos = currJobSpec.RefRepos

	return nil
}

func (j TestingJobController) SetOptions(ticket *commonmodels.ApprovalTicket) error {
	return nil
}

func (j TestingJobController) ClearOptions() {
	return
}

func (j TestingJobController) ClearSelection() {
	return
}

func (j TestingJobController) ToTask(taskID int64) ([]*commonmodels.JobTask, error) {
	resp := make([]*commonmodels.JobTask, 0)

	return resp, nil
}

func (j TestingJobController) SetRepo(repo *types.Repository) error {
	return nil
}

func (j TestingJobController) SetRepoCommitInfo() error {
	return nil
}

func (j TestingJobController) GetVariableList(jobName string, getAggregatedVariables, getRuntimeVariables, getPlaceHolderVariables, getServiceSpecificVariables, getReferredKeyValVariables bool) ([]*commonmodels.KeyVal, error) {
	return make([]*commonmodels.KeyVal, 0), nil
}

func (j TestingJobController) GetUsedRepos() ([]*types.Repository, error) {
	return make([]*types.Repository, 0), nil
}
