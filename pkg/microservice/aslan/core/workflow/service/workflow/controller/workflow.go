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
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	configbase "github.com/koderover/zadig/v2/pkg/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	templaterepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb/template"
	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	jobctrl "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/workflow/service/workflow/controller/job"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/shared/client/plutusvendor"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/types"
	"github.com/koderover/zadig/v2/pkg/util"
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
			if job.RunPolicy == config.DefaultNotRun {
				job.Skipped = true
			}

			ctrl, err := jobctrl.CreateJobController(job, w.WorkflowV4)
			if err != nil {
				return err
			}

			err = ctrl.Update(true, ticket)
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

func (w *Workflow) ToJobTasks(taskID int64, creator, account, uid string) ([]*commonmodels.StageTask, error) {
	resp := make([]*commonmodels.StageTask, 0)

	globalKeyMap := make(map[string]string)

	// first we need to set the commit info to jobs so the built-in parameters can be rendered
	for _, stage := range w.Stages {
		for _, job := range stage.Jobs {
			if job.Skipped {
				continue
			}

			ctrl, err := jobctrl.CreateJobController(job, w.WorkflowV4)
			if err != nil {
				return nil, err
			}

			err = ctrl.SetRepoCommitInfo()
			if err != nil {
				return nil, err
			}

			ctrl.ClearOptions()

			job.Spec = ctrl.GetSpec()

			kvs, err := ctrl.GetVariableList(job.Name,
				true,
				false,
				false,
				true,
				true,
			)
			if err != nil {
				return nil, err
			}

			for _, kv := range kvs {
				if kv.GetValue() != "" && !strings.HasPrefix(kv.GetValue(), "{{.") {
					globalKeyMap[kv.Key] = kv.GetValue()
					log.Debugf("insert key %s with value %s", kv.Key, kv.GetValue())
				} else {
					log.Warnf("key %s skipped due to no value or reference value: [%s]", kv.Key, kv.GetValue())
				}
			}
		}
	}

	// then we render the workflow with the built-in & user-defined parameter
	err := w.RenderWorkflowDefaultParams(taskID, creator, account, uid)
	if err != nil {
		return nil, err
	}

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

			// Update the spec, since sometimes we update the calculated field
			job.Spec = ctrl.GetSpec()

			switch job.JobType {
			case config.JobFreestyle, config.JobZadigTesting, config.JobZadigBuild, config.JobZadigScanning:
				if w.Debug {
					for _, task := range tasks {
						task.BreakpointBefore = true
					}
				}
			}

			for _, task := range tasks {
				taskBytes, _ := json.Marshal(task)
				taskString := string(taskBytes)
				for k, v := range globalKeyMap {
					taskString = strings.ReplaceAll(taskString, fmt.Sprintf("{{.%s}}", k), v)
					log.Debugf("replacing key %s with value: %s", fmt.Sprintf("{{.%s}}", k), v)
				}

				err := json.Unmarshal([]byte(taskString), &task)
				if err != nil {
					return nil, fmt.Errorf("failed to replace input variable for task: %s, error: %s", task.Name, err)
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

func (w *Workflow) SetParameterRepoCommitInfo() {
	for _, param := range w.Params {
		if param.ParamsType != "repo" {
			continue
		}
		err := commonservice.FillRepositoryInfo(param.Repo)
		// TODO: possibly fix this logic. This is a compatibility code for old version. we should not skip it.
		if err != nil {
			log.Errorf("failed to fill repository info for workflow: %s, param key: %s, error: %s", w.Name, param.Name, err)
		}
	}
}

// UpdateWithLatestWorkflow use the current workflow as input and update the fields to the latest workflow's setting
func (w *Workflow) UpdateWithLatestWorkflow(ticket *commonmodels.ApprovalTicket) error {
	latestWorkflowSettings, err := commonrepo.NewWorkflowV4Coll().Find(w.Name)
	if err != nil {
		return e.ErrFindWorkflow.AddDesc(fmt.Sprintf("cannot find workflow [%s]'s latest setting, error: %s", w.Name, err))
	}

	w.Params = renderParams(latestWorkflowSettings.Params, w.Params)

	newStage := make([]*commonmodels.WorkflowStage, 0)
	err = util.DeepCopy(&newStage, &latestWorkflowSettings.Stages)
	if err != nil {
		return err
	}

	originJobMap := make(map[string]*commonmodels.Job)
	for _, stage := range w.Stages {
		for _, job := range stage.Jobs {
			originJobMap[job.Name] = job
		}
	}

	for _, stage := range newStage {
		jobList := make([]*commonmodels.Job, 0)
		for _, job := range stage.Jobs {
			if originJob, ok := originJobMap[job.Name]; !ok || originJob.JobType != job.JobType {
				// if we didn't find the job in the workflow to be merged, simply add the new job to the list
				jobList = append(jobList, job)
				continue
			}

			// otherwise we do a merge
			ctrl, err := jobctrl.CreateJobController(originJobMap[job.Name], latestWorkflowSettings)
			if err != nil {
				return err
			}

			err = ctrl.Update(true, ticket)
			if err != nil {
				return err
			}

			err = ctrl.SetOptions(ticket)
			if err != nil {
				return err
			}

			originJobMap[job.Name].Spec = ctrl.GetSpec()
			jobList = append(jobList, originJobMap[job.Name])
		}
		stage.Jobs = jobList
	}

	w.Stages = newStage
	return nil
}

func (w *Workflow) ClearOptions() error {
	for _, stage := range w.Stages {
		for _, job := range stage.Jobs {
			ctrl, err := jobctrl.CreateJobController(job, w.WorkflowV4)
			if err != nil {
				return err
			}

			ctrl.ClearOptions()

			job.Spec = ctrl.GetSpec()
		}
	}
	return nil
}

func (w *Workflow) RenderWorkflowDefaultParams(taskID int64, creator, account, uid string) error {
	b, err := json.Marshal(w.WorkflowV4)
	if err != nil {
		return fmt.Errorf("marshal workflow error: %v", err)
	}
	globalParams, err := w.getWorkflowDefaultParams(taskID, creator, account, uid)
	if err != nil {
		return fmt.Errorf("get workflow default params error: %v", err)
	}
	replacedString := renderMultiLineString(string(b), globalParams)
	return json.Unmarshal([]byte(replacedString), &w.WorkflowV4)
}

func (w *Workflow) getWorkflowDefaultParams(taskID int64, creator, account, uid string) ([]*commonmodels.Param, error) {
	resp := []*commonmodels.Param{}
	projectInfo, err := templaterepo.NewProductColl().Find(w.Project)
	if err != nil {
		return nil, fmt.Errorf("failed to find project info for project %s, error: %s", w.Project, err)
	}
	resp = append(resp, &commonmodels.Param{Name: "project", Value: w.Project, ParamsType: "string", IsCredential: false})
	resp = append(resp, &commonmodels.Param{Name: "project.id", Value: w.Project, ParamsType: "string", IsCredential: false})
	resp = append(resp, &commonmodels.Param{Name: "project.name", Value: projectInfo.ProjectName, ParamsType: "string", IsCredential: false})
	resp = append(resp, &commonmodels.Param{Name: "workflow.id", Value: w.Name, ParamsType: "string", IsCredential: false})
	resp = append(resp, &commonmodels.Param{Name: "workflow.name", Value: w.DisplayName, ParamsType: "string", IsCredential: false})
	resp = append(resp, &commonmodels.Param{Name: "workflow.task.id", Value: fmt.Sprintf("%d", taskID), ParamsType: "string", IsCredential: false})
	resp = append(resp, &commonmodels.Param{Name: "workflow.task.creator", Value: creator, ParamsType: "string", IsCredential: false})
	resp = append(resp, &commonmodels.Param{Name: "workflow.task.creator.id", Value: account, ParamsType: "string", IsCredential: false})
	resp = append(resp, &commonmodels.Param{Name: "workflow.task.creator.userId", Value: uid, ParamsType: "string", IsCredential: false})
	resp = append(resp, &commonmodels.Param{Name: "workflow.task.timestamp", Value: fmt.Sprintf("%d", time.Now().Unix()), ParamsType: "string", IsCredential: false})
	detailURL := fmt.Sprintf("%s/v1/projects/detail/%s/pipelines/custom/%s/%d?display_name=%s",
		configbase.SystemAddress(),
		w.Project,
		w.Name,
		taskID,
		url.QueryEscape(w.DisplayName),
	)
	resp = append(resp, &commonmodels.Param{Name: "workflow.task.url", Value: detailURL, ParamsType: "string", IsCredential: false})

	for _, param := range w.Params {
		paramsKey := strings.Join([]string{"workflow", "params", param.Name}, ".")
		newParam := &commonmodels.Param{Name: paramsKey, Value: param.Value, ParamsType: "string", IsCredential: false}
		if param.ParamsType == string(commonmodels.MultiSelectType) {
			newParam.Value = strings.Join(param.ChoiceValue, ",")
		}
		resp = append(resp, newParam)
	}
	return resp, nil
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

	var latestWorkflowSettings *commonmodels.WorkflowV4
	if isExecution {
		latestWorkflowSettings, err = commonrepo.NewWorkflowV4Coll().Find(w.Name)
		if err != nil {
			return e.ErrFindWorkflow.AddDesc(fmt.Sprintf("cannot find workflow [%s]'s latest setting, error: %s", w.Name, err))
		}
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

			if isExecution {
				if job.Skipped {
					// skip validation if a job is skipped when executing
					continue
				}
				ctrl.SetWorkflow(latestWorkflowSettings)
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

func (w *Workflow) GetDynamicVariableValues(jobName, serviceName, moduleName, key string, buildInVarMap map[string]string) ([]string, error) {
	job, err := w.FindJob(jobName, "")
	if err != nil {
		return nil, err
	}

	ctrl, err := jobctrl.CreateJobController(job, w.WorkflowV4)
	if err != nil {
		return nil, err
	}

	resp, err := ctrl.RenderDynamicVariableOptions(key, &jobctrl.RenderDynamicVariableValue{
		ServiceName:   serviceName,
		ServiceModule: moduleName,
		Values:        buildInVarMap,
	})

	if err != nil {
		return nil, err
	}

	return resp, nil
}

type GetWorkflowVariablesOption struct {
	// GetAggregatedVariables gets the job's aggregated information, such as job.<name>.SERVICES in build job/ job.<name>.IMAGES in deploy job
	GetAggregatedVariables bool
	// GetRuntimeVariables gets the variables that can only be rendered in the workflow runtime. There are several examples:
	// 1. workflow level variables: workflow.task.creator
	// 2. outputs defined in each separate job
	GetRuntimeVariables bool
	// GetPlaceHolderVariables gets the variables with service/module placeholders, such as
	// job.jobName.<service>.<module>.xxxx
	GetPlaceHolderVariables bool
	// GetServiceSpecificVariables gets the variables with service/module placeholders, such as
	// job.jobName.service1.module1.xxxx
	// NOTE that there is a special case to this flag: job.jobName.service1.module1.BRANCH/COMMITID/GITURL in the build job.
	// these 3 variable is controlled by GetRuntimeVariables
	GetServiceSpecificVariables bool
	// GetReferredKeyValVariables gets the referred build/scan/testing job's key value as variables
	UseUserInput bool
}

// GetReferableVariables gets all the variable that can be used by dynamic variables/other job to refer.
// 1. the key in the response is returned in the a.b.c format, there will be no {{.}} format or replacing . with _ logic
// caller will need to process that by themselves.
// 2. Note that runtime variables will not have values in the response, use the value in the response with care.
// 3. the rendered KV will only have type string since it is mainly used for dynamic variable rendering, change this if required
func (w *Workflow) GetReferableVariables(currentJobName string, option GetWorkflowVariablesOption, skipCurrentJob bool) ([]*commonmodels.KeyVal, error) {
	resp := make([]*commonmodels.KeyVal, 0)

	resp = append(resp, &commonmodels.KeyVal{
		Key:          "project",
		Value:        w.Project,
		Type:         "string",
		IsCredential: false,
	})

	resp = append(resp, &commonmodels.KeyVal{
		Key:          "workflow.name",
		Value:        w.Name,
		Type:         "string",
		IsCredential: false,
	})

	if option.GetRuntimeVariables {
		resp = append(resp, &commonmodels.KeyVal{
			Key:          "workflow.task.creator",
			Value:        "",
			Type:         "string",
			IsCredential: false,
		})

		resp = append(resp, &commonmodels.KeyVal{
			Key:          "workflow.task.creator.id",
			Value:        "",
			Type:         "string",
			IsCredential: false,
		})

		resp = append(resp, &commonmodels.KeyVal{
			Key:          "workflow.task.creator.userId",
			Value:        "",
			Type:         "string",
			IsCredential: false,
		})

		resp = append(resp, &commonmodels.KeyVal{
			Key:          "workflow.task.timestamp",
			Value:        "",
			Type:         "string",
			IsCredential: false,
		})

		resp = append(resp, &commonmodels.KeyVal{
			Key:          "workflow.task.id",
			Value:        "",
			Type:         "string",
			IsCredential: false,
		})
	}

	for _, param := range w.Params {
		if param.ParamsType == "repo" {
			continue
		}

		resp = append(resp, &commonmodels.KeyVal{
			Key:          strings.Join([]string{"workflow", "params", param.Name}, "."),
			Value:        param.GetValue(),
			Type:         "string",
			IsCredential: false,
		})
	}

	currJob, err := w.FindJob(currentJobName, "")
	if err != nil {
		return nil, fmt.Errorf("failed to find job: %s, error: %s", currentJobName, err)
	}

	currJobCtrl, err := jobctrl.CreateJobController(currJob, w.WorkflowV4)
	if err != nil {
		return nil, err
	}

	jobRankMap := jobctrl.GetJobRankMap(w.Stages)

	for _, stage := range w.Stages {
		for _, j := range stage.Jobs {
			if skipCurrentJob && j.Name == currentJobName {
				continue
			}
			getServiceSpecificVariablesFlag := option.GetServiceSpecificVariables
			getPlaceHolderVariablesFlag := option.GetPlaceHolderVariables
			getRuntimeVariableFlag := option.GetRuntimeVariables
			getAggregatedVariableFlag := option.GetAggregatedVariables
			if currentJobName != "" && jobRankMap[currentJobName] < jobRankMap[j.Name] {
				// you cant get a job's output if the current job is runs before given job
				getRuntimeVariableFlag = false
				getAggregatedVariableFlag = false
			}

			// service_module cannot be determined in
			if currJob.JobType == config.JobZadigDeploy {
				getPlaceHolderVariablesFlag = false
			}

			ctrl, err := jobctrl.CreateJobController(j, w.WorkflowV4)
			if err != nil {
				return nil, err
			}

			if !currJobCtrl.IsServiceTypeJob() {
				getServiceSpecificVariablesFlag = true
				getPlaceHolderVariablesFlag = false
			}

			kv, err := ctrl.GetVariableList(j.Name,
				getAggregatedVariableFlag,
				getRuntimeVariableFlag,
				getPlaceHolderVariablesFlag,
				getServiceSpecificVariablesFlag,
				option.UseUserInput,
			)

			if err != nil {
				return nil, err
			}

			resp = append(resp, kv...)
		}
	}

	return resp, nil
}

func (w *Workflow) GetUsedRepos() ([]*types.Repository, error) {
	resp := make([]*types.Repository, 0)
	for _, stage := range w.Stages {
		for _, j := range stage.Jobs {
			ctrl, err := jobctrl.CreateJobController(j, w.WorkflowV4)
			if err != nil {
				return nil, err
			}

			usedRepos, err := ctrl.GetUsedRepos()
			if err != nil {
				return nil, err
			}
			resp = append(resp, usedRepos...)
		}
	}
	return resp, nil
}

func renderParams(origin, input []*commonmodels.Param) []*commonmodels.Param {
	resp := make([]*commonmodels.Param, 0)
	for _, originParam := range origin {
		found := false
		for _, inputParam := range input {
			if originParam.Name == inputParam.Name {
				// always use origin credential config.
				newParam := &commonmodels.Param{
					Name:         originParam.Name,
					Description:  originParam.Description,
					ParamsType:   originParam.ParamsType,
					Value:        originParam.Value,
					Repo:         originParam.Repo,
					ChoiceOption: originParam.ChoiceOption,
					ChoiceValue:  originParam.ChoiceValue,
					Default:      originParam.Default,
					IsCredential: originParam.IsCredential,
					Source:       originParam.Source,
				}
				if originParam.Source != config.ParamSourceFixed {
					newParam.Value = inputParam.Value
					newParam.Repo = inputParam.Repo
					newParam.ChoiceValue = inputParam.ChoiceValue
				}
				resp = append(resp, newParam)
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
