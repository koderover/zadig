/*
Copyright 2023 The KodeRover Authors.

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

package workflow

import (
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
	"gorm.io/gorm/utils"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/workflow/service/workflow/controller"
	"github.com/koderover/zadig/v2/pkg/microservice/systemconfig/core/codehost/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/setting"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

// CreateCustomWorkflowTask creates a task for custom workflow with user-friendly inputs, this is currently
// used for openAPI
func CreateCustomWorkflowTask(username string, args *OpenAPICreateCustomWorkflowTaskArgs, log *zap.SugaredLogger) (*CreateTaskV4Resp, error) {
	// first we generate a detailed workflow.
	workflow, err := commonrepo.NewWorkflowV4Coll().Find(args.WorkflowName)
	if err != nil {
		log.Errorf("cannot find workflow %s, the error is: %v", args.WorkflowName, err)
		return nil, e.ErrFindWorkflow.AddDesc(err.Error())
	}

	if workflow.EnableApprovalTicket {
		return nil, e.ErrCreateTask.AddDesc("workflow need approval ticket to run, which is not supported by openAPI right now.")
	}

	workflowController := controller.CreateWorkflowController(workflow)

	if err := workflowController.SetPreset(nil); err != nil {
		log.Errorf("cannot get workflow %s preset, the error is: %v", args.WorkflowName, err)
		return nil, e.ErrPresetWorkflow.AddDesc(err.Error())
	}

	if err := fillWorkflowV4(workflow, log); err != nil {
		return nil, err
	}

	workflowParamMap := make(map[string]*commonmodels.Param)
	for _, param := range workflow.Params {
		workflowParamMap[param.Name] = param
	}

	for _, argParam := range args.Params {
		if workflowParam, ok := workflowParamMap[argParam.Name]; ok {
			switch workflowParam.ParamsType {
			case "string":
				workflowParam.Value = argParam.Value
			case "text":
				workflowParam.Value = argParam.Value
			case "choice":
				choiceOptionSet := sets.NewString(workflowParam.ChoiceOption...)
				if !choiceOptionSet.Has(argParam.Value) {
					return nil, fmt.Errorf("invalid choice value %s for param %s", argParam.Value, argParam.Name)
				}
				workflowParam.Value = argParam.Value
			case "repo":
				repoInfo, err := mongodb.NewCodehostColl().GetSystemCodeHostByAlias(argParam.Repo.CodeHostName)
				if err != nil {
					return nil, errors.New("failed to find code host with name:" + argParam.Repo.CodeHostName)
				}

				if workflowParam.Repo.CodehostID == repoInfo.ID {
					if workflowParam.Repo.RepoNamespace == argParam.Repo.RepoNamespace && workflowParam.Repo.RepoName == argParam.Repo.RepoName {
						workflowParam.Repo.Branch = argParam.Repo.Branch
						workflowParam.Repo.PRs = argParam.Repo.PRs
					}
				} else {
					return nil, fmt.Errorf("codehost %s (ID %d) not found in workflow %s", argParam.Repo.CodeHostName, repoInfo.ID, args.WorkflowName)
				}
			}
		} else {
			return nil, fmt.Errorf("param %s not found in workflow %s", argParam.Name, args.WorkflowName)
		}
	}

	inputMap := make(map[string]interface{})
	for _, input := range args.Inputs {
		inputMap[input.JobName] = input.Parameters
	}

	for _, stage := range workflow.Stages {
		jobList := make([]*commonmodels.Job, 0)
		for _, job := range stage.Jobs {
			// if a job is found, add it to the job creation list
			if inputParam, ok := inputMap[job.Name]; ok {
				updater, err := GetInputUpdater(job, inputParam)
				if err != nil {
					return nil, err
				}
				newJob, err := updater.UpdateJobSpec(job)
				if err != nil {
					log.Errorf("Failed to update jobspec for job: %s, error: %s", job.Name, err)
					return nil, fmt.Errorf("failed to update jobspec for job: %s, err: %w", job.Name, err)
				}
				jobList = append(jobList, newJob)
			} else {
				job.Skipped = true
				jobList = append(jobList, job)
			}
		}
		stage.Jobs = jobList
	}

	return CreateWorkflowTaskV4(&CreateWorkflowTaskV4Args{
		Name:               username,
		SkipWorkflowUpdate: true,
	}, workflow, log)
}

func CreateWorkflowViewOpenAPI(name, projectName string, workflowList []*OpenAPIWorkflowViewDetail, username string, logger *zap.SugaredLogger) error {
	// the list we got in openAPI is slightly different from the normal version, adding the missing field for workflowList
	for _, workflowInfo := range workflowList {
		workflowInfo.Enabled = true
	}

	// change the type of the workflow
	for _, workflowInfo := range workflowList {
		switch workflowInfo.WorkflowType {
		case "custom":
			workflowInfo.WorkflowType = setting.CustomWorkflowType
		case "product":
			workflowInfo.WorkflowType = setting.ProductWorkflowType
		}
	}

	list := make([]*commonmodels.WorkflowViewDetail, 0)
	for _, workflowInfo := range workflowList {
		list = append(list, &commonmodels.WorkflowViewDetail{
			WorkflowName:        workflowInfo.WorkflowName,
			WorkflowDisplayName: workflowInfo.WorkflowDisplayName,
			WorkflowType:        workflowInfo.WorkflowType,
			Enabled:             workflowInfo.Enabled,
		})
	}

	return CreateWorkflowView(name, projectName, list, username, logger)
}

func UpdateWorkflowViewOpenAPI(name, projectName string, workflowList []*commonmodels.WorkflowViewDetail, username string, logger *zap.SugaredLogger) error {
	view, err := commonrepo.NewWorkflowViewColl().Find(projectName, name)
	if err != nil {
		logger.Errorf("Failed to find workflow view %s for project %s, error: %s", name, projectName, err)
		return fmt.Errorf("failed to find workflow view %s for project %s", name, projectName)
	}

	for i := 0; i < len(workflowList); i++ {
		for j := i + 1; j < len(workflowList); j++ {
			if workflowList[i].WorkflowName == workflowList[j].WorkflowName {
				logger.Errorf("workflow name duplicated")
				return errors.New("workflow name duplicated")
			}
		}
	}

	workflowNames := make([]string, 0)
	for _, workflow := range view.Workflows {
		workflowNames = append(workflowNames, workflow.WorkflowName)
	}

	for _, w := range workflowList {
		if !utils.Contains(workflowNames, w.WorkflowName) && w.Enabled {
			switch w.WorkflowType {
			case "custom":
				w.WorkflowType = setting.CustomWorkflowType
			case "product":
				w.WorkflowType = setting.ProductWorkflowType
			default:
				return fmt.Errorf("invalid workflow type %s", w.WorkflowType)
			}

			view.Workflows = append(view.Workflows, w)
			continue
		}
	}

	for _, wdb := range view.Workflows {
		for _, wuser := range workflowList {
			if wdb.WorkflowName == wuser.WorkflowName {
				wdb.Enabled = wuser.Enabled
			}
		}
	}

	input := &commonmodels.WorkflowView{
		ID:          view.ID,
		Name:        view.Name,
		ProjectName: projectName,
		Workflows:   view.Workflows,
	}
	return UpdateWorkflowView(input, username, logger)
}

func OpenAPIGetWorkflowViews(projectName string, logger *zap.SugaredLogger) ([]*OpenAPIWorkflowViewBrief, error) {
	views, err := commonrepo.NewWorkflowViewColl().ListByProject(projectName)
	if err != nil {
		logger.Errorf("Failed to list workflow views for project %s, error: %s", projectName, err)
		return nil, err
	}

	resp := make([]*OpenAPIWorkflowViewBrief, 0)
	for _, v := range views {
		view := &OpenAPIWorkflowViewBrief{
			Name:        v.Name,
			UpdateTime:  v.UpdateTime,
			UpdateBy:    v.UpdateBy,
			ProjectName: projectName,
			Workflows:   make([]*ViewWorkflow, 0),
		}
		for _, w := range v.Workflows {
			if w.Enabled {
				wf := &ViewWorkflow{
					WorkflowName: w.WorkflowName,
				}
				if w.WorkflowType == setting.CustomWorkflowType {
					wf.WorkflowType = "custom"
				}
				if w.WorkflowType == setting.ProductWorkflowType {
					wf.WorkflowType = "product"
				}
				view.Workflows = append(view.Workflows, wf)
			}
		}
		resp = append(resp, view)
	}

	return resp, nil
}

func fillWorkflowV4(workflow *commonmodels.WorkflowV4, logger *zap.SugaredLogger) error {
	for _, stage := range workflow.Stages {
		for _, job := range stage.Jobs {
			if job.JobType == config.JobZadigBuild {
				spec := &commonmodels.ZadigBuildJobSpec{}
				if err := commonmodels.IToi(job.Spec, spec); err != nil {
					logger.Errorf(err.Error())
					return e.ErrFindWorkflow.AddErr(err)
				}
				for _, build := range spec.ServiceAndBuilds {
					buildInfo, err := commonrepo.NewBuildColl().Find(&commonrepo.BuildFindOption{Name: build.BuildName})
					if err != nil {
						logger.Errorf(err.Error())
						return e.ErrFindWorkflow.AddErr(err)
					}
					kvs := buildInfo.PreBuild.Envs
					if buildInfo.TemplateID != "" {
						var templateEnvs commonmodels.KeyValList
						buildTemplate, err := commonrepo.NewBuildTemplateColl().Find(&commonrepo.BuildTemplateQueryOption{
							ID: buildInfo.TemplateID,
						})
						// if template not found, envs are empty, but do not block user.
						if err != nil {
							logger.Error("build job: %s, template not found", buildInfo.Name)
						} else {
							templateEnvs = buildTemplate.PreBuild.Envs
						}

						for _, target := range buildInfo.Targets {
							if target.ServiceName == build.ServiceName && target.ServiceModule == build.ServiceModule {
								kvs = target.Envs
							}
						}
						// if build template update any keyvals, merge it.
						kvs = commonservice.MergeBuildEnvs(templateEnvs.ToRuntimeList(), kvs.ToRuntimeList()).ToKVList()
					}
					build.KeyVals = commonservice.MergeBuildEnvs(kvs.ToRuntimeList(), build.KeyVals)
				}
				job.Spec = spec
			}
			if job.JobType == config.JobFreestyle {
				spec := &commonmodels.FreestyleJobSpec{}
				if err := commonmodels.IToi(job.Spec, spec); err != nil {
					logger.Errorf(err.Error())
					return e.ErrFindWorkflow.AddErr(err)
				}
				job.Spec = spec
			}
			if job.JobType == config.JobPlugin {
				spec := &commonmodels.PluginJobSpec{}
				if err := commonmodels.IToi(job.Spec, spec); err != nil {
					logger.Errorf(err.Error())
					return e.ErrFindWorkflow.AddErr(err)
				}
				job.Spec = spec
			}
		}
	}
	return nil
}

func GetInputUpdater(job *commonmodels.Job, input interface{}) (CustomJobInput, error) {
	switch job.JobType {
	case config.JobPlugin:
		updater := new(PluginJobInput)
		err := commonmodels.IToi(input, updater)
		return updater, err
	case config.JobFreestyle:
		updater := new(FreestyleJobInput)
		err := commonmodels.IToi(input, updater)
		return updater, err
	case config.JobZadigBuild:
		updater := new(ZadigBuildJobInput)
		err := commonmodels.IToi(input, updater)
		return updater, err
	case config.JobZadigDeploy:
		updater := new(ZadigDeployJobInput)
		err := commonmodels.IToi(input, updater)
		return updater, err
	case config.JobK8sBlueGreenDeploy:
		updater := new(BlueGreenDeployJobInput)
		err := commonmodels.IToi(input, updater)
		return updater, err
	case config.JobK8sCanaryDeploy:
		updater := new(CanaryDeployJobInput)
		err := commonmodels.IToi(input, updater)
		return updater, err
	case config.JobCustomDeploy:
		updater := new(CustomDeployJobInput)
		err := commonmodels.IToi(input, updater)
		return updater, err
	case config.JobK8sBlueGreenRelease, config.JobK8sCanaryRelease:
		updater := new(EmptyInput)
		err := commonmodels.IToi(input, updater)
		return updater, err
	case config.JobZadigTesting:
		updater := new(ZadigTestingJobInput)
		err := commonmodels.IToi(input, updater)
		return updater, err
	case config.JobK8sGrayRelease:
		updater := new(GrayReleaseJobInput)
		err := commonmodels.IToi(input, updater)
		return updater, err
	case config.JobK8sGrayRollback:
		updater := new(GrayRollbackJobInput)
		err := commonmodels.IToi(input, updater)
		return updater, err
	case config.JobK8sPatch:
		updater := new(K8sPatchJobInput)
		err := commonmodels.IToi(input, updater)
		return updater, err
	case config.JobZadigScanning:
		updater := new(ZadigScanningJobInput)
		err := commonmodels.IToi(input, updater)
		return updater, err
	case config.JobZadigVMDeploy:
		updater := new(ZadigVMDeployJobInput)
		err := commonmodels.IToi(input, updater)
		return updater, err
	case config.JobSQL:
		updater := new(SQLJobInput)
		err := commonmodels.IToi(input, updater)
		return updater, err
	default:
		return nil, errors.New("undefined job type of type:" + string(job.JobType))
	}
}

func OpenAPIDeleteCustomWorkflowV4(workflowName, projectName string, logger *zap.SugaredLogger) error {
	return DeleteWorkflowV4(workflowName, logger)
}

func OpenAPIGetCustomWorkflowV4(workflowName, projectName string, logger *zap.SugaredLogger) (*OpenAPIWorkflowV4Detail, error) {
	workflow, err := commonrepo.NewWorkflowV4Coll().Find(workflowName)
	if err != nil {
		return nil, err
	}

	resp := &OpenAPIWorkflowV4Detail{
		Name:             workflow.Name,
		DisplayName:      workflow.DisplayName,
		ProjectName:      projectName,
		Description:      workflow.Description,
		CreatedBy:        workflow.CreatedBy,
		CreateTime:       workflow.CreateTime,
		UpdateTime:       workflow.UpdateTime,
		Params:           workflow.Params,
		NotifyCtls:       workflow.NotifyCtls,
		ShareStorages:    workflow.ShareStorages,
		ConcurrencyLimit: workflow.ConcurrencyLimit,
	}

	stages := make([]*OpenAPIStage, 0)
	for _, st := range workflow.Stages {
		stage := &OpenAPIStage{
			Name:     st.Name,
			Parallel: st.Parallel,
			Jobs:     st.Jobs,
		}

		stages = append(stages, stage)
	}
	resp.Stages = stages
	return resp, nil
}

func OpenAPIGetCustomWorkflowV4List(args *OpenAPIWorkflowV4ListReq, logger *zap.SugaredLogger) (*OpenAPIWorkflowListResp, error) {
	customWorkflowNames := make([]string, 0)
	productWorkflowNames := make([]string, 0)
	if args.ViewName != "" {
		view, err := commonrepo.NewWorkflowViewColl().Find(args.ProjectKey, args.ViewName)
		if err != nil {
			if err != mongo.ErrNoDocuments && err != mongo.ErrNilDocument {
				logger.Errorf("Failed to find workflow view %s in project %s, error: %s", args.ViewName, args.ProjectKey, err)
				return nil, fmt.Errorf("failed to find workflow view %s in project %s", args.ViewName, args.ProjectKey)
			}
		} else {
			for _, workflow := range view.Workflows {
				if workflow.WorkflowType == setting.CustomWorkflowType && workflow.Enabled {
					customWorkflowNames = append(customWorkflowNames, workflow.WorkflowName)
				}
				if workflow.WorkflowType == setting.ProductWorkflowType && workflow.Enabled {
					productWorkflowNames = append(productWorkflowNames, workflow.WorkflowName)
				}
			}
		}
	}

	customs, _, err := commonrepo.NewWorkflowV4Coll().List(&commonrepo.ListWorkflowV4Option{
		ProjectName: args.ProjectKey,
		Names:       customWorkflowNames,
	}, 0, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to list custom workflow from db, error: %v", err)
	}

	products, err := commonrepo.NewWorkflowColl().List(&commonrepo.ListWorkflowOption{
		Projects: []string{args.ProjectKey},
		Names:    productWorkflowNames,
		IsSort:   true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list product workflow from db, error: %v", err)
	}

	resp := &OpenAPIWorkflowListResp{
		Workflows: make([]*WorkflowBrief, 0),
	}
	for _, workflow := range customs {
		resp.Workflows = append(resp.Workflows, &WorkflowBrief{
			WorkflowName: workflow.Name,
			DisplayName:  workflow.DisplayName,
			UpdateBy:     workflow.UpdatedBy,
			UpdateTime:   workflow.UpdateTime,
			Type:         "custom",
		})
	}
	for _, workflow := range products {
		resp.Workflows = append(resp.Workflows, &WorkflowBrief{
			WorkflowName: workflow.Name,
			DisplayName:  workflow.DisplayName,
			UpdateBy:     workflow.UpdateBy,
			UpdateTime:   workflow.UpdateTime,
			Type:         "product",
		})
	}
	return resp, nil
}

func OpenAPIRetryCustomWorkflowTaskV4(name, projectName string, taskID int64, logger *zap.SugaredLogger) error {
	return RetryWorkflowTaskV4(name, taskID, logger)
}

func OpenAPIGetCustomWorkflowTaskV4(name, projectName string, pageNum, pageSize int64, logger *zap.SugaredLogger) (*OpenAPIWorkflowV4TaskListResp, error) {
	filter := &TaskHistoryFilter{
		WorkflowName: name,
		ProjectName:  projectName,
		PageNum:      pageNum,
		PageSize:     pageSize,
	}

	tasks, total, err := ListWorkflowTaskV4ByFilter(filter, nil, logger)
	if err != nil {
		logger.Errorf("OpenAPI: ListWorkflowTaskV4ByFilter err:%v", err)
		return nil, err
	}

	resp := &OpenAPIWorkflowV4TaskListResp{
		Total:         total,
		WorkflowTasks: make([]*OpenAPIWorkflowV4Task, 0),
	}
	for _, task := range tasks {
		wt := &OpenAPIWorkflowV4Task{
			WorkflowName: task.WorkflowName,
			DisplayName:  task.WorkflowDisplayName,
			ProjectName:  projectName,
			TaskID:       task.TaskID,
			CreateTime:   task.CreateTime,
			StartTime:    task.StartTime,
			EndTime:      task.EndTime,
			TaskCreator:  task.TaskCreator,
			Status:       task.Status.ToLower(),
		}
		resp.WorkflowTasks = append(resp.WorkflowTasks, wt)
	}

	return resp, nil
}
