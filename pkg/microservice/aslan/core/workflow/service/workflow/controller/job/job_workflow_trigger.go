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
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
)

type WorkflowTriggerJobController struct {
	*BasicInfo

	jobSpec *commonmodels.WorkflowTriggerJobSpec
}

func CreateWorkflowTriggerJobController(job *commonmodels.Job, workflow *commonmodels.WorkflowV4) (Job, error) {
	spec := new(commonmodels.WorkflowTriggerJobSpec)
	if err := commonmodels.IToi(job.Spec, spec); err != nil {
		return nil, fmt.Errorf("failed to create apollo job controller, error: %s", err)
	}

	basicInfo := &BasicInfo{
		name:        job.Name,
		jobType:     job.JobType,
		errorPolicy: job.ErrorPolicy,
		workflow:    workflow,
	}

	return WorkflowTriggerJobController{
		BasicInfo: basicInfo,
		jobSpec:   spec,
	}, nil
}

func (j WorkflowTriggerJobController) SetWorkflow(wf *commonmodels.WorkflowV4) {
	j.workflow = wf
}

func (j WorkflowTriggerJobController) GetSpec() interface{} {
	return j.jobSpec
}

func (j WorkflowTriggerJobController) Validate(isExecution bool) error {
	if err := util.CheckZadigProfessionalLicense(); err != nil {
		return e.ErrLicenseInvalid.AddDesc("")
	}

	workflowSet := sets.NewString(j.workflow.Name)
	// every workflow only need check loop once
	checkedWorkflow := sets.NewString()
	for _, info := range j.jobSpec.ServiceTriggerWorkflow {
		if checkedWorkflow.Has(info.WorkflowName) {
			continue
		}
		workflow, err := mongodb.NewWorkflowV4Coll().Find(info.WorkflowName)
		if err != nil {
			return fmt.Errorf("can't found workflow %s: %v", info.WorkflowName, err)
		}
		if workflowSet.Has(workflow.Name) {
			return fmt.Errorf("工作流不能循环触发, 工作流名称: %s", workflow.Name)
		}
		checkedWorkflow.Insert(workflow.Name)

		if err := checkWorkflowTriggerLoop(workflow, sets.NewString(append(workflowSet.List(), workflow.Name)...)); err != nil {
			return err
		}

		for _, stage := range workflow.Stages {
			for _, job := range stage.Jobs {
				switch job.JobType {
				case config.JobFreestyle, config.JobPlugin, config.JobWorkflowTrigger:
				default:
					return fmt.Errorf("工作流 %s 中的任务 %s 类型不支持被触发", workflow.Name, job.Name)
				}
			}
		}
	}

	if j.jobSpec.TriggerType != config.WorkflowTriggerTypeCommon || j.jobSpec.Source != config.TriggerWorkflowSourceFromJob {
		return nil
	}
	jobRankMap := GetJobRankMap(j.workflow.Stages)
	sourceJobRank, ok := jobRankMap[j.jobSpec.SourceJobName]
	if !ok || sourceJobRank >= jobRankMap[j.name] {
		return fmt.Errorf("can not quote job %s in job %s", j.jobSpec.SourceJobName, j.name)
	}

	return nil
}

