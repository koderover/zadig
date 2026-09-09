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
	"strings"

	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
	"gorm.io/gorm/utils"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/workflow/service/workflow/controller"
	"github.com/koderover/zadig/v2/pkg/microservice/systemconfig/core/codehost/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/setting"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

// CreateCustomWorkflowTask creates a task for custom workflow with user-friendly inputs, this is currently
// used for openAPI
func CreateCustomWorkflowTask(username, userID string, args *OpenAPICreateCustomWorkflowTaskArgs, log *zap.SugaredLogger) (*CreateTaskV4Resp, error) {
	// first we generate a detailed workflow.
	workflow, err := FindWorkflowV4RenderedForExecution(args.WorkflowName, log)
	if err != nil {
		log.Errorf("cannot find workflow %s, the error is: %v", args.WorkflowName, err)
		return nil, e.ErrFindWorkflow.AddDesc(err.Error())
	}
	if workflow.Project != args.ProjectName {
		return nil, e.ErrInvalidParam.AddDesc("workflow does not belong to the specified project")
	}

	if workflow.EnableApprovalTicket {
		return nil, e.ErrCreateTask.AddDesc("workflow need approval ticket to run, which is not supported by openAPI right now.")
	}

	workflowController := controller.CreateWorkflowController(workflow)
	if err := workflowController.SetPreset(nil); err != nil {
		return nil, e.ErrPresetWorkflow.AddErr(err)
	}
	if err := ensureWorkflowV4Resp("", workflow, log); err != nil {
		return nil, err
	}
	workflow.Remark = args.Remark

	err = UpdateProjectWorkflowParam(workflow.Params, args.Params, args.ProjectName)
	if err != nil {
		return nil, e.ErrInvalidParam.AddErr(err)
	}
	if err := workflowController.RenderWorkflowDynamicParams(0, username, username, userID, nil); err != nil {
		return nil, e.ErrPresetWorkflow.AddErr(err)
	}
	if err := validateOpenAPIWorkflowChoices(workflow.Params); err != nil {
		return nil, e.ErrInvalidParam.AddErr(err)
	}

	if err := updateOpenAPIWorkflowJobs(workflow, args.Inputs); err != nil {
		return nil, e.ErrInvalidParam.AddErr(err)
	}
	if err := ValidateWorkflowControllerWithLatestRenderedWorkflow(workflowController, log); err != nil {
		return nil, e.ErrCreateTask.AddErr(err)
	}

	return CreateWorkflowTaskV4(&CreateWorkflowTaskV4Args{
		Name:               username,
		UserID:             userID,
		SkipWorkflowUpdate: true,
		NotifyInput:        args.NotifyInputs,
	}, workflow, log)
}

func updateOpenAPIWorkflowJobs(workflow *commonmodels.WorkflowV4, inputs []*CreateCustomTaskJobInput) error {
	inputMap := make(map[string]interface{}, len(inputs))
	for _, input := range inputs {
		if input == nil || input.JobName == "" {
			return errors.New("job_name is required")
		}
		if _, exists := inputMap[input.JobName]; exists {
			return fmt.Errorf("duplicate job input: %s", input.JobName)
		}
		inputMap[input.JobName] = input.Parameters
	}

	for _, stage := range workflow.Stages {
		for i, job := range stage.Jobs {
			if inputParam, ok := inputMap[job.Name]; ok {
				updater, err := getInputUpdater(job, inputParam, workflow, true)
				if err != nil {
					return err
				}

				newJob, err := updater.UpdateJobSpec(job)
				if err != nil {
					return fmt.Errorf("failed to update jobspec for job: %s, err: %w", job.Name, err)
				}

				newJob.Skipped = false
				stage.Jobs[i] = newJob
				delete(inputMap, job.Name)
			} else {
				job.Skipped = true
			}
		}
	}
	for name := range inputMap {
		return fmt.Errorf("job not found in workflow: %s", name)
	}
	return nil
}

func UpdateWorkflowParam(workflowParams []*commonmodels.Param, inputParams []*CreateCustomTaskParam) error {
	workflowParamMap := make(map[string]*commonmodels.Param)
	for _, param := range workflowParams {
		workflowParamMap[param.Name] = param
	}

	for _, argParam := range inputParams {
		if workflowParam, ok := workflowParamMap[argParam.Name]; ok {
			switch workflowParam.ParamsType {
			case "string", "text":
				workflowParam.Value = argParam.Value
			case "choice":
				choiceOptionSet := sets.NewString(workflowParam.ChoiceOption...)
				if !choiceOptionSet.Has(argParam.Value) {
					return fmt.Errorf("invalid choice value %s for param %s", argParam.Value, argParam.Name)
				}
				workflowParam.Value = argParam.Value
			case "repo":
				repoInfo, err := mongodb.NewCodehostColl().GetSystemCodeHostByAlias(argParam.Repo.CodeHostName)
				if err != nil {
					return errors.New("failed to find code host with name:" + argParam.Repo.CodeHostName)
				}

				if workflowParam.Repo.CodehostID == repoInfo.ID {
					if workflowParam.Repo.RepoNamespace == argParam.Repo.RepoNamespace && workflowParam.Repo.RepoName == argParam.Repo.RepoName {
						workflowParam.Repo.Branch = argParam.Repo.Branch
						workflowParam.Repo.PRs = argParam.Repo.PRs
					}
				} else {
					return fmt.Errorf("codehost %s (ID %d) not found in workflow", argParam.Repo.CodeHostName, repoInfo.ID)
				}
			}
		} else {
			return fmt.Errorf("param %s not found in workflow", argParam.Name)
		}
	}

	return nil
}

