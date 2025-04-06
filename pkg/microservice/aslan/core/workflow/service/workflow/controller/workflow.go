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

package controller

import (
	"fmt"
	"github.com/koderover/zadig/v2/pkg/types"
	"regexp"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	templaterepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb/template"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	jobctrl "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/workflow/service/workflow/controller/job"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/shared/client/plutusvendor"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

type Workflow struct {
	*commonmodels.WorkflowV4
}

func CreateWorkflowController(wf *commonmodels.WorkflowV4) *Workflow {
	return &Workflow{wf}
}

func (w *Workflow) SetPreset(ticket *commonmodels.ApprovalTicket) error {
	for _, stage := range w.Stages {
		for _, job := range stage.Jobs {
			ctrl, err := jobctrl.CreateJobController(job, w.WorkflowV4)
			if err != nil {
				return err
			}

			err = ctrl.Update(true)
			if err != nil {
				return err
			}

			err = ctrl.SetOptions(ticket)
			if err != nil {
				return err
			}

			if job.JobType == config.JobZadigBuild ||
				job.JobType == config.JobIstioRelease ||
				job.JobType == config.JobIstioRollback ||
				job.JobType == config.JobZadigHelmChartDeploy ||
				job.JobType == config.JobK8sBlueGreenDeploy ||
				job.JobType == config.JobApollo ||
				job.JobType == config.JobK8sCanaryDeploy ||
				job.JobType == config.JobK8sGrayRelease {
				ctrl.ClearSelection()
			}

			job.Spec = ctrl.GetSpec()
		}
	}

	return nil
}

func (w *Workflow) ToJobTasks(taskID int64) ([]*commonmodels.StageTask, error) {
	resp := make([]*commonmodels.StageTask, 0)

	for _, stage := range w.Stages {
		stageTask := &commonmodels.StageTask{
			Name:       stage.Name,
			Parallel:   stage.Parallel,
			ManualExec: stage.ManualExec,
		}

		jobTasks := make([]*commonmodels.JobTask, 0)
		for _, job := range stage.Jobs {
			if job.Skipped {
				if job.RunPolicy == config.ForceRun {
					return nil, fmt.Errorf("job %s skipped, but the run policy is set to force run", job.Name)
				} else {
					continue
				}
			}
			ctrl, err := jobctrl.CreateJobController(job, w.WorkflowV4)
			if err != nil {
				return nil, err
			}

			tasks, err := ctrl.ToTask(taskID)
			if err != nil {
				return nil, err
			}

			switch job.JobType {
			case config.JobFreestyle, config.JobZadigTesting, config.JobZadigBuild, config.JobZadigScanning:
				if w.Debug {
					for _, task := range tasks {
						task.BreakpointBefore = true
					}
				}
			}

			jobTasks = append(jobTasks, tasks...)
		}

		if len(jobTasks) > 0 {
			stageTask.Jobs = jobTasks
			resp = append(resp, stageTask)
		}
	}

	return resp, nil
}

// UpdateWithLatestWorkflow use the current workflow as input and update the fields to the latest workflow's setting
func (w *Workflow) UpdateWithLatestWorkflow(ticket *commonmodels.ApprovalTicket) error {
	latestWorkflowSettings, err := commonrepo.NewWorkflowV4Coll().Find(w.Name)
	if err != nil {
		return e.ErrFindWorkflow.AddDesc(fmt.Sprintf("cannot find workflow [%s]'s latest setting, error: %s", w.Name, err))
	}

	w.Params = renderParams(w.Params, latestWorkflowSettings.Params)

	for _, stage := range w.Stages {
		for _, job := range stage.Jobs {
			ctrl, err := jobctrl.CreateJobController(job, latestWorkflowSettings)
			if err != nil {
				return err
			}

			err = ctrl.Update(true)
			if err != nil {
				return err
			}

			err = ctrl.SetOptions(ticket)
			if err != nil {
				return err
			}

			job.Spec = ctrl.GetSpec()
		}
	}

	return nil
}

func (w *Workflow) Validate(isExecution bool) error {
	if w.Project == "" {
		err := fmt.Errorf("project should not be empty")
		return e.ErrLintWorkflow.AddErr(err)
	}

	match, err := regexp.MatchString(setting.WorkflowRegx, w.Name)
	if err != nil {
		return e.ErrLintWorkflow.AddErr(err)
	}
	if !match {
		errMsg := "工作流标识支持大小写字母、数字和中划线"
		return e.ErrLintWorkflow.AddDesc(errMsg)
	}

	project, err := templaterepo.NewProductColl().Find(w.Project)
	if err != nil {
		return e.ErrLintWorkflow.AddErr(err)
	}

	licenseStatus, err := plutusvendor.New().CheckZadigXLicenseStatus()
	if err != nil {
		return fmt.Errorf("failed to validate zadig license status, error: %s", err)
	}
	if !commonutil.ValidateZadigProfessionalLicense(licenseStatus) {
		if w.ConcurrencyLimit != -1 && w.ConcurrencyLimit != 1 {
			return e.ErrLicenseInvalid.AddDesc("基础版工作流并发只支持开关，不支持数量")
		}
	}

	if project.ProductFeature != nil {
		if project.ProductFeature.DeployType != setting.K8SDeployType && project.ProductFeature.DeployType != setting.HelmDeployType {
			return e.ErrLintWorkflow.AddDesc("common workflow only support k8s and helm project")
		}
	}
	stageNameMap := make(map[string]bool)
	jobNameMap := make(map[string]string)

	reg, err := regexp.Compile(setting.JobNameRegx)
	if err != nil {
		return e.ErrLintWorkflow.AddErr(err)
	}
	for _, stage := range w.Stages {
		if !commonutil.ValidateZadigProfessionalLicense(licenseStatus) {
			if stage.ManualExec != nil && stage.ManualExec.Enabled {
				return e.ErrLicenseInvalid.AddDesc("基础版不支持工作流手动执行")
			}
		}

		if _, ok := stageNameMap[stage.Name]; !ok {
			stageNameMap[stage.Name] = true
		} else {
			return e.ErrLintWorkflow.AddDesc(fmt.Sprintf("duplicated stage name: %s", stage.Name))
		}
		for _, job := range stage.Jobs {
			if match := reg.MatchString(job.Name); !match {
				return e.ErrLintWorkflow.AddDesc(fmt.Sprintf("job name [%s] did not match %s", job.Name, setting.JobNameRegx))
			}
			if _, ok := jobNameMap[job.Name]; !ok {
				jobNameMap[job.Name] = string(job.JobType)
			} else {
				return e.ErrLintWorkflow.AddDesc(fmt.Sprintf("duplicated job name: %s", job.Name))
			}
			ctrl, err := jobctrl.CreateJobController(job, w.WorkflowV4)
			if err != nil {
				return e.ErrLintWorkflow.AddErr(err)
			}

			if err := ctrl.Validate(isExecution); err != nil {
				return e.ErrLintWorkflow.AddErr(err)
			}
		}
	}
	return nil
}

func (w *Workflow) SetRepo(repo *types.Repository) error {
	for _, stage := range w.Stages {
		for _, job := range stage.Jobs {
			ctrl, err := jobctrl.CreateJobController(job, w.WorkflowV4)
			if err != nil {
				return err
			}

			err = ctrl.SetRepo(repo)
			if err != nil {
				return err
			}

			job.Spec = ctrl.GetSpec()
		}
	}
	return nil
}

func renderParams(origin, input []*commonmodels.Param) []*commonmodels.Param {
	resp := make([]*commonmodels.Param, 0)
	for _, originParam := range origin {
		found := false
		for _, inputParam := range input {
			if originParam.Name == inputParam.Name {
				// always use origin credential config.
				resp = append(resp, &commonmodels.Param{
					Name:         originParam.Name,
					Description:  originParam.Description,
					ParamsType:   originParam.ParamsType,
					Value:        inputParam.Value,
					Repo:         inputParam.Repo,
					ChoiceOption: originParam.ChoiceOption,
					Default:      originParam.Default,
					IsCredential: originParam.IsCredential,
					Source:       originParam.Source,
				})
				found = true
				break
			}
		}
		if !found {
			resp = append(resp, originParam)
		}
	}
	return resp
}
