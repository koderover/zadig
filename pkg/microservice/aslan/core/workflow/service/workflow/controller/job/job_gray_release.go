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
	templaterepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb/template"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/types"
)

type GrayReleaseJobController struct {
	*BasicInfo

	jobSpec *commonmodels.GrayReleaseJobSpec
}

func CreateGrayReleaseJobController(job *commonmodels.Job, workflow *commonmodels.WorkflowV4) (Job, error) {
	spec := new(commonmodels.GrayReleaseJobSpec)
	if err := commonmodels.IToi(job.Spec, spec); err != nil {
		return nil, fmt.Errorf("failed to create apollo job controller, error: %s", err)
	}

	basicInfo := &BasicInfo{
		name:        job.Name,
		jobType:     job.JobType,
		errorPolicy: job.ErrorPolicy,
		workflow:    workflow,
	}

	return GrayReleaseJobController{
		BasicInfo: basicInfo,
		jobSpec:   spec,
	}, nil
}

func (j GrayReleaseJobController) SetWorkflow(wf *commonmodels.WorkflowV4) {
	j.workflow = wf
}

func (j GrayReleaseJobController) GetSpec() interface{} {
	return j.jobSpec
}

func (j GrayReleaseJobController) Validate(isExecution bool) error {
	if err := util.CheckZadigProfessionalLicense(); err != nil {
		return e.ErrLicenseInvalid.AddDesc("")
	}

	currJob, err := j.workflow.FindJob(j.name, j.jobType)
	if err != nil {
		return err
	}

	currJobSpec := new(commonmodels.BlueGreenReleaseV2JobSpec)
	if err := commonmodels.IToi(currJob.Spec, currJobSpec); err != nil {
		return fmt.Errorf("failed to decode apollo job spec, error: %s", err)
	}

	if isExecution {
		if j.jobSpec.FromJob != currJobSpec.FromJob {
			return fmt.Errorf("from job [%s] is different from configuration in the workflow: [%s]", j.jobSpec.FromJob, currJobSpec.FromJob)
		}
	}

	_, err = j.workflow.FindJob(j.jobSpec.FromJob, config.JobK8sBlueGreenDeploy)
	if err != nil {
		return fmt.Errorf("failed to find referred job: %s, error: %s", j.jobSpec.FromJob, err)
	}

	return nil
}

func (j GrayReleaseJobController) Update(useUserInput bool, ticket *commonmodels.ApprovalTicket) error {
	currJob, err := j.workflow.FindJob(j.name, j.jobType)
	if err != nil {
		return err
	}

	currJobSpec := new(commonmodels.BlueGreenReleaseV2JobSpec)
	if err := commonmodels.IToi(currJob.Spec, currJobSpec); err != nil {
		return fmt.Errorf("failed to decode apollo job spec, error: %s", err)
	}

	j.jobSpec.FromJob = currJobSpec.FromJob

	return nil
}

func (j GrayReleaseJobController) SetOptions(ticket *commonmodels.ApprovalTicket) error {
	return nil
}

func (j GrayReleaseJobController) ClearOptions() {
	return
}

func (j GrayReleaseJobController) ClearSelection() {
	return
}

func (j GrayReleaseJobController) ToTask(taskID int64) ([]*commonmodels.JobTask, error) {
	resp := make([]*commonmodels.JobTask, 0)

	deployJob, err := j.workflow.FindJob(j.jobSpec.FromJob, config.JobK8sBlueGreenDeploy)
	if err != nil {
		return nil, err
	}

	deployJobSpec := &commonmodels.BlueGreenDeployV2JobSpec{}
	if err := commonmodels.IToi(deployJob.Spec, deployJobSpec); err != nil {
		return resp, err
	}

	templateProduct, err := templaterepo.NewProductColl().Find(j.workflow.Project)
	if err != nil {
		return resp, fmt.Errorf("cannot find product %s: %w", j.workflow.Project, err)
	}
	timeout := templateProduct.Timeout * 60

	for jobSubTaskID, target := range deployJobSpec.Services {
		task := &commonmodels.JobTask{
			Name:        GenJobName(j.workflow, j.name, jobSubTaskID),
			Key:         genJobKey(j.name, target.ServiceName),
			DisplayName: genJobDisplayName(j.name, target.ServiceName),
			OriginName:  j.name,
			JobInfo: map[string]string{
				JobNameKey:     j.name,
				"service_name": target.ServiceName,
			},
			JobType: string(config.JobK8sBlueGreenRelease),
			Spec: &commonmodels.JobTaskBlueGreenReleaseV2Spec{
				Production:    deployJobSpec.Production,
				Env:           deployJobSpec.Env,
				Service:       target,
				DeployTimeout: timeout,
			},
			ErrorPolicy: j.errorPolicy,
		}
		resp = append(resp, task)
	}

	return resp, nil
}

func (j GrayReleaseJobController) SetRepo(repo *types.Repository) error {
	return nil
}

func (j GrayReleaseJobController) SetRepoCommitInfo() error {
	return nil
}

func (j GrayReleaseJobController) GetVariableList(jobName string, getAggregatedVariables, getRuntimeVariables, getPlaceHolderVariables, getServiceSpecificVariables, getReferredKeyValVariables bool) ([]*commonmodels.KeyVal, error) {
	return make([]*commonmodels.KeyVal, 0), nil
}

func (j GrayReleaseJobController) GetUsedRepos() ([]*types.Repository, error) {
	return make([]*types.Repository, 0), nil
}
