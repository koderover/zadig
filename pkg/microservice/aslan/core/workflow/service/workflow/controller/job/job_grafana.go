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
)

type GrafanaJobController struct {
	*BasicInfo

	jobSpec *commonmodels.GrafanaJobSpec
}

func CreateGrafanaJobJobController(job *commonmodels.Job, workflow *commonmodels.WorkflowV4) (Job, error) {
	spec := new(commonmodels.GrafanaJobSpec)
	if err := commonmodels.IToi(job.Spec, spec); err != nil {
		return nil, fmt.Errorf("failed to create apollo job controller, error: %s", err)
	}

	basicInfo := &BasicInfo{
		name:        job.Name,
		jobType:     job.JobType,
		errorPolicy: job.ErrorPolicy,
		workflow:    workflow,
	}

	return GrafanaJobController{
		BasicInfo: basicInfo,
		jobSpec:   spec,
	}, nil
}

func (j GrafanaJobController) SetWorkflow(wf *commonmodels.WorkflowV4) {
	j.workflow = wf
}

func (j GrafanaJobController) GetSpec() interface{} {
	return j.jobSpec
}

func (j GrafanaJobController) Validate(isExecution bool) error {
	if err := util.CheckZadigProfessionalLicense(); err != nil {
		return e.ErrLicenseInvalid.AddDesc("")
	}

	return nil
}

func (j GrafanaJobController) Update(useUserInput bool, ticket *commonmodels.ApprovalTicket) error {
	currJob, err := j.workflow.FindJob(j.name, j.jobType)
	if err != nil {
		return err
	}

	currJobSpec := new(commonmodels.GrafanaJobSpec)
	if err := commonmodels.IToi(currJob.Spec, currJobSpec); err != nil {
		return fmt.Errorf("failed to decode apollo job spec, error: %s", err)
	}

	j.jobSpec.ID = currJobSpec.ID
	j.jobSpec.Name = currJobSpec.Name
	j.jobSpec.CheckTime = currJobSpec.CheckTime
	j.jobSpec.CheckMode = currJobSpec.CheckMode
	userConfiguredAlerts := make(map[string]*commonmodels.GrafanaAlert)
	for _, userAlert := range j.jobSpec.Alerts {
		userConfiguredAlerts[userAlert.ID] = userAlert
	}
	mergedAlerts := make([]*commonmodels.GrafanaAlert, 0)
	for _, alert := range currJobSpec.Alerts {
		if userConfiguredAlert, ok := userConfiguredAlerts[alert.ID]; ok {
			mergedAlerts = append(mergedAlerts, userConfiguredAlert)
		}
	}
	j.jobSpec.Alerts = mergedAlerts

	return nil
}

func (j GrafanaJobController) SetOptions(ticket *commonmodels.ApprovalTicket) error {
	return nil
}

func (j GrafanaJobController) ClearOptions() {
	return
}

func (j GrafanaJobController) ClearSelection() {
	return
}

func (j GrafanaJobController) ToTask(taskID int64) ([]*commonmodels.JobTask, error) {
	resp := make([]*commonmodels.JobTask, 0)

	if len(j.jobSpec.Alerts) == 0 {
		return nil, fmt.Errorf("no alert")
	}
	for _, alert := range j.jobSpec.Alerts {
		alert.Status = "checking"
	}

	jobTask := &commonmodels.JobTask{
		Name:        GenJobName(j.workflow, j.name, 0),
		Key:         genJobKey(j.name),
		DisplayName: genJobDisplayName(j.name),
		OriginName:  j.name,
		JobInfo: map[string]string{
			JobNameKey: j.name,
		},
		JobType: string(config.JobGrafana),
		Spec: &commonmodels.JobTaskGrafanaSpec{
			ID:        j.jobSpec.ID,
			Name:      j.jobSpec.Name,
			CheckTime: j.jobSpec.CheckTime,
			CheckMode: j.jobSpec.CheckMode,
			Alerts:    j.jobSpec.Alerts,
		},
		ErrorPolicy: j.errorPolicy,
		Timeout:     0,
	}

	resp = append(resp, jobTask)
	return resp, nil
}

func (j GrafanaJobController) SetRepo(repo *types.Repository) error {
	return nil
}

func (j GrafanaJobController) SetRepoCommitInfo() error {
	return nil
}

func (j GrafanaJobController) GetVariableList(jobName string, getAggregatedVariables, getRuntimeVariables, getPlaceHolderVariables, getServiceSpecificVariables, getReferredKeyValVariables bool) ([]*commonmodels.KeyVal, error) {
	return make([]*commonmodels.KeyVal, 0), nil
}

func (j GrafanaJobController) GetUsedRepos() ([]*types.Repository, error) {
	return make([]*types.Repository, 0), nil
}