func (j WorkflowTriggerJobController) Update(useUserInput bool, ticket *commonmodels.ApprovalTicket) error {
	currJob, err := j.workflow.FindJob(j.name, j.jobType)
	if err != nil {
		return err
	}

	currJobSpec := new(commonmodels.WorkflowTriggerJobSpec)
	if err := commonmodels.IToi(currJob.Spec, currJobSpec); err != nil {
		return fmt.Errorf("failed to decode apollo job spec, error: %s", err)
	}

	j.jobSpec.IsEnableCheck = currJobSpec.IsEnableCheck
	j.jobSpec.TriggerType = currJobSpec.TriggerType
	j.jobSpec.Source = currJobSpec.Source
	j.jobSpec.SourceJobName = currJobSpec.SourceJobName
	j.jobSpec.SourceService = currJobSpec.SourceService

	newFixedWorkflowList := make([]*commonmodels.ServiceTriggerWorkflowInfo, 0)
	newServiceWorkflowList := make([]*commonmodels.ServiceTriggerWorkflowInfo, 0)

	if j.jobSpec.TriggerType == config.WorkflowTriggerTypeFixed {
		fixedWorkflowMap := make(map[string]*commonmodels.ServiceTriggerWorkflowInfo)
		for _, wf := range j.jobSpec.FixedWorkflowList {
			fixedWorkflowMap[wf.WorkflowName] = wf
		}

		for _, wf := range currJobSpec.FixedWorkflowList {
			newItem := &commonmodels.ServiceTriggerWorkflowInfo{
				WorkflowName: wf.WorkflowName,
				ProjectName:  wf.ProjectName,
				Params:       wf.Params,
			}
			if userInput, ok := fixedWorkflowMap[wf.WorkflowName]; ok {
				newItem.Params = renderParams(wf.Params, userInput.Params)
			}
			newFixedWorkflowList = append(newFixedWorkflowList, newItem)
		}
	} else if j.jobSpec.TriggerType == config.WorkflowTriggerTypeCommon {
		userInput := make(map[string]*commonmodels.ServiceTriggerWorkflowInfo)
		for _, svcInput := range j.jobSpec.ServiceTriggerWorkflow {
			key := fmt.Sprintf("%s++%s++%s", svcInput.ServiceName, svcInput.ServiceModule, svcInput.WorkflowName)
			userInput[key] = svcInput
		}

		for _, wf := range currJobSpec.ServiceTriggerWorkflow {
			newItem := &commonmodels.ServiceTriggerWorkflowInfo{
				WorkflowName: wf.WorkflowName,
				ProjectName:  wf.ProjectName,
				Params:       wf.Params,
			}
			key := fmt.Sprintf("%s++%s++%s", wf.ServiceName, wf.ServiceModule, wf.WorkflowName)
			if userInput, ok := userInput[key]; ok {
				newItem.Params = renderParams(wf.Params, userInput.Params)
			}
			newFixedWorkflowList = append(newFixedWorkflowList, newItem)
		}
	}

	j.jobSpec.FixedWorkflowList = newFixedWorkflowList
	j.jobSpec.ServiceTriggerWorkflow = newServiceWorkflowList

	return nil
}

func (j WorkflowTriggerJobController) SetOptions(ticket *commonmodels.ApprovalTicket) error {
	return nil
}

func (j WorkflowTriggerJobController) ClearOptions() {
	return
}

func (j WorkflowTriggerJobController) ClearSelection() {
	return
}