func UpdateProjectWorkflowParam(workflowParams []*commonmodels.Param, inputParams []*CreateCustomTaskParam, projectKey string) error {
	workflowParamMap := make(map[string]*commonmodels.Param)
	for _, param := range workflowParams {
		workflowParamMap[param.Name] = param
	}

	for _, argParam := range inputParams {
		if argParam == nil {
			return errors.New("workflow parameter cannot be empty")
		}
		if workflowParam, ok := workflowParamMap[argParam.Name]; ok {
			switch workflowParam.ParamsType {
			case "string", "text", "choice":
				workflowParam.Value = argParam.Value
			case "multi-select":
				workflowParam.ChoiceValue = argParam.ChoiceValue
			case "file":
				workflowParam.FileID, workflowParam.FileName, workflowParam.FilePath = argParam.FileID, argParam.FileName, argParam.FilePath
			case "repo":
				if argParam.Repo == nil || workflowParam.Repo == nil {
					return fmt.Errorf("repo value is required for param %s", argParam.Name)
				}
				if err := validateOpenAPIRepositoryRef(argParam.Repo.Branch, argParam.Repo.Tag, 0, argParam.Repo.PRs, argParam.Repo.EnableCommit, argParam.Repo.CommitID); err != nil {
					return fmt.Errorf("invalid repo value for param %s: %w", argParam.Name, err)
				}
				codehosts, err := getCodeHostInfoMapByNames([]string{argParam.Repo.CodeHostName}, projectKey)
				if err != nil {
					return err
				}
				repoInfo := codehosts[argParam.Repo.CodeHostName]
				if workflowParam.Repo.CodehostID != repoInfo.ID || workflowParam.Repo.GetRepoNamespace() != argParam.Repo.RepoNamespace || workflowParam.Repo.RepoName != argParam.Repo.RepoName {
					return fmt.Errorf("repository %s/%s from codehost %s not found in workflow", argParam.Repo.RepoNamespace, argParam.Repo.RepoName, argParam.Repo.CodeHostName)
				}
				updateOpenAPIRepositoryRef(workflowParam.Repo, argParam.Repo.Branch, argParam.Repo.Tag, 0, argParam.Repo.PRs, argParam.Repo.EnableCommit, argParam.Repo.CommitID)
			}
		} else {
			return fmt.Errorf("param %s not found in workflow", argParam.Name)
		}
	}

	return nil
}

func validateOpenAPIWorkflowChoices(params []*commonmodels.Param) error {
	for _, param := range params {
		var values []string
		switch param.ParamsType {
		case "choice":
			if param.Value != "" {
				values = []string{param.Value}
			}
		case "multi-select":
			values = param.ChoiceValue
		default:
			continue
		}
		options := sets.NewString(param.ChoiceOption...)
		for _, value := range values {
			if !options.Has(value) {
				return fmt.Errorf("invalid choice value for param %s", param.Name)
			}
		}
	}
	return nil
}

func OpenAPIPrepareCustomWorkflowTask(projectKey, workflowKey, userID, username string, log *zap.SugaredLogger) (map[string]interface{}, error) {
	workflow, err := commonrepo.NewWorkflowV4Coll().Find(workflowKey)
	if err != nil {
		return nil, e.ErrFindWorkflow.AddDesc(err.Error())
	}
	if workflow.Project != projectKey {
		return nil, e.ErrInvalidParam.AddDesc("workflow does not belong to the specified project")
	}
	if workflow.EnableApprovalTicket {
		return nil, e.ErrCreateTask.AddDesc("workflow need approval ticket to run, which is not supported by openAPI right now.")
	}
	workflow, err = GetWorkflowV4Preset("", workflowKey, userID, username, "", log)
	if err != nil {
		return nil, err
	}
	workflow.NotifyCtls = nil
	codehosts, err := mongodb.NewCodehostColl().AvailableCodeHost(projectKey)
	if err != nil {
		return nil, err
	}
	codehostNames := make(map[int]string, len(codehosts))
	codehostNameCount := make(map[string]int, len(codehosts))
	for _, codehost := range codehosts {
		codehostNames[codehost.ID] = codehost.Alias
		codehostNameCount[codehost.Alias]++
	}
	ambiguousCodehostNames := make(map[string]struct{})
	for name, count := range codehostNameCount {
		if count > 1 {
			ambiguousCodehostNames[name] = struct{}{}
		}
	}

	resp := make(map[string]interface{})
	if err := commonmodels.IToi(workflow, &resp); err != nil {
		return nil, err
	}
	if err := sanitizeOpenAPIWorkflowPreset(resp, codehostNames, ambiguousCodehostNames); err != nil {
		return nil, e.ErrInvalidParam.AddDesc(err.Error())
	}
	return resp, nil
}

