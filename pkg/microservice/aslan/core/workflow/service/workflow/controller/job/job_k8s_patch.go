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
	"github.com/koderover/zadig/v2/pkg/setting"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/types"
)

type K8sPatchJobController struct {
	*BasicInfo

	jobSpec *commonmodels.K8sPatchJobSpec
}

func CreateK8sPatchJobController(job *commonmodels.Job, workflow *commonmodels.WorkflowV4) (Job, error) {
	spec := new(commonmodels.K8sPatchJobSpec)
	if err := commonmodels.IToi(job.Spec, spec); err != nil {
		return nil, fmt.Errorf("failed to create apollo job controller, error: %s", err)
	}

	basicInfo := &BasicInfo{
		name:        job.Name,
		jobType:     job.JobType,
		errorPolicy: job.ErrorPolicy,
		workflow:    workflow,
	}

	return K8sPatchJobController{
		BasicInfo: basicInfo,
		jobSpec:   spec,
	}, nil
}

func (j K8sPatchJobController) SetWorkflow(wf *commonmodels.WorkflowV4) {
	j.workflow = wf
}

func (j K8sPatchJobController) GetSpec() interface{} {
	return j.jobSpec
}

func (j K8sPatchJobController) Validate(isExecution bool) error {
	if err := util.CheckZadigProfessionalLicense(); err != nil {
		return e.ErrLicenseInvalid.AddDesc("")
	}

	return nil
}

func (j K8sPatchJobController) Update(useUserInput bool, ticket *commonmodels.ApprovalTicket) error {
	currJob, err := j.workflow.FindJob(j.name, j.jobType)
	if err != nil {
		return err
	}

	currJobSpec := new(commonmodels.K8sPatchJobSpec)
	if err := commonmodels.IToi(currJob.Spec, currJobSpec); err != nil {
		return fmt.Errorf("failed to decode apollo job spec, error: %s", err)
	}

	j.jobSpec.ClusterID = currJobSpec.ClusterID
	j.jobSpec.ClusterSource = currJobSpec.ClusterSource
	j.jobSpec.Namespace = currJobSpec.Namespace
	j.jobSpec.PatchItemOptions = currJobSpec.PatchItemOptions
	return nil
}

func (j K8sPatchJobController) SetOptions(ticket *commonmodels.ApprovalTicket) error {
	return nil
}

func (j K8sPatchJobController) ClearOptions() {
	return
}

func (j K8sPatchJobController) ClearSelection() {
	j.jobSpec.PatchItems = make([]*commonmodels.PatchItem, 0)
}

func (j K8sPatchJobController) ToTask(taskID int64) ([]*commonmodels.JobTask, error) {
	resp := make([]*commonmodels.JobTask, 0)

	spec := &commonmodels.JobTasK8sPatchSpec{
		ClusterID: j.jobSpec.ClusterID,
		Namespace: j.jobSpec.Namespace,
	}
	for _, patch := range j.jobSpec.PatchItems {
		patchTaskItem := &commonmodels.PatchTaskItem{
			ResourceName:    patch.ResourceName,
			ResourceKind:    patch.ResourceKind,
			ResourceGroup:   patch.ResourceGroup,
			ResourceVersion: patch.ResourceVersion,
			PatchContent:    renderString(patch.PatchContent, setting.RenderValueTemplate, patch.Params),
			PatchStrategy:   patch.PatchStrategy,
			Params:          patch.Params,
		}
		spec.PatchItems = append(spec.PatchItems, patchTaskItem)
	}

	jobTask := &commonmodels.JobTask{
		Name:        GenJobName(j.workflow, j.name, 0),
		Key:         genJobKey(j.name),
		DisplayName: genJobDisplayName(j.name),
		OriginName:  j.name,
		JobInfo: map[string]string{
			JobNameKey: j.name,
		},
		JobType:     string(config.JobK8sPatch),
		Spec:        spec,
		ErrorPolicy: j.errorPolicy,
	}

	resp = append(resp, jobTask)
	return resp, nil
}

func (j K8sPatchJobController) SetRepo(repo *types.Repository) error {
	return nil
}

func (j K8sPatchJobController) SetRepoCommitInfo() error {
	return nil
}

func (j K8sPatchJobController) GetVariableList(jobName string, getAggregatedVariables, getRuntimeVariables, getPlaceHolderVariables, getServiceSpecificVariables, getReferredKeyValVariables bool) ([]*commonmodels.KeyVal, error) {
	return make([]*commonmodels.KeyVal, 0), nil
}

func (j K8sPatchJobController) GetUsedRepos() ([]*types.Repository, error) {
	return make([]*types.Repository, 0), nil
}

func (j K8sPatchJobController) RenderDynamicVariableOptions(key string, option *RenderDynamicVariableValue) ([]string, error) {
	return nil, fmt.Errorf("invalid job type: %s to render dynamic variable", j.name)
}