func (j WorkflowTriggerJobController) ToTask(taskID int64) ([]*commonmodels.JobTask, error) {
	resp := make([]*commonmodels.JobTask, 0)
	var workflowTriggerEvents []*commonmodels.WorkflowTriggerEvent
	switch j.jobSpec.TriggerType {
	case config.WorkflowTriggerTypeCommon:
		m := make(map[commonmodels.ServiceWithModule]*commonmodels.ServiceTriggerWorkflowInfo)
		for _, info := range j.jobSpec.ServiceTriggerWorkflow {
			m[commonmodels.ServiceWithModule{
				ServiceName:   info.ServiceName,
				ServiceModule: info.ServiceModule,
			}] = info
		}
		switch j.jobSpec.Source {
		case config.TriggerWorkflowSourceRuntime:
			for _, service := range j.jobSpec.SourceService {
				// Every SourceService must exist in ServiceTriggerWorkflow
				if info, ok := m[commonmodels.ServiceWithModule{
					ServiceName:   service.ServiceName,
					ServiceModule: service.ServiceModule,
				}]; ok {
					workflowTriggerEvents = append(workflowTriggerEvents, &commonmodels.WorkflowTriggerEvent{
						WorkflowName:  info.WorkflowName,
						Params:        info.Params,
						ServiceName:   service.ServiceName,
						ServiceModule: service.ServiceModule,
						ProjectName:   info.ProjectName,
					})
				} else {
					return nil, fmt.Errorf("no workflow trigger info for service %s-%s", service.ServiceName, service.ServiceModule)
				}
			}
		case config.TriggerWorkflowSourceFromJob:
			var err error
			workflowTriggerEvents, err = j.getSourceJobTargets(j.jobSpec.SourceJobName, m)
			if err != nil {
				return nil, err
			}
		}
	case config.WorkflowTriggerTypeFixed:
		for _, w := range j.jobSpec.FixedWorkflowList {
			workflowTriggerEvents = append(workflowTriggerEvents, &commonmodels.WorkflowTriggerEvent{
				WorkflowName: w.WorkflowName,
				Params:       w.Params,
				ProjectName:  w.ProjectName,
			})
		}
	default:
		return nil, fmt.Errorf("invalid trigger type: %s", j.jobSpec.TriggerType)
	}

	for _, event := range workflowTriggerEvents {
		for _, param := range event.Params {
			j.getRepoFromJob(param)
		}
	}

	jobTask := &commonmodels.JobTask{
		Name:        GenJobName(j.workflow, j.name, 0),
		Key:         genJobKey(j.name),
		DisplayName: genJobDisplayName(j.name),
		OriginName:  j.name,
		JobInfo: map[string]string{
			JobNameKey: j.name,
		},
		JobType: string(config.JobWorkflowTrigger),
		Spec: &commonmodels.JobTaskWorkflowTriggerSpec{
			TriggerType:           j.jobSpec.TriggerType,
			IsEnableCheck:         j.jobSpec.IsEnableCheck,
			WorkflowTriggerEvents: workflowTriggerEvents,
		},
		Timeout:     0,
		ErrorPolicy: j.errorPolicy,
	}

	resp = append(resp, jobTask)

	return resp, nil
}

func (j WorkflowTriggerJobController) SetRepo(repo *types.Repository) error {
	return nil
}

func (j WorkflowTriggerJobController) SetRepoCommitInfo() error {
	return nil
}

func (j WorkflowTriggerJobController) GetVariableList(jobName string, getAggregatedVariables, getRuntimeVariables, getPlaceHolderVariables, getServiceSpecificVariables, useUserInputValue bool) ([]*commonmodels.KeyVal, error) {
	return make([]*commonmodels.KeyVal, 0), nil
}

func (j WorkflowTriggerJobController) GetUsedRepos() ([]*types.Repository, error) {
	return make([]*types.Repository, 0), nil
}

func (j WorkflowTriggerJobController) RenderDynamicVariableOptions(key string, option *RenderDynamicVariableValue) ([]string, error) {
	return nil, fmt.Errorf("invalid job type: %s to render dynamic variable", j.name)
}

func (j WorkflowTriggerJobController) IsServiceTypeJob() bool {
	return false
}