func sanitizeOpenAPIWorkflowPreset(value interface{}, codehostNames map[int]string, ambiguousCodehostNames map[string]struct{}) error {
	switch value := value.(type) {
	case []interface{}:
		for _, item := range value {
			if err := sanitizeOpenAPIWorkflowPreset(item, codehostNames, ambiguousCodehostNames); err != nil {
				return err
			}
		}
	case map[string]interface{}:
		if namespace, ok := value["repo_namespace"].(string); ok && namespace == "" {
			if owner, _ := value["repo_owner"].(string); owner != "" {
				value["repo_namespace"] = owner
			}
		}
		if rawCodehostID, ok := value["codehost_id"].(float64); ok && rawCodehostID != 0 {
			codehostID := int(rawCodehostID)
			codehostName, ok := codehostNames[codehostID]
			if !ok {
				return fmt.Errorf("code host with ID %d is not available in project", codehostID)
			}
			if _, ambiguous := ambiguousCodehostNames[codehostName]; ambiguous {
				return fmt.Errorf("multiple code hosts named %s are available in project", codehostName)
			}
			value["codehost_name"] = codehostName
		}
		if credential, _ := value["is_credential"].(bool); credential {
			paramType, _ := value["type"].(string)
			switch paramType {
			case "multi-select":
				choiceValue, _ := value["choice_value"].([]interface{})
				value["has_value"] = len(choiceValue) > 0
			case "file":
				fileID, _ := value["file_id"].(string)
				filePath, _ := value["file_path"].(string)
				value["has_value"] = fileID != "" || filePath != ""
			default:
				credentialValue, _ := value["value"].(string)
				defaultValue, _ := value["default"].(string)
				value["has_value"] = credentialValue != "" || defaultValue != ""
			}
			value["value"], value["default"], value["file_id"], value["file_path"] = "", "", "", ""
			value["choice_value"] = []interface{}{}
			value["choice_option"] = []interface{}{}
		}
		if _, hasToken := value["token"]; hasToken {
			if _, hasAddress := value["address"]; hasAddress {
				value["address"] = ""
			}
		}
		for key := range value {
			if isOpenAPIWorkflowPresetCredential(key) {
				value[key] = ""
			}
		}
		for _, item := range value {
			if err := sanitizeOpenAPIWorkflowPreset(item, codehostNames, ambiguousCodehostNames); err != nil {
				return err
			}
		}
	}
	return nil
}

func isOpenAPIWorkflowPresetCredential(key string) bool {
	key = strings.ToLower(key)
	switch key {
	case "password", "token", "access_key", "secret_key", "ak", "sk", "api_key", "private_key", "ssh_key", "hook_address":
		return true
	}
	return strings.HasSuffix(key, "_password") || strings.HasSuffix(key, "_token") ||
		strings.HasSuffix(key, "_secret") || strings.HasSuffix(key, "_webhook")
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

func GetInputUpdater(job *commonmodels.Job, input interface{}, workflow *commonmodels.WorkflowV4) (CustomJobInput, error) {
	return getInputUpdater(job, input, workflow, false)
}

func getInputUpdater(job *commonmodels.Job, input interface{}, workflow *commonmodels.WorkflowV4, useProjectCodehosts bool) (CustomJobInput, error) {
	switch job.JobType {
	case config.JobPlugin:
		updater := new(PluginJobInput)
		err := commonmodels.IToi(input, updater)
		return updater, err
	case config.JobFreestyle:
		updater := new(FreestyleJobInput)
		updater.OpenAPIBasicInfo = &OpenAPIBasicInfo{
			workflow:            workflow,
			useProjectCodehosts: useProjectCodehosts,
		}
		err := commonmodels.IToi(input, updater)
		return updater, err
	case config.JobZadigBuild:
		updater := new(ZadigBuildJobInput)
		updater.OpenAPIBasicInfo = &OpenAPIBasicInfo{
			workflow:            workflow,
			useProjectCodehosts: useProjectCodehosts,
		}
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
		updater.OpenAPIBasicInfo = &OpenAPIBasicInfo{
			workflow:            workflow,
			useProjectCodehosts: useProjectCodehosts,
		}
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
		updater.OpenAPIBasicInfo = &OpenAPIBasicInfo{
			workflow:            workflow,
			useProjectCodehosts: useProjectCodehosts,
		}
		err := commonmodels.IToi(input, updater)
		return updater, err
	case config.JobZadigVMDeploy:
		updater := new(ZadigVMDeployJobInput)
		err := commonmodels.IToi(input, updater)
		return updater, err
	case config.JobApproval:
		updater := new(ApprovalJobInput)
		return updater, nil
	case config.JobAIReleaseSpecialist, config.JobAI:
		updater := new(EmptyInput)
		err := commonmodels.IToi(input, updater)
		return updater, err
	case config.JobSQL:
		updater := new(SQLJobInput)
		err := commonmodels.IToi(input, updater)
		return updater, err
	case config.JobTapd:
		updater := new(TapdJobInput)
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
