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

type IstioReleaseJobController struct {
	*BasicInfo

	jobSpec *commonmodels.IstioJobSpec
}

func CreateIstioReleaseJobController(job *commonmodels.Job, workflow *commonmodels.WorkflowV4) (Job, error) {
	spec := new(commonmodels.IstioJobSpec)
	if err := commonmodels.IToi(job.Spec, spec); err != nil {
		return nil, fmt.Errorf("failed to create apollo job controller, error: %s", err)
	}

	basicInfo := &BasicInfo{
		name:        job.Name,
		jobType:     job.JobType,
		errorPolicy: job.ErrorPolicy,
		workflow:    workflow,
	}

	return IstioReleaseJobController{
		BasicInfo: basicInfo,
		jobSpec:   spec,
	}, nil
}

func (j IstioReleaseJobController) SetWorkflow(wf *commonmodels.WorkflowV4) {
	j.workflow = wf
}

func (j IstioReleaseJobController) GetSpec() interface{} {
	return j.jobSpec
}

type lintIstioReleaseJob struct {
	jobName string
	weight  int64
}

func (j IstioReleaseJobController) Validate(isExecution bool) error {
	if err := util.CheckZadigProfessionalLicense(); err != nil {
		return e.ErrLicenseInvalid.AddDesc("")
	}

	if j.jobSpec.Weight > 100 {
		return fmt.Errorf("istio release job: [%s] weight cannot be more than 100", j.name)
	}

	//from job was empty means it is the first deploy job.
	if j.jobSpec.FromJob == "" {
		jobRankmap := GetJobRankMap(j.workflow.Stages)
		releaseJobs := make([]*lintIstioReleaseJob, 0)
		for _, stage := range j.workflow.Stages {
			for _, job := range stage.Jobs {
				if job.JobType != config.JobIstioRelease {
					continue
				}
				jobSpec := &commonmodels.IstioJobSpec{}
				if err := commonmodels.IToiYaml(job.Spec, jobSpec); err != nil {
					return err
				}
				if jobSpec.FromJob != j.name {
					continue
				}
				releaseJobs = append(releaseJobs, &lintIstioReleaseJob{jobName: job.Name, weight: jobSpec.Weight})
			}
		}
		if len(releaseJobs) == 0 {
			return fmt.Errorf("no release job found for job [%s]", j.name)
		}
		for i, releaseJob := range releaseJobs {
			if jobRankmap[j.name] >= jobRankmap[releaseJob.jobName] {
				return fmt.Errorf("istio release job: [%s] must be run before [%s]", j.name, releaseJob.jobName)
			}
			if i < len(releaseJobs)-1 && releaseJob.weight >= 100 {
				return fmt.Errorf("istio release job: [%s] cannot full release in the middle", releaseJob.jobName)
			}
			if i == len(releaseJobs)-1 && releaseJob.weight != 100 {
				return fmt.Errorf("istio last release job: [%s] must be full released", releaseJob.jobName)
			}
		}
		return nil
	}

	var quoteJobSpec *commonmodels.IstioJobSpec
	for _, stage := range j.workflow.Stages {
		for _, job := range stage.Jobs {
			if job.JobType != config.JobIstioRelease || job.Name != j.jobSpec.FromJob {
				continue
			}
			quoteJobSpec = &commonmodels.IstioJobSpec{}
			if err := commonmodels.IToiYaml(job.Spec, quoteJobSpec); err != nil {
				return err
			}
			break
		}
	}

	if quoteJobSpec == nil {
		return fmt.Errorf("[%s] quote istio relase job: [%s] not found", j.name, j.jobSpec.FromJob)
	}
	if quoteJobSpec.FromJob != "" {
		return fmt.Errorf("[%s] cannot quote a non-first-release job [%s]", j.name, j.jobSpec.FromJob)
	}
	return nil
}

func (j IstioReleaseJobController) Update(useUserInput bool, ticket *commonmodels.ApprovalTicket) error {
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

func (j IstioReleaseJobController) SetOptions(ticket *commonmodels.ApprovalTicket) error {
	return nil
}

func (j IstioReleaseJobController) ClearOptions() {
	return
}

func (j IstioReleaseJobController) ClearSelection() {
	return
}

func (j IstioReleaseJobController) ToTask(taskID int64) ([]*commonmodels.JobTask, error) {
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

func (j IstioReleaseJobController) SetRepo(repo *types.Repository) error {
	return nil
}

func (j IstioReleaseJobController) SetRepoCommitInfo() error {
	return nil
}

func (j IstioReleaseJobController) GetVariableList(jobName string, getAggregatedVariables, getRuntimeVariables, getPlaceHolderVariables, getServiceSpecificVariables, getReferredKeyValVariables bool) ([]*commonmodels.KeyVal, error) {
	return make([]*commonmodels.KeyVal, 0), nil
}

func (j IstioReleaseJobController) GetUsedRepos() ([]*types.Repository, error) {
	return make([]*types.Repository, 0), nil
}

func (j IstioReleaseJobController) RenderDynamicVariableOptions(key string, option *RenderDynamicVariableValue) ([]string, error) {
	return nil, fmt.Errorf("invalid job type: %s to render dynamic variable", j.name)
}
