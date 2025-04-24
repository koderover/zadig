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
	"strconv"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/types"
)

type BlueKingJobController struct {
	*BasicInfo

	jobSpec *commonmodels.BlueKingJobSpec
}

func CreateBlueKingJobController(job *commonmodels.Job, workflow *commonmodels.WorkflowV4) (Job, error) {
	spec := new(commonmodels.BlueKingJobSpec)
	if err := commonmodels.IToi(job.Spec, spec); err != nil {
		return nil, fmt.Errorf("failed to create apollo job controller, error: %s", err)
	}

	basicInfo := &BasicInfo{
		name:        job.Name,
		jobType:     job.JobType,
		errorPolicy: job.ErrorPolicy,
		workflow:    workflow,
	}

	return BlueKingJobController{
		BasicInfo: basicInfo,
		jobSpec:   spec,
	}, nil
}

func (j BlueKingJobController) SetWorkflow(wf *commonmodels.WorkflowV4) {
	j.workflow = wf
}

func (j BlueKingJobController) GetSpec() interface{} {
	return j.jobSpec
}

func (j BlueKingJobController) Validate(isExecution bool) error {
	if err := util.CheckZadigProfessionalLicense(); err != nil {
		return e.ErrLicenseInvalid.AddDesc("")
	}

	currJob, err := j.workflow.FindJob(j.name, j.jobType)
	if err != nil {
		return err
	}

	currJobSpec := new(commonmodels.BlueKingJobSpec)
	if err := commonmodels.IToi(currJob.Spec, currJobSpec); err != nil {
		return fmt.Errorf("failed to decode blue king job spec, error: %s", err)
	}

	if _, err := mongodb.NewCICDToolColl().Get(j.jobSpec.ToolID); err != nil {
		return fmt.Errorf("not found blue king system: %s in mongo, err: %v", j.jobSpec.ToolID, err)
	}

	return nil
}

func (j BlueKingJobController) Update(useUserInput bool, ticket *commonmodels.ApprovalTicket) error {
	currJob, err := j.workflow.FindJob(j.name, j.jobType)
	if err != nil {
		return err
	}

	currJobSpec := new(commonmodels.BlueKingJobSpec)
	if err := commonmodels.IToi(currJob.Spec, currJobSpec); err != nil {
		return fmt.Errorf("failed to decode blue king job spec, error: %s", err)
	}

	j.jobSpec.ToolID = currJobSpec.ToolID
	if currJobSpec.BusinessID != j.jobSpec.BusinessID {
		j.jobSpec.ExecutionPlanID = 0
	}
	j.jobSpec.BusinessID = currJobSpec.BusinessID

	return nil
}

// SetOptions sets the actual kv for each configured apollo namespace for users to select and edit
func (j BlueKingJobController) SetOptions(ticket *commonmodels.ApprovalTicket) error {
	return nil
}

// ClearOptions does nothing since the option field happens to be the user configured field, clear it would cause problems
func (j BlueKingJobController) ClearOptions() {
	return
}

func (j BlueKingJobController) ClearSelection() {
	j.jobSpec.ExecutionPlanID = 0
	return
}

func (j BlueKingJobController) ToTask(taskID int64) ([]*commonmodels.JobTask, error) {
	if err := j.Validate(true); err != nil {
		return nil, err
	}

	resp := make([]*commonmodels.JobTask, 0)
	resp = append(resp, &commonmodels.JobTask{
		Name:        GenJobName(j.workflow, j.name, 0),
		Key:         genJobKey(j.name, strconv.FormatInt(j.jobSpec.ExecutionPlanID, 10)),
		DisplayName: genJobDisplayName(j.name),
		OriginName:  j.name,
		JobInfo: map[string]string{
			JobNameKey:        j.name,
			"blueking_job_id": strconv.FormatInt(j.jobSpec.ExecutionPlanID, 10),
		},
		JobType: string(config.JobBlueKing),
		Spec: &commonmodels.JobTaskBlueKingSpec{
			ToolID:          j.jobSpec.ToolID,
			BusinessID:      j.jobSpec.BusinessID,
			ExecutionPlanID: j.jobSpec.ExecutionPlanID,
			Parameters:      j.jobSpec.Parameters,
		},
		Timeout:     0,
		ErrorPolicy: j.errorPolicy,
	})

	return resp, nil
}

func (j BlueKingJobController) SetRepo(repo *types.Repository) error {
	return nil
}

func (j BlueKingJobController) SetRepoCommitInfo() error {
	return nil
}

func (j BlueKingJobController) GetVariableList(jobName string, getAggregatedVariables, getRuntimeVariables, getPlaceHolderVariables, getServiceSpecificVariables, getReferredKeyValVariables bool) ([]*commonmodels.KeyVal, error) {
	return make([]*commonmodels.KeyVal, 0), nil
}

func (j BlueKingJobController) GetUsedRepos() ([]*types.Repository, error) {
	return make([]*types.Repository, 0), nil
}

func (j BlueKingJobController) RenderDynamicVariableOptions(key string, option *RenderDynamicVariableValue) ([]string, error) {
	return nil, fmt.Errorf("invalid job type: %s to render dynamic variable", j.name)
}

func (j BlueKingJobController) IsServiceTypeJob() bool {
	return false
}
