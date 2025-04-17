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
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
)

type GuanceyunCheckJobController struct {
	*BasicInfo

	jobSpec *commonmodels.GuanceyunCheckJobSpec
}

func CreateGuanceyunCheckJobController(job *commonmodels.Job, workflow *commonmodels.WorkflowV4) (Job, error) {
	spec := new(commonmodels.GuanceyunCheckJobSpec)
	if err := commonmodels.IToi(job.Spec, spec); err != nil {
		return nil, fmt.Errorf("failed to create apollo job controller, error: %s", err)
	}

	basicInfo := &BasicInfo{
		name:        job.Name,
		jobType:     job.JobType,
		errorPolicy: job.ErrorPolicy,
		workflow:    workflow,
	}

	return GuanceyunCheckJobController{
		BasicInfo: basicInfo,
		jobSpec:   spec,
	}, nil
}

func (j GuanceyunCheckJobController) SetWorkflow(wf *commonmodels.WorkflowV4) {
	j.workflow = wf
}

func (j GuanceyunCheckJobController) GetSpec() interface{} {
	return j.jobSpec
}

func (j GuanceyunCheckJobController) Validate(isExecution bool) error {
	if err := util.CheckZadigProfessionalLicense(); err != nil {
		return e.ErrLicenseInvalid.AddDesc("")
	}

	if j.jobSpec.CheckTime <= 0 {
		return fmt.Errorf("check time must be greater than 0")
	}
	if len(j.jobSpec.Monitors) == 0 {
		return fmt.Errorf("num of check monitor must be greater than 0")
	}
	switch j.jobSpec.CheckMode {
	case "monitor", "trigger":
	default:
		return fmt.Errorf("invalid failed policy")
	}

	return nil
}

func (j GuanceyunCheckJobController) Update(useUserInput bool, ticket *commonmodels.ApprovalTicket) error {
	currJob, err := j.workflow.FindJob(j.name, j.jobType)
	if err != nil {
		return err
	}

	currJobSpec := new(commonmodels.GuanceyunCheckJobSpec)
	if err := commonmodels.IToi(currJob.Spec, currJobSpec); err != nil {
		return fmt.Errorf("failed to decode apollo job spec, error: %s", err)
	}

	j.jobSpec.ID = currJobSpec.ID
	j.jobSpec.Name = currJobSpec.Name
	j.jobSpec.CheckTime = currJobSpec.CheckTime
	j.jobSpec.CheckMode = currJobSpec.CheckMode
	j.jobSpec.Monitors = currJobSpec.Monitors
	return nil
}

func (j GuanceyunCheckJobController) SetOptions(ticket *commonmodels.ApprovalTicket) error {
	return nil
}

func (j GuanceyunCheckJobController) ClearOptions() {
	return
}

func (j GuanceyunCheckJobController) ClearSelection() {
	return
}

func (j GuanceyunCheckJobController) ToTask(taskID int64) ([]*commonmodels.JobTask, error) {
	resp := make([]*commonmodels.JobTask, 0)

	nameSet := sets.NewString()
	for _, monitor := range j.jobSpec.Monitors {
		if nameSet.Has(monitor.Name) {
			return nil, fmt.Errorf("duplicate monitor name %s", monitor.Name)
		}
		nameSet.Insert(monitor.Name)
		monitor.Status = "checking"
	}

	jobTask := &commonmodels.JobTask{
		Name:        GenJobName(j.workflow, j.name, 0),
		Key:         genJobKey(j.name),
		DisplayName: genJobDisplayName(j.name),
		OriginName:  j.name,
		JobInfo: map[string]string{
			JobNameKey: j.name,
		},
		JobType: string(config.JobGuanceyunCheck),
		Spec: &commonmodels.JobTaskGuanceyunCheckSpec{
			ID:        j.jobSpec.ID,
			Name:      j.jobSpec.Name,
			CheckTime: j.jobSpec.CheckTime,
			CheckMode: j.jobSpec.CheckMode,
			Monitors:  j.jobSpec.Monitors,
		},
		ErrorPolicy: j.errorPolicy,
	}

	resp = append(resp, jobTask)

	return resp, nil
}

func (j GuanceyunCheckJobController) SetRepo(repo *types.Repository) error {
	return nil
}

func (j GuanceyunCheckJobController) SetRepoCommitInfo() error {
	return nil
}

func (j GuanceyunCheckJobController) GetVariableList(jobName string, getAggregatedVariables, getRuntimeVariables, getPlaceHolderVariables, getServiceSpecificVariables, getReferredKeyValVariables bool) ([]*commonmodels.KeyVal, error) {
	return make([]*commonmodels.KeyVal, 0), nil
}

func (j GuanceyunCheckJobController) GetUsedRepos() ([]*types.Repository, error) {
	return make([]*types.Repository, 0), nil
}

func (j GuanceyunCheckJobController) RenderDynamicVariableOptions(key string, option *RenderDynamicVariableValue) ([]string, error) {
	return nil, fmt.Errorf("invalid job type: %s to render dynamic variable", j.name)
}