func checkWorkflowTriggerLoop(workflow *commonmodels.WorkflowV4, workflowSet sets.String) error {
	// every workflow only need check loop once
	checkedWorkflow := sets.NewString()
	for _, stage := range workflow.Stages {
		for _, job := range stage.Jobs {
			if job.JobType == config.JobWorkflowTrigger {
				triggerSpec := &commonmodels.WorkflowTriggerJobSpec{}
				if err := commonmodels.IToi(job.Spec, triggerSpec); err != nil {
					return err
				}
				for _, info := range triggerSpec.ServiceTriggerWorkflow {
					if checkedWorkflow.Has(info.WorkflowName) {
						continue
					}
					w, err := mongodb.NewWorkflowV4Coll().Find(info.WorkflowName)
					if err != nil {
						return fmt.Errorf("can't found workflow %s: %v", info.WorkflowName, err)
					}

					if workflowSet.Has(info.WorkflowName) {
						return fmt.Errorf("工作流不能循环触发, 工作流名称: %s", workflow.Name)
					}
					checkedWorkflow.Insert(info.WorkflowName)

					if err := checkWorkflowTriggerLoop(w, sets.NewString(append(workflowSet.List(), info.WorkflowName)...)); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

func (j *WorkflowTriggerJobController) getRepoFromJob(param *commonmodels.Param) {
	if param.ParamsType != "repo" {
		return
	}
	if param.Repo == nil {
		return
	}
	if param.Repo.SourceFrom == types.RepoSourceJob {
		for _, stage := range j.workflow.Stages {
			for _, job := range stage.Jobs {
				if job.Name != param.Repo.JobName {
					continue
				}
				switch v := job.Spec.(type) {
				case *commonmodels.ZadigBuildJobSpec:
					for _, build := range v.ServiceAndBuilds {
						if build.ServiceName != param.Repo.ServiceName || build.ServiceModule != param.Repo.ServiceModule {
							continue
						}
						if len(build.Repos) >= param.Repo.JobRepoIndex {
							param.Repo = build.Repos[param.Repo.JobRepoIndex]
							return
						}
					}
				}
			}
		}
	}
}

func (j *WorkflowTriggerJobController) getSourceJobTargets(jobName string, m map[commonmodels.ServiceWithModule]*commonmodels.ServiceTriggerWorkflowInfo) (resp []*commonmodels.WorkflowTriggerEvent, err error) {
	for _, stage := range j.workflow.Stages {
		for _, job := range stage.Jobs {
			if j.jobSpec.SourceJobName != job.Name {
				continue
			}
			switch job.JobType {
			case config.JobZadigBuild:
				buildSpec := &commonmodels.ZadigBuildJobSpec{}
				if err := commonmodels.IToi(job.Spec, buildSpec); err != nil {
					return nil, err
				}
				for _, build := range buildSpec.ServiceAndBuilds {
					if info, ok := m[commonmodels.ServiceWithModule{
						ServiceName:   build.ServiceName,
						ServiceModule: build.ServiceModule,
					}]; ok {
						resp = append(resp, &commonmodels.WorkflowTriggerEvent{
							WorkflowName:  info.WorkflowName,
							Params:        info.Params,
							ServiceName:   build.ServiceName,
							ServiceModule: build.ServiceModule,
							ProjectName:   info.ProjectName,
						})
					}
				}
				return
			case config.JobZadigDistributeImage:
				distributeSpec := &commonmodels.ZadigDistributeImageJobSpec{}
				if err := commonmodels.IToi(job.Spec, distributeSpec); err != nil {
					return nil, err
				}
				for _, distribute := range distributeSpec.Targets {
					if info, ok := m[commonmodels.ServiceWithModule{
						ServiceName:   distribute.ServiceName,
						ServiceModule: distribute.ServiceModule,
					}]; ok {
						resp = append(resp, &commonmodels.WorkflowTriggerEvent{
							WorkflowName:  info.WorkflowName,
							Params:        info.Params,
							ServiceName:   distribute.ServiceName,
							ServiceModule: distribute.ServiceModule,
							ProjectName:   info.ProjectName,
						})
					}
				}
			case config.JobZadigDeploy:
				deploySpec := &commonmodels.ZadigDeployJobSpec{}
				if err := commonmodels.IToi(job.Spec, deploySpec); err != nil {
					return nil, err
				}
				for _, svc := range deploySpec.Services {
					for _, module := range svc.Modules {
						if info, ok := m[commonmodels.ServiceWithModule{
							ServiceName:   svc.ServiceName,
							ServiceModule: module.ServiceModule,
						}]; ok {
							resp = append(resp, &commonmodels.WorkflowTriggerEvent{
								WorkflowName:  info.WorkflowName,
								Params:        info.Params,
								ServiceName:   svc.ServiceName,
								ServiceModule: module.ServiceModule,
								ProjectName:   info.ProjectName,
							})
						}
					}
				}
			}
			return
		}
	}
	return nil, fmt.Errorf("service from job %s not found", jobName)
}
