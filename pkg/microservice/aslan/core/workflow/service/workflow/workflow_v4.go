/*
Copyright 2022 The KodeRover Authors.

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
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/util/sets"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/template"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	templaterepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb/template"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/collaboration"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/nsq"
	commomtemplate "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/template"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/webhook"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/workflow/service/workflow/job"
	jobctl "github.com/koderover/zadig/pkg/microservice/aslan/core/workflow/service/workflow/job"
	"github.com/koderover/zadig/pkg/setting"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/log"
)

func CreateWorkflowV4(user string, workflow *commonmodels.WorkflowV4, logger *zap.SugaredLogger) error {
	existedWorkflow, err := commonrepo.NewWorkflowV4Coll().Find(workflow.Name)
	if err == nil {
		errStr := fmt.Sprintf("与项目 [%s] 中的工作流 [%s] 标识相同", existedWorkflow.Project, existedWorkflow.DisplayName)
		return e.ErrUpsertWorkflow.AddDesc(errStr)
	}
	existedWorkflows, _, _ := commonrepo.NewWorkflowV4Coll().List(&commonrepo.ListWorkflowV4Option{ProjectName: workflow.Project, DisplayName: workflow.DisplayName}, 0, 0)
	if len(existedWorkflows) > 0 {
		errStr := fmt.Sprintf("当前项目已存在工作流 [%s]", workflow.DisplayName)
		return e.ErrUpsertWorkflow.AddDesc(errStr)
	}
	if err := LintWorkflowV4(workflow, logger); err != nil {
		return err
	}

	workflow.CreatedBy = user
	workflow.UpdatedBy = user
	workflow.CreateTime = time.Now().Unix()
	workflow.UpdateTime = time.Now().Unix()

	for _, stage := range workflow.Stages {
		for _, job := range stage.Jobs {
			if err := jobctl.Instantiate(job, workflow); err != nil {
				logger.Errorf("Failed to instantiate workflow v4,error: %v", err)
				return e.ErrUpsertWorkflow.AddErr(err)
			}
		}
	}

	if _, err := commonrepo.NewWorkflowV4Coll().Create(workflow); err != nil {
		logger.Errorf("Failed to create workflow v4, the error is: %s", err)
		return e.ErrUpsertWorkflow.AddErr(err)
	}
	return nil
}

func UpdateWorkflowV4(name, user string, inputWorkflow *commonmodels.WorkflowV4, logger *zap.SugaredLogger) error {
	workflow, err := commonrepo.NewWorkflowV4Coll().Find(name)
	if err != nil {
		logger.Errorf("Failed to find WorkflowV4: %s, the error is: %v", name, err)
		return e.ErrFindWorkflow.AddErr(err)
	}
	if workflow.DisplayName != inputWorkflow.DisplayName {
		existedWorkflows, _, _ := commonrepo.NewWorkflowV4Coll().List(&commonrepo.ListWorkflowV4Option{ProjectName: workflow.Project, DisplayName: inputWorkflow.DisplayName}, 0, 0)
		if len(existedWorkflows) > 0 {
			errStr := fmt.Sprintf("workflow v4 [%s] 展示名称在当前项目下重复!", inputWorkflow.DisplayName)
			return e.ErrUpsertWorkflow.AddDesc(errStr)
		}
	}
	if err := LintWorkflowV4(inputWorkflow, logger); err != nil {
		return err
	}

	inputWorkflow.UpdatedBy = user
	inputWorkflow.UpdateTime = time.Now().Unix()
	inputWorkflow.ID = workflow.ID
	inputWorkflow.HookCtls = workflow.HookCtls
	inputWorkflow.JiraHookCtls = workflow.JiraHookCtls
	inputWorkflow.GeneralHookCtls = workflow.GeneralHookCtls
	inputWorkflow.MeegoHookCtls = workflow.MeegoHookCtls

	for _, stage := range inputWorkflow.Stages {
		for _, job := range stage.Jobs {
			if err := jobctl.Instantiate(job, workflow); err != nil {
				logger.Errorf("Failed to instantiate workflow v4,error: %v", err)
				return e.ErrUpsertWorkflow.AddErr(err)
			}
		}
	}

	if err := commonrepo.NewWorkflowV4Coll().Update(
		workflow.ID.Hex(),
		inputWorkflow,
	); err != nil {
		logger.Errorf("update workflowV4 error: %s", err)
		return e.ErrUpsertWorkflow.AddErr(err)
	}
	return nil
}

func FindWorkflowV4(encryptedKey, name string, logger *zap.SugaredLogger) (*commonmodels.WorkflowV4, error) {
	workflow, err := commonrepo.NewWorkflowV4Coll().Find(name)
	if err != nil {
		logger.Errorf("Failed to find WorkflowV4: %s, the error is: %v", name, err)
		return workflow, e.ErrFindWorkflow.AddErr(err)
	}
	if err := ensureWorkflowV4Resp(encryptedKey, workflow, logger); err != nil {
		return workflow, err
	}
	return workflow, err
}

func FindWorkflowV4Raw(name string, logger *zap.SugaredLogger) (*commonmodels.WorkflowV4, error) {
	workflow, err := commonrepo.NewWorkflowV4Coll().Find(name)
	if err != nil {
		logger.Errorf("Failed to find WorkflowV4: %s, the error is: %v", name, err)
		return workflow, e.ErrFindWorkflow.AddErr(err)
	}
	return workflow, err
}

func DeleteWorkflowV4(name string, logger *zap.SugaredLogger) error {
	workflow, err := commonrepo.NewWorkflowV4Coll().Find(name)
	if err != nil {
		logger.Errorf("Failed to delete WorkflowV4: %s, the error is: %v", name, err)
		return e.ErrDeleteWorkflow.AddErr(err)
	}
	if err := commonrepo.NewWorkflowV4Coll().DeleteByID(workflow.ID.Hex()); err != nil {
		logger.Errorf("Failed to delete WorkflowV4: %s, the error is: %v", name, err)
		return e.ErrDeleteWorkflow.AddErr(err)
	}
	if err := commonrepo.NewworkflowTaskv4Coll().DeleteByWorkflowName(name); err != nil {
		logger.Errorf("Failed to delete WorkflowV4 task: %s, the error is: %v", name, err)
		return e.ErrDeleteWorkflow.AddErr(err)
	}
	if err := commonrepo.NewCounterColl().Delete("WorkflowTaskV4:" + name); err != nil {
		log.Errorf("Counter.Delete error: %s", err)
	}
	return nil
}

func ListWorkflowV4(projectName, viewName, userID string, names, v4Names []string, policyFound bool, logger *zap.SugaredLogger) ([]*Workflow, error) {
	resp := make([]*Workflow, 0)
	var err error
	ignoreWorkflow := false
	ignoreWorkflowV4 := false
	if viewName == "" {
		if policyFound && len(names) == 0 {
			ignoreWorkflow = true
		}
		if policyFound && len(v4Names) == 0 {
			ignoreWorkflowV4 = true
		}
	} else {
		names, v4Names, err = filterWorkflowNamesByView(projectName, viewName, names, v4Names, policyFound)
		if err != nil {
			logger.Errorf("filterWorkflowNames error: %s", err)
			return resp, err
		}
		if len(names) == 0 {
			ignoreWorkflow = true
		}
		if len(v4Names) == 0 {
			ignoreWorkflowV4 = true
		}
	}
	workflowV4List := []*commonmodels.WorkflowV4{}
	if !ignoreWorkflowV4 {
		workflowV4List, _, err = commonrepo.NewWorkflowV4Coll().List(&commonrepo.ListWorkflowV4Option{
			ProjectName: projectName,
			Names:       v4Names,
		}, 0, 0)
		if err != nil {
			logger.Errorf("Failed to list workflow v4, the error is: %s", err)
			return resp, err
		}
	}

	workflow := []*Workflow{}

	// distribute center only surpport custom workflow.
	if !ignoreWorkflow && projectName != setting.EnterpriseProject {
		workflow, err = ListWorkflows([]string{projectName}, userID, names, logger)
		if err != nil {
			return resp, err
		}
	}

	workflowList := []string{}
	for _, wV4 := range workflowV4List {
		workflowList = append(workflowList, wV4.Name)
	}
	resp = append(resp, workflow...)
	workflowCMMap, err := collaboration.GetWorkflowCMMap([]string{projectName}, logger)
	if err != nil {
		return nil, err
	}
	tasks, _, err := commonrepo.NewworkflowTaskv4Coll().List(&commonrepo.ListWorkflowTaskV4Option{WorkflowNames: workflowList})
	if err != nil {
		return resp, err
	}
	favorites, err := commonrepo.NewFavoriteColl().List(&commonrepo.FavoriteArgs{UserID: userID, Type: string(config.WorkflowTypeV4)})
	if err != nil {
		return resp, errors.Errorf("failed to get custom workflow favorite data, err: %v", err)
	}
	favoriteSet := sets.NewString()
	for _, f := range favorites {
		favoriteSet.Insert(f.Name)
	}
	workflowStatMap := getWorkflowStatMap(workflowList, config.WorkflowTypeV4)

	for _, workflowModel := range workflowV4List {
		stages := []string{}
		for _, stage := range workflowModel.Stages {
			if stage.Approval != nil && stage.Approval.Enabled {
				stages = append(stages, "人工审批")
			}
			stages = append(stages, stage.Name)
		}
		var baseRefs []string
		if cmSet, ok := workflowCMMap[collaboration.BuildWorkflowCMMapKey(workflowModel.Project, workflowModel.Name)]; ok {
			for _, cm := range cmSet.List() {
				baseRefs = append(baseRefs, cm)
			}
		}
		workflow := &Workflow{
			Name:          workflowModel.Name,
			DisplayName:   workflowModel.DisplayName,
			ProjectName:   workflowModel.Project,
			EnabledStages: stages,
			CreateTime:    workflowModel.CreateTime,
			UpdateTime:    workflowModel.UpdateTime,
			UpdateBy:      workflowModel.UpdatedBy,
			WorkflowType:  setting.CustomWorkflowType,
			Description:   workflowModel.Description,
			BaseRefs:      baseRefs,
			BaseName:      workflowModel.BaseName,
		}
		if workflowModel.Category == setting.ReleaseWorkflow {
			workflow.WorkflowType = string(setting.ReleaseWorkflow)
		}
		if favoriteSet.Has(workflow.Name) {
			workflow.IsFavorite = true
		}
		getRecentTaskV4Info(workflow, tasks)
		setWorkflowStat(workflow, workflowStatMap)

		resp = append(resp, workflow)
	}
	return resp, nil
}

func filterWorkflowNamesByView(projectName, viewName string, workflowNames, workflowV4Names []string, policyFound bool) ([]string, []string, error) {
	if viewName == "" {
		return workflowNames, workflowV4Names, nil
	}
	view, err := commonrepo.NewWorkflowViewColl().Find(projectName, viewName)
	if err != nil {
		return workflowNames, workflowV4Names, err
	}
	enabledWorkflow := []string{}
	enabledWorkflowV4 := []string{}
	for _, workflow := range view.Workflows {
		if !workflow.Enabled {
			continue
		}
		if workflow.WorkflowType == setting.CustomWorkflowType {
			enabledWorkflowV4 = append(enabledWorkflowV4, workflow.WorkflowName)
		} else {
			enabledWorkflow = append(enabledWorkflow, workflow.WorkflowName)
		}
	}
	if !policyFound {
		return enabledWorkflow, enabledWorkflowV4, nil
	}
	return intersection(workflowNames, enabledWorkflow), intersection(workflowV4Names, enabledWorkflowV4), nil
}

func intersection(a, b []string) []string {
	m := make(map[string]bool)
	var intersection []string
	for _, item := range a {
		m[item] = true
	}
	for _, item := range b {
		if _, ok := m[item]; ok {
			intersection = append(intersection, item)
		}
	}
	return intersection
}

func getRecentTaskV4Info(workflow *Workflow, tasks []*commonmodels.WorkflowTask) {
	recentTask := &commonmodels.WorkflowTask{}
	recentFailedTask := &commonmodels.WorkflowTask{}
	recentSucceedTask := &commonmodels.WorkflowTask{}
	workflow.NeverRun = true
	for _, task := range tasks {
		if task.WorkflowName != workflow.Name {
			continue
		}
		workflow.NeverRun = false
		if task.TaskID > recentTask.TaskID {
			recentTask = task
		}
		if task.Status == config.StatusPassed && task.TaskID > recentSucceedTask.TaskID {
			recentSucceedTask = task
		}
		if task.Status == config.StatusFailed && task.TaskID > recentFailedTask.TaskID {
			recentFailedTask = task
		}
	}
	if recentTask.TaskID > 0 {
		workflow.RecentTask = &TaskInfo{
			TaskID:       recentTask.TaskID,
			PipelineName: recentTask.WorkflowName,
			Status:       string(recentTask.Status),
			TaskCreator:  recentTask.TaskCreator,
			CreateTime:   recentTask.CreateTime,
		}
	}
	if recentSucceedTask.TaskID > 0 {
		workflow.RecentSuccessfulTask = &TaskInfo{
			TaskID:       recentSucceedTask.TaskID,
			PipelineName: recentSucceedTask.WorkflowName,
			Status:       string(recentSucceedTask.Status),
			TaskCreator:  recentSucceedTask.TaskCreator,
			CreateTime:   recentSucceedTask.CreateTime,
		}
	}
	if recentFailedTask.TaskID > 0 {
		workflow.RecentFailedTask = &TaskInfo{
			TaskID:       recentFailedTask.TaskID,
			PipelineName: recentFailedTask.WorkflowName,
			Status:       string(recentFailedTask.Status),
			TaskCreator:  recentFailedTask.TaskCreator,
			CreateTime:   recentFailedTask.CreateTime,
		}
	}
}

func ensureWorkflowV4Resp(encryptedKey string, workflow *commonmodels.WorkflowV4, logger *zap.SugaredLogger) error {
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
						logger.Errorf("find build: %s error: %s", build.BuildName, err)
						continue
					}
					kvs := buildInfo.PreBuild.Envs
					if buildInfo.TemplateID != "" {
						templateEnvs := []*commonmodels.KeyVal{}
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
						kvs = commonservice.MergeBuildEnvs(templateEnvs, kvs)
					}
					build.KeyVals = commonservice.MergeBuildEnvs(kvs, build.KeyVals)
					if err := commonservice.EncryptKeyVals(encryptedKey, build.KeyVals, logger); err != nil {
						logger.Errorf(err.Error())
						return e.ErrFindWorkflow.AddErr(err)
					}
				}
				job.Spec = spec
			}
			if job.JobType == config.JobFreestyle {
				spec := &commonmodels.FreestyleJobSpec{}
				if err := commonmodels.IToi(job.Spec, spec); err != nil {
					logger.Errorf(err.Error())
					return e.ErrFindWorkflow.AddErr(err)
				}
				if err := commonservice.EncryptKeyVals(encryptedKey, spec.Properties.Envs, logger); err != nil {
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
				if err := commonservice.EncryptParams(encryptedKey, spec.Plugin.Inputs, logger); err != nil {
					logger.Errorf(err.Error())
					return e.ErrFindWorkflow.AddErr(err)
				}
				job.Spec = spec
			}
		}
	}
	return nil
}

func LintWorkflowV4(workflow *commonmodels.WorkflowV4, logger *zap.SugaredLogger) error {
	if workflow.Project == "" {
		err := fmt.Errorf("project should not be empty")
		logger.Errorf(err.Error())
		return e.ErrUpsertWorkflow.AddErr(err)
	}
	match, err := regexp.MatchString(setting.WorkflowRegx, workflow.Name)
	if err != nil {
		logger.Errorf("reg compile failed: %v", err)
		return e.ErrUpsertWorkflow.AddErr(err)
	}
	if !match {
		errMsg := "工作流标识支持小写字母、数字和中划线"
		logger.Error(errMsg)
		return e.ErrUpsertWorkflow.AddDesc(errMsg)
	}

	project := &template.Product{}
	// for deploy center workflow, it doesn't belongs to any project, so we use a specical project name to distinguish it.
	if workflow.Project != setting.EnterpriseProject {
		project, err = templaterepo.NewProductColl().Find(workflow.Project)
		if err != nil {
			logger.Errorf("Failed to get project %s, error: %v", workflow.Project, err)
			return e.ErrUpsertWorkflow.AddErr(err)
		}
	}

	if project.ProductFeature != nil {
		if project.ProductFeature.DeployType != setting.K8SDeployType && project.ProductFeature.DeployType != setting.HelmDeployType {
			logger.Error("common workflow only support k8s and helm project")
			return e.ErrUpsertWorkflow.AddDesc("common workflow only support k8s and helm project")
		}
	}
	stageNameMap := make(map[string]bool)
	jobNameMap := make(map[string]string)

	reg, err := regexp.Compile(setting.JobNameRegx)
	if err != nil {
		logger.Errorf("reg compile failed: %v", err)
		return e.ErrUpsertWorkflow.AddErr(err)
	}
	for _, stage := range workflow.Stages {
		if err := lintApprovals(stage.Approval); err != nil {
			logger.Errorf("stage: %s approval info error: %v", stage.Name, err)
			return e.ErrUpsertWorkflow.AddDesc(fmt.Sprintf("stage: %s approval info error: %v", stage.Name, err))
		}
		if _, ok := stageNameMap[stage.Name]; !ok {
			stageNameMap[stage.Name] = true
		} else {
			logger.Errorf("duplicated stage name: %s", stage.Name)
			return e.ErrUpsertWorkflow.AddDesc(fmt.Sprintf("duplicated stage name: %s", stage.Name))
		}
		for _, job := range stage.Jobs {
			if match := reg.MatchString(job.Name); !match {
				logger.Errorf("job name [%s] did not match %s", job.Name, setting.JobNameRegx)
				return e.ErrUpsertWorkflow.AddDesc(fmt.Sprintf("job name [%s] did not match %s", job.Name, setting.JobNameRegx))
			}
			if _, ok := jobNameMap[job.Name]; !ok {
				jobNameMap[job.Name] = string(job.JobType)
			} else {
				logger.Errorf("duplicated job name: %s", job.Name)
				return e.ErrUpsertWorkflow.AddDesc(fmt.Sprintf("duplicated job name: %s", job.Name))
			}
			if err := jobctl.LintJob(job, workflow); err != nil {
				logger.Errorf("lint job %s failed: %v", job.Name, err)
				return e.ErrUpsertWorkflow.AddErr(err)
			}
		}
	}
	return nil
}

func lintApprovals(approval *commonmodels.Approval) error {
	if approval == nil {
		return nil
	}
	if !approval.Enabled {
		return nil
	}
	switch approval.Type {
	case config.NativeApproval:
		if approval.NativeApproval == nil {
			return errors.New("approval not found")
		}
		if len(approval.NativeApproval.ApproveUsers) < approval.NativeApproval.NeededApprovers {
			return errors.New("all approve users should not less than needed approvers")
		}
	case config.LarkApproval:
		if approval.LarkApproval == nil {
			return errors.New("approval not found")
		}
		if len(approval.LarkApproval.ApproveUsers) == 0 {
			return errors.New("num of approver is 0")
		}
	default:
		return errors.New("invalid approval type")
	}

	return nil
}

func CreateWebhookForWorkflowV4(workflowName string, input *commonmodels.WorkflowV4Hook, logger *zap.SugaredLogger) error {
	workflow, err := commonrepo.NewWorkflowV4Coll().Find(workflowName)
	if err != nil {
		logger.Errorf("Failed to find WorkflowV4: %s, the error is: %v", workflowName, err)
		return e.ErrCreateWebhook.AddErr(err)
	}
	for _, hook := range workflow.HookCtls {
		if hook.Name == input.Name {
			errMsg := fmt.Sprintf("webhook %s already exists", input.Name)
			logger.Error(errMsg)
			return e.ErrCreateWebhook.AddDesc(errMsg)
		}
	}
	if err := validateHookNames([]string{input.Name}); err != nil {
		logger.Errorf(err.Error())
		return e.ErrCreateWebhook.AddErr(err)
	}
	err = commonservice.ProcessWebhook([]*models.WorkflowV4Hook{input}, nil, webhook.WorkflowV4Prefix+workflowName, logger)
	if err != nil {
		errMsg := fmt.Sprintf("failed to create webhook for workflow %s, the error is: %v", workflowName, err)
		log.Error(errMsg)
		return e.ErrCreateWebhook.AddDesc(errMsg)
	}
	workflow.HookCtls = append(workflow.HookCtls, input)
	if err := commonrepo.NewWorkflowV4Coll().Update(workflow.ID.Hex(), workflow); err != nil {
		errMsg := fmt.Sprintf("failed to create webhook for workflow %s, the error is: %v", workflowName, err)
		log.Error(errMsg)
		return e.ErrCreateWebhook.AddDesc(errMsg)
	}
	if err := createGerritWebhook(input.MainRepo, workflowName); err != nil {
		logger.Errorf("create gerrit webhook failed: %v", err)
	}
	return nil
}

func UpdateWebhookForWorkflowV4(workflowName string, input *commonmodels.WorkflowV4Hook, logger *zap.SugaredLogger) error {
	workflow, err := commonrepo.NewWorkflowV4Coll().Find(workflowName)
	if err != nil {
		logger.Errorf("Failed to find WorkflowV4: %s, the error is: %v", workflowName, err)
		return e.ErrUpdateWebhook.AddErr(err)
	}
	updatedHooks := []*commonmodels.WorkflowV4Hook{}
	var existHook *commonmodels.WorkflowV4Hook
	for _, hook := range workflow.HookCtls {
		if hook.Name == input.Name {
			updatedHooks = append(updatedHooks, input)
			existHook = hook
			continue
		}
		updatedHooks = append(updatedHooks, hook)
	}
	if existHook == nil {
		errMsg := fmt.Sprintf("webhook %s does not exist", input.Name)
		logger.Error(errMsg)
		return e.ErrUpdateWebhook.AddDesc(errMsg)
	}
	if err := validateHookNames([]string{input.Name}); err != nil {
		logger.Errorf(err.Error())
		return e.ErrUpdateWebhook.AddErr(err)
	}
	err = commonservice.ProcessWebhook([]*models.WorkflowV4Hook{input}, []*models.WorkflowV4Hook{existHook}, webhook.WorkflowV4Prefix+workflowName, logger)
	if err != nil {
		errMsg := fmt.Sprintf("failed to update webhook for workflow %s, the error is: %v", workflowName, err)
		log.Error(errMsg)
		return e.ErrUpdateWebhook.AddDesc(errMsg)
	}
	workflow.HookCtls = updatedHooks
	if err := commonrepo.NewWorkflowV4Coll().Update(workflow.ID.Hex(), workflow); err != nil {
		errMsg := fmt.Sprintf("failed to update webhook for workflow %s, the error is: %v", workflowName, err)
		log.Error(errMsg)
		return e.ErrUpdateWebhook.AddDesc(errMsg)
	}

	if err := deleteGerritWebhook(existHook.MainRepo, workflowName); err != nil {
		logger.Errorf("delete gerrit webhook failed: %v", err)
	}
	if err := createGerritWebhook(input.MainRepo, workflowName); err != nil {
		logger.Errorf("create gerrit webhook failed: %v", err)
	}
	return nil
}

func ListWebhookForWorkflowV4(workflowName string, logger *zap.SugaredLogger) ([]*commonmodels.WorkflowV4Hook, error) {
	workflow, err := commonrepo.NewWorkflowV4Coll().Find(workflowName)
	if err != nil {
		logger.Errorf("Failed to find WorkflowV4: %s, the error is: %v", workflowName, err)
		return []*commonmodels.WorkflowV4Hook{}, e.ErrListWebhook.AddErr(err)
	}
	return workflow.HookCtls, nil
}

func GetWebhookForWorkflowV4Preset(workflowName, triggerName string, logger *zap.SugaredLogger) (*commonmodels.WorkflowV4Hook, error) {
	workflow, err := commonrepo.NewWorkflowV4Coll().Find(workflowName)
	if err != nil {
		logger.Errorf("Failed to find WorkflowV4: %s, the error is: %v", workflowName, err)
		return nil, e.ErrGetWebhook.AddErr(err)
	}
	var workflowArg *commonmodels.WorkflowV4
	workflowHook := &commonmodels.WorkflowV4Hook{}
	for _, hook := range workflow.HookCtls {
		if hook.Name == triggerName {
			workflowArg = hook.WorkflowArg
			workflowHook = hook
			break
		}
	}
	if err := job.MergeArgs(workflow, workflowArg); err != nil {
		errMsg := fmt.Sprintf("merge workflow args error: %v", err)
		log.Error(errMsg)
		return nil, e.ErrGetWebhook.AddDesc(errMsg)
	}
	repos, err := job.GetRepos(workflow)
	if err != nil {
		errMsg := fmt.Sprintf("get workflow webhook repos error: %v", err)
		log.Error(errMsg)
		return nil, e.ErrGetWebhook.AddDesc(errMsg)
	}
	workflowHook.Repos = repos
	workflowHook.WorkflowArg = workflow
	workflowHook.WorkflowArg.JiraHookCtls = nil
	workflowHook.WorkflowArg.MeegoHookCtls = nil
	workflowHook.WorkflowArg.GeneralHookCtls = nil
	workflowHook.WorkflowArg.HookCtls = nil
	return workflowHook, nil
}

func DeleteWebhookForWorkflowV4(workflowName, triggerName string, logger *zap.SugaredLogger) error {
	workflow, err := commonrepo.NewWorkflowV4Coll().Find(workflowName)
	if err != nil {
		logger.Errorf("Failed to find WorkflowV4: %s, the error is: %v", workflowName, err)
		return e.ErrDeleteWebhook.AddErr(err)
	}
	updatedHooks := []*commonmodels.WorkflowV4Hook{}
	var existHook *commonmodels.WorkflowV4Hook
	for _, hook := range workflow.HookCtls {
		if hook.Name == triggerName {
			existHook = hook
			continue
		}
		updatedHooks = append(updatedHooks, hook)
	}
	if existHook == nil {
		errMsg := fmt.Sprintf("webhook %s does not exist", triggerName)
		logger.Error(errMsg)
		return e.ErrDeleteWebhook.AddDesc(errMsg)
	}
	err = commonservice.ProcessWebhook(nil, []*models.WorkflowV4Hook{existHook}, webhook.WorkflowV4Prefix+workflowName, logger)
	if err != nil {
		errMsg := fmt.Sprintf("failed to delete webhook for workflow %s, the error is: %v", workflowName, err)
		log.Error(errMsg)
		return e.ErrDeleteWebhook.AddDesc(errMsg)
	}
	workflow.HookCtls = updatedHooks
	if err := commonrepo.NewWorkflowV4Coll().Update(workflow.ID.Hex(), workflow); err != nil {
		errMsg := fmt.Sprintf("failed to delete webhook for workflow %s, the error is: %v", workflowName, err)
		log.Error(errMsg)
		return e.ErrDeleteWebhook.AddDesc(errMsg)
	}
	if err := deleteGerritWebhook(existHook.MainRepo, workflowName); err != nil {
		logger.Errorf("delete gerrit webhook failed: %v", err)
	}
	return nil
}

func CreateGeneralHookForWorkflowV4(workflowName string, arg *models.GeneralHook, logger *zap.SugaredLogger) error {
	workflow, err := commonrepo.NewWorkflowV4Coll().Find(workflowName)
	if err != nil {
		logger.Errorf("Failed to find WorkflowV4: %s, the error is: %v", workflowName, err)
		return e.ErrCreateGeneralHook.AddErr(err)
	}
	for _, hook := range workflow.GeneralHookCtls {
		if hook.Name == arg.Name {
			errMsg := fmt.Sprintf("general hook %s already exists", arg.Name)
			logger.Error(errMsg)
			return e.ErrCreateGeneralHook.AddDesc(errMsg)
		}
	}
	if err := validateHookNames([]string{arg.Name}); err != nil {
		logger.Errorf(err.Error())
		return e.ErrCreateGeneralHook.AddErr(err)
	}
	workflow.GeneralHookCtls = append(workflow.GeneralHookCtls, arg)
	if err := commonrepo.NewWorkflowV4Coll().Update(workflow.ID.Hex(), workflow); err != nil {
		errMsg := fmt.Sprintf("failed to create general hook for workflow %s, the error is: %v", workflowName, err)
		log.Error(errMsg)
		return e.ErrCreateGeneralHook.AddDesc(errMsg)
	}
	return nil
}

func GetGeneralHookForWorkflowV4Preset(workflowName, hookName string, logger *zap.SugaredLogger) (*commonmodels.GeneralHook, error) {
	workflow, err := commonrepo.NewWorkflowV4Coll().Find(workflowName)
	if err != nil {
		logger.Errorf("Failed to find WorkflowV4: %s, the error is: %v", workflowName, err)
		return nil, e.ErrGetGeneralHook.AddErr(err)
	}
	gHook := &commonmodels.GeneralHook{}
	for _, hook := range workflow.GeneralHookCtls {
		if hook.Name == hookName {
			gHook = hook
		}
	}
	if err := job.MergeArgs(workflow, gHook.WorkflowArg); err != nil {
		errMsg := fmt.Sprintf("merge workflow args error: %v", err)
		log.Error(errMsg)
		return nil, e.ErrGetGeneralHook.AddDesc(errMsg)
	}
	gHook.WorkflowArg = workflow
	gHook.WorkflowArg.JiraHookCtls = nil
	gHook.WorkflowArg.MeegoHookCtls = nil
	gHook.WorkflowArg.GeneralHookCtls = nil
	gHook.WorkflowArg.HookCtls = nil
	return gHook, nil
}

func ListGeneralHookForWorkflowV4(workflowName string, logger *zap.SugaredLogger) ([]*models.GeneralHook, error) {
	workflow, err := commonrepo.NewWorkflowV4Coll().Find(workflowName)
	if err != nil {
		logger.Errorf("Failed to find WorkflowV4: %s, the error is: %v", workflowName, err)
		return nil, e.ErrListGeneralHook.AddErr(err)
	}
	return workflow.GeneralHookCtls, nil
}

func UpdateGeneralHookForWorkflowV4(workflowName string, arg *models.GeneralHook, logger *zap.SugaredLogger) error {
	workflow, err := commonrepo.NewWorkflowV4Coll().Find(workflowName)
	if err != nil {
		logger.Errorf("Failed to find WorkflowV4: %s, the error is: %v", workflowName, err)
		return e.ErrUpdateGeneralHook.AddErr(err)
	}
	updated := false
	for i, hook := range workflow.GeneralHookCtls {
		if hook.Name == arg.Name {
			workflow.GeneralHookCtls[i] = arg
			updated = true
		}
	}
	if !updated {
		errMsg := fmt.Sprintf("failed to find general hook %s", arg.Name)
		log.Error(errMsg)
		return e.ErrUpdateGeneralHook.AddDesc(errMsg)
	}
	if err := commonrepo.NewWorkflowV4Coll().Update(workflow.ID.Hex(), workflow); err != nil {
		errMsg := fmt.Sprintf("failed to update general hook for workflow %s, the error is: %v", workflowName, err)
		log.Error(errMsg)
		return e.ErrUpdateGeneralHook.AddDesc(errMsg)
	}
	return nil
}

func DeleteGeneralHookForWorkflowV4(workflowName, hookName string, logger *zap.SugaredLogger) error {
	workflow, err := commonrepo.NewWorkflowV4Coll().Find(workflowName)
	if err != nil {
		logger.Errorf("Failed to find WorkflowV4: %s, the error is: %v", workflowName, err)
		return e.ErrDeleteGeneralHook.AddErr(err)
	}
	var list []*models.GeneralHook
	for _, ctl := range workflow.GeneralHookCtls {
		if ctl.Name == hookName {
			continue
		}
		list = append(list, ctl)
	}
	if len(list) == len(workflow.GeneralHookCtls) {
		errMsg := fmt.Sprintf("general hook %s not found", hookName)
		log.Error(errMsg)
		return e.ErrDeleteGeneralHook.AddDesc(errMsg)
	}
	workflow.GeneralHookCtls = list
	if err := commonrepo.NewWorkflowV4Coll().Update(workflow.ID.Hex(), workflow); err != nil {
		errMsg := fmt.Sprintf("failed to delete general hook for workflow %s, the error is: %v", workflowName, err)
		log.Error(errMsg)
		return e.ErrDeleteGeneralHook.AddDesc(errMsg)
	}
	return nil
}

func GeneralHookEventHandler(workflowName, hookName string, logger *zap.SugaredLogger) error {
	workflowInfo, err := commonrepo.NewWorkflowV4Coll().Find(workflowName)
	if err != nil {
		errMsg := fmt.Sprintf("Failed to find WorkflowV4: %s, the error is: %v", workflowName, err)
		logger.Error(errMsg)
		return errors.New(errMsg)
	}
	var generalHook *models.GeneralHook
	for _, hook := range workflowInfo.GeneralHookCtls {
		if hook.Name == hookName {
			generalHook = hook
			break
		}
	}
	if generalHook == nil {
		errMsg := fmt.Sprintf("Failed to find general hook %s", hookName)
		logger.Error(errMsg)
		return errors.New(errMsg)
	}
	if !generalHook.Enabled {
		errMsg := fmt.Sprintf("Not enabled general hook %s", hookName)
		logger.Error(errMsg)
		return errors.New(errMsg)
	}
	_, err = CreateWorkflowTaskV4ByBuildInTrigger(setting.JiraHookTaskCreator, generalHook.WorkflowArg, logger)
	if err != nil {
		errMsg := fmt.Sprintf("HandleGeneralHookEvent: failed to create workflow task: %s", err)
		logger.Error(errMsg)
		return errors.New(errMsg)
	}
	logger.Infof("HandleGeneralHookEvent: workflow-%s hook-%s create workflow task success", workflowName, hookName)
	return nil
}

func CreateJiraHookForWorkflowV4(workflowName string, arg *models.JiraHook, logger *zap.SugaredLogger) error {
	workflow, err := commonrepo.NewWorkflowV4Coll().Find(workflowName)
	if err != nil {
		logger.Errorf("Failed to find WorkflowV4: %s, the error is: %v", workflowName, err)
		return e.ErrCreateJiraHook.AddErr(err)
	}
	for _, hook := range workflow.JiraHookCtls {
		if hook.Name == arg.Name {
			errMsg := fmt.Sprintf("jira hook %s already exists", arg.Name)
			logger.Error(errMsg)
			return e.ErrCreateJiraHook.AddDesc(errMsg)
		}
	}
	if err := validateHookNames([]string{arg.Name}); err != nil {
		logger.Errorf(err.Error())
		return e.ErrCreateJiraHook.AddErr(err)
	}
	workflow.JiraHookCtls = append(workflow.JiraHookCtls, arg)
	if err := commonrepo.NewWorkflowV4Coll().Update(workflow.ID.Hex(), workflow); err != nil {
		errMsg := fmt.Sprintf("failed to create jira hook for workflow %s, the error is: %v", workflowName, err)
		log.Error(errMsg)
		return e.ErrCreateJiraHook.AddDesc(errMsg)
	}
	return nil
}

func GetJiraHookForWorkflowV4Preset(workflowName, hookName string, logger *zap.SugaredLogger) (*commonmodels.JiraHook, error) {
	workflow, err := commonrepo.NewWorkflowV4Coll().Find(workflowName)
	if err != nil {
		logger.Errorf("Failed to find WorkflowV4: %s, the error is: %v", workflowName, err)
		return nil, e.ErrGetJiraHook.AddErr(err)
	}
	jiraHook := &commonmodels.JiraHook{}
	for _, hook := range workflow.JiraHookCtls {
		if hook.Name == hookName {
			jiraHook = hook
		}
	}
	if err := job.MergeArgs(workflow, jiraHook.WorkflowArg); err != nil {
		errMsg := fmt.Sprintf("merge workflow args error: %v", err)
		log.Error(errMsg)
		return nil, e.ErrGetJiraHook.AddDesc(errMsg)
	}
	jiraHook.WorkflowArg = workflow
	jiraHook.WorkflowArg.JiraHookCtls = nil
	jiraHook.WorkflowArg.MeegoHookCtls = nil
	jiraHook.WorkflowArg.GeneralHookCtls = nil
	jiraHook.WorkflowArg.HookCtls = nil
	return jiraHook, nil
}

func ListJiraHookForWorkflowV4(workflowName string, logger *zap.SugaredLogger) ([]*models.JiraHook, error) {
	workflow, err := commonrepo.NewWorkflowV4Coll().Find(workflowName)
	if err != nil {
		logger.Errorf("Failed to find WorkflowV4: %s, the error is: %v", workflowName, err)
		return nil, e.ErrListJiraHook.AddErr(err)
	}
	return workflow.JiraHookCtls, nil
}

func UpdateJiraHookForWorkflowV4(workflowName string, arg *models.JiraHook, logger *zap.SugaredLogger) error {
	workflow, err := commonrepo.NewWorkflowV4Coll().Find(workflowName)
	if err != nil {
		logger.Errorf("Failed to find WorkflowV4: %s, the error is: %v", workflowName, err)
		return e.ErrUpdateJiraHook.AddErr(err)
	}
	updated := false
	for i, hook := range workflow.JiraHookCtls {
		if hook.Name == arg.Name {
			workflow.JiraHookCtls[i] = arg
			updated = true
		}
	}
	if !updated {
		errMsg := fmt.Sprintf("failed to find jira hook %s", arg.Name)
		log.Error(errMsg)
		return e.ErrUpdateJiraHook.AddDesc(errMsg)
	}
	if err := commonrepo.NewWorkflowV4Coll().Update(workflow.ID.Hex(), workflow); err != nil {
		errMsg := fmt.Sprintf("failed to update jira hook for workflow %s, the error is: %v", workflowName, err)
		log.Error(errMsg)
		return e.ErrUpdateJiraHook.AddDesc(errMsg)
	}
	return nil
}

func DeleteJiraHookForWorkflowV4(workflowName, hookName string, logger *zap.SugaredLogger) error {
	workflow, err := commonrepo.NewWorkflowV4Coll().Find(workflowName)
	if err != nil {
		logger.Errorf("Failed to find WorkflowV4: %s, the error is: %v", workflowName, err)
		return e.ErrDeleteJiraHook.AddErr(err)
	}
	var list []*models.JiraHook
	for _, ctl := range workflow.JiraHookCtls {
		if ctl.Name == hookName {
			continue
		}
		list = append(list, ctl)
	}
	if len(list) == len(workflow.JiraHookCtls) {
		errMsg := fmt.Sprintf("jira hook %s not found", hookName)
		log.Error(errMsg)
		return e.ErrDeleteJiraHook.AddDesc(errMsg)
	}
	workflow.JiraHookCtls = list
	if err := commonrepo.NewWorkflowV4Coll().Update(workflow.ID.Hex(), workflow); err != nil {
		errMsg := fmt.Sprintf("failed to delete jira hook for workflow %s, the error is: %v", workflowName, err)
		log.Error(errMsg)
		return e.ErrDeleteJiraHook.AddDesc(errMsg)
	}
	return nil
}

func CreateMeegoHookForWorkflowV4(workflowName string, arg *models.MeegoHook, logger *zap.SugaredLogger) error {
	workflow, err := commonrepo.NewWorkflowV4Coll().Find(workflowName)
	if err != nil {
		logger.Errorf("Failed to find WorkflowV4: %s, the error is: %v", workflowName, err)
		return e.ErrCreateMeegoHook.AddErr(err)
	}
	for _, hook := range workflow.MeegoHookCtls {
		if hook.Name == arg.Name {
			errMsg := fmt.Sprintf("meego hook %s already exists", arg.Name)
			logger.Error(errMsg)
			return e.ErrCreateMeegoHook.AddDesc(errMsg)
		}
	}
	if err := validateHookNames([]string{arg.Name}); err != nil {
		logger.Errorf(err.Error())
		return e.ErrCreateMeegoHook.AddErr(err)
	}
	workflow.MeegoHookCtls = append(workflow.MeegoHookCtls, arg)
	if err := commonrepo.NewWorkflowV4Coll().Update(workflow.ID.Hex(), workflow); err != nil {
		errMsg := fmt.Sprintf("failed to create jira hook for workflow %s, the error is: %v", workflowName, err)
		log.Error(errMsg)
		return e.ErrCreateMeegoHook.AddDesc(errMsg)
	}
	return nil
}

func GetMeegoHookForWorkflowV4Preset(workflowName, hookName string, logger *zap.SugaredLogger) (*commonmodels.MeegoHook, error) {
	workflow, err := commonrepo.NewWorkflowV4Coll().Find(workflowName)
	if err != nil {
		logger.Errorf("Failed to find WorkflowV4: %s, the error is: %v", workflowName, err)
		return nil, e.ErrGetMeegoHook.AddErr(err)
	}
	meegoHook := &commonmodels.MeegoHook{}
	for _, hook := range workflow.MeegoHookCtls {
		if hook.Name == hookName {
			meegoHook = hook
		}
	}
	if err := job.MergeArgs(workflow, meegoHook.WorkflowArg); err != nil {
		errMsg := fmt.Sprintf("merge workflow args error: %v", err)
		log.Error(errMsg)
		return nil, e.ErrGetMeegoHook.AddDesc(errMsg)
	}
	meegoHook.WorkflowArg = workflow
	meegoHook.WorkflowArg.JiraHookCtls = nil
	meegoHook.WorkflowArg.MeegoHookCtls = nil
	meegoHook.WorkflowArg.GeneralHookCtls = nil
	meegoHook.WorkflowArg.HookCtls = nil
	return meegoHook, nil
}

func ListMeegoHookForWorkflowV4(workflowName string, logger *zap.SugaredLogger) ([]*models.MeegoHook, error) {
	workflow, err := commonrepo.NewWorkflowV4Coll().Find(workflowName)
	if err != nil {
		logger.Errorf("Failed to find WorkflowV4: %s, the error is: %v", workflowName, err)
		return nil, e.ErrListMeegoHook.AddErr(err)
	}
	return workflow.MeegoHookCtls, nil
}

func UpdateMeegoHookForWorkflowV4(workflowName string, arg *models.MeegoHook, logger *zap.SugaredLogger) error {
	workflow, err := commonrepo.NewWorkflowV4Coll().Find(workflowName)
	if err != nil {
		logger.Errorf("Failed to find WorkflowV4: %s, the error is: %v", workflowName, err)
		return e.ErrUpdateMeegoHook.AddErr(err)
	}
	updated := false
	for i, hook := range workflow.MeegoHookCtls {
		if hook.Name == arg.Name {
			workflow.MeegoHookCtls[i] = arg
			updated = true
		}
	}
	if !updated {
		errMsg := fmt.Sprintf("failed to find jira hook %s", arg.Name)
		log.Error(errMsg)
		return e.ErrUpdateMeegoHook.AddDesc(errMsg)
	}
	if err := commonrepo.NewWorkflowV4Coll().Update(workflow.ID.Hex(), workflow); err != nil {
		errMsg := fmt.Sprintf("failed to update jira hook for workflow %s, the error is: %v", workflowName, err)
		log.Error(errMsg)
		return e.ErrUpdateMeegoHook.AddDesc(errMsg)
	}
	return nil
}

func DeleteMeegoHookForWorkflowV4(workflowName, hookName string, logger *zap.SugaredLogger) error {
	workflow, err := commonrepo.NewWorkflowV4Coll().Find(workflowName)
	if err != nil {
		logger.Errorf("Failed to find WorkflowV4: %s, the error is: %v", workflowName, err)
		return e.ErrDeleteMeegoHook.AddErr(err)
	}
	var list []*models.MeegoHook
	for _, ctl := range workflow.MeegoHookCtls {
		if ctl.Name == hookName {
			continue
		}
		list = append(list, ctl)
	}
	if len(list) == len(workflow.MeegoHookCtls) {
		errMsg := fmt.Sprintf("meego hook %s not found", hookName)
		log.Error(errMsg)
		return e.ErrDeleteMeegoHook.AddDesc(errMsg)
	}
	workflow.MeegoHookCtls = list
	if err := commonrepo.NewWorkflowV4Coll().Update(workflow.ID.Hex(), workflow); err != nil {
		errMsg := fmt.Sprintf("failed to delete jira hook for workflow %s, the error is: %v", workflowName, err)
		log.Error(errMsg)
		return e.ErrDeleteMeegoHook.AddDesc(errMsg)
	}
	return nil
}

func BulkCopyWorkflowV4(args BulkCopyWorkflowArgs, username string, log *zap.SugaredLogger) error {
	var workflows []commonrepo.WorkflowV4

	for _, item := range args.Items {
		workflows = append(workflows, commonrepo.WorkflowV4{
			ProjectName: item.ProjectName,
			Name:        item.Old,
		})
	}
	oldWorkflows, err := commonrepo.NewWorkflowV4Coll().ListByWorkflows(commonrepo.ListWorkflowV4Opt{
		Workflows: workflows,
	})
	if err != nil {
		log.Error(err)
		return e.ErrGetPipeline.AddErr(err)
	}
	workflowMap := make(map[string]*commonmodels.WorkflowV4)
	for _, workflow := range oldWorkflows {
		workflowMap[workflow.Project+"-"+workflow.Name] = workflow
	}
	var newWorkflows []*commonmodels.WorkflowV4
	for _, workflow := range args.Items {
		if item, ok := workflowMap[workflow.ProjectName+"-"+workflow.Old]; ok {
			newItem := *item
			newItem.UpdatedBy = username
			newItem.Name = workflow.New
			newItem.DisplayName = workflow.NewDisplayName
			newItem.BaseName = workflow.BaseName
			newItem.ID = primitive.NewObjectID()
			// do not copy webhook triggers.
			newItem.HookCtls = []*commonmodels.WorkflowV4Hook{}

			newWorkflows = append(newWorkflows, &newItem)
		} else {
			return fmt.Errorf("workflow:%s not exist", item.Project+"-"+item.Name)
		}
	}
	return commonrepo.NewWorkflowV4Coll().BulkCreate(newWorkflows)
}

func CreateCronForWorkflowV4(workflowName string, input *commonmodels.Cronjob, logger *zap.SugaredLogger) error {
	if !input.ID.IsZero() {
		return e.ErrUpsertCronjob.AddDesc("cronjob id is not empty")
	}
	input.Name = workflowName
	input.Type = config.WorkflowV4Cronjob
	err := commonrepo.NewCronjobColl().Create(input)
	if err != nil {
		msg := fmt.Sprintf("Failed to create cron job, error: %v", err)
		log.Error(msg)
		return errors.New(msg)
	}
	if !input.Enabled {
		return nil
	}

	payload := &commonservice.CronjobPayload{
		Name:    workflowName,
		JobType: config.WorkflowV4Cronjob,
		Action:  setting.TypeEnableCronjob,
		JobList: []*commonmodels.Schedule{cronJobToSchedule(input)},
	}

	pl, _ := json.Marshal(payload)
	if err := nsq.Publish(setting.TopicCronjob, pl); err != nil {
		log.Errorf("Failed to publish to nsq topic: %s, the error is: %v", setting.TopicCronjob, err)
		return e.ErrUpsertCronjob.AddDesc(err.Error())
	}
	return nil
}

func UpdateCronForWorkflowV4(input *commonmodels.Cronjob, logger *zap.SugaredLogger) error {
	_, err := commonrepo.NewCronjobColl().GetByID(input.ID)
	if err != nil {
		msg := fmt.Sprintf("cron job not exist, error: %v", err)
		log.Error(msg)
		return errors.New(msg)
	}
	if err := commonrepo.NewCronjobColl().Update(input); err != nil {
		msg := fmt.Sprintf("Failed to update cron job, error: %v", err)
		log.Error(msg)
		return errors.New(msg)
	}
	payload := &commonservice.CronjobPayload{
		Name:    input.Name,
		JobType: config.WorkflowV4Cronjob,
		Action:  setting.TypeEnableCronjob,
	}
	if !input.Enabled {
		payload.DeleteList = []string{input.ID.Hex()}
	} else {
		payload.JobList = []*commonmodels.Schedule{cronJobToSchedule(input)}
	}

	pl, _ := json.Marshal(payload)
	if err := nsq.Publish(setting.TopicCronjob, pl); err != nil {
		log.Errorf("Failed to publish to nsq topic: %s, the error is: %v", setting.TopicCronjob, err)
		return e.ErrUpsertCronjob.AddDesc(err.Error())
	}
	return nil
}

func ListCronForWorkflowV4(workflowName string, logger *zap.SugaredLogger) ([]*commonmodels.Cronjob, error) {
	crons, err := commonrepo.NewCronjobColl().List(&commonrepo.ListCronjobParam{
		ParentName: workflowName,
		ParentType: config.WorkflowV4Cronjob,
	})
	if err != nil {
		logger.Errorf("Failed to list WorkflowV4 : %s cron jobs, the error is: %v", workflowName, err)
		return crons, e.ErrUpsertCronjob.AddErr(err)
	}
	return crons, nil
}

func GetCronForWorkflowV4Preset(workflowName, cronID string, logger *zap.SugaredLogger) (*commonmodels.Cronjob, error) {
	workflow, err := commonrepo.NewWorkflowV4Coll().Find(workflowName)
	if err != nil {
		logger.Errorf("Failed to find WorkflowV4: %s, the error is: %v", workflowName, err)
		return nil, e.ErrUpsertCronjob.AddErr(err)
	}
	cronJob := &commonmodels.Cronjob{}
	if cronID != "" {
		id, err := primitive.ObjectIDFromHex(cronID)
		if err != nil {
			logger.Errorf("Failed to parse cron id: %s, the error is: %v", cronID, err)
			return nil, e.ErrUpsertCronjob.AddErr(err)
		}
		cronJob, err = commonrepo.NewCronjobColl().GetByID(id)
		if err != nil {
			msg := fmt.Sprintf("cron job not exist, error: %v", err)
			log.Error(msg)
			return nil, errors.New(msg)
		}
	}

	if err := job.MergeArgs(workflow, cronJob.WorkflowV4Args); err != nil {
		errMsg := fmt.Sprintf("merge workflow args error: %v", err)
		log.Error(errMsg)
		return nil, e.ErrGetWebhook.AddDesc(errMsg)
	}
	cronJob.WorkflowV4Args = workflow
	return cronJob, nil
}

func DeleteCronForWorkflowV4(workflowName, cronID string, logger *zap.SugaredLogger) error {
	id, err := primitive.ObjectIDFromHex(cronID)
	if err != nil {
		logger.Errorf("Failed to parse cron id: %s, the error is: %v", cronID, err)
		return e.ErrUpsertCronjob.AddErr(err)
	}
	job, err := commonrepo.NewCronjobColl().GetByID(id)
	if err != nil {
		msg := fmt.Sprintf("cron job not exist, error: %v", err)
		log.Error(msg)
		return errors.New(msg)
	}
	payload := &commonservice.CronjobPayload{
		Name:       workflowName,
		JobType:    config.WorkflowV4Cronjob,
		Action:     setting.TypeEnableCronjob,
		DeleteList: []string{job.ID.Hex()},
	}

	pl, _ := json.Marshal(payload)
	if err := nsq.Publish(setting.TopicCronjob, pl); err != nil {
		log.Errorf("Failed to publish to nsq topic: %s, the error is: %v", setting.TopicCronjob, err)
		return e.ErrUpsertCronjob.AddDesc(err.Error())
	}
	if err := commonrepo.NewCronjobColl().Delete(&commonrepo.CronjobDeleteOption{IDList: []string{cronID}}); err != nil {
		logger.Errorf("Failed to delete cron job: %s, the error is: %v", cronID, err)
		return e.ErrUpsertCronjob.AddDesc(err.Error())
	}
	return nil
}

func cronJobToSchedule(input *commonmodels.Cronjob) *commonmodels.Schedule {
	return &commonmodels.Schedule{
		ID:             input.ID,
		Number:         input.Number,
		Frequency:      input.Frequency,
		Time:           input.Time,
		MaxFailures:    input.MaxFailure,
		WorkflowV4Args: input.WorkflowV4Args,
		Type:           config.ScheduleType(input.JobType),
		Cron:           input.Cron,
		Enabled:        input.Enabled,
	}
}

func GetPatchParams(patchItem *commonmodels.PatchItem, logger *zap.SugaredLogger) ([]*commonmodels.Param, error) {
	resp := []*commonmodels.Param{}
	kvs, err := commomtemplate.GetYamlVariables(patchItem.PatchContent, logger)
	if err != nil {
		return resp, fmt.Errorf("get kv from content error: %s", err)
	}
	paramMap := map[string]*commonmodels.Param{}
	for _, param := range patchItem.Params {
		paramMap[param.Name] = param
	}
	for _, kv := range kvs {
		if param, ok := paramMap[kv.Key]; ok {
			resp = append(resp, param)
			continue
		}
		resp = append(resp, &commonmodels.Param{
			Name:       kv.Key,
			ParamsType: "string",
		})
	}
	return resp, nil
}

func GetWorkflowGlabalVars(workflow *commonmodels.WorkflowV4, currentJobName string, log *zap.SugaredLogger) []string {
	return append(getDefaultVars(workflow), jobctl.GetWorkflowOutputs(workflow, currentJobName, log)...)
}

func getDefaultVars(workflow *commonmodels.WorkflowV4) []string {
	vars := []string{}
	vars = append(vars, fmt.Sprintf(setting.RenderValueTemplate, "project"))
	vars = append(vars, fmt.Sprintf(setting.RenderValueTemplate, "workflow.name"))
	vars = append(vars, fmt.Sprintf(setting.RenderValueTemplate, "workflow.task.creator"))
	vars = append(vars, fmt.Sprintf(setting.RenderValueTemplate, "workflow.task.timestamp"))
	for _, param := range workflow.Params {
		vars = append(vars, fmt.Sprintf(setting.RenderValueTemplate, strings.Join([]string{"workflow", "params", param.Name}, ".")))
	}
	return vars
}

func CheckShareStorageEnabled(clusterID, jobType, identifyName, project string, logger *zap.SugaredLogger) (bool, error) {
	// if cluster id was set, we just check if the cluster has share storage enabled
	if clusterID != "" {
		return checkClusterShareStorage(clusterID)
	}
	switch jobType {
	case string(config.JobZadigBuild):
		build, err := commonrepo.NewBuildColl().Find(&commonrepo.BuildFindOption{Name: identifyName, ProductName: project})
		if err != nil {
			return false, fmt.Errorf("find build error: %v", err)
		}
		if build.TemplateID == "" {
			clusterID = build.PreBuild.ClusterID
			break
		}
		template, err := commonrepo.NewBuildTemplateColl().Find(&commonrepo.BuildTemplateQueryOption{ID: build.TemplateID})
		if err != nil {
			return false, fmt.Errorf("find build template error: %v", err)
		}
		clusterID = template.PreBuild.ClusterID
	case string(config.JobZadigTesting):
		testing, err := commonrepo.NewTestingColl().Find(identifyName, "")
		if err != nil {
			return false, fmt.Errorf("find testing error: %v", err)
		}
		clusterID = testing.PreTest.ClusterID
	case string(config.JobZadigScanning):
		scanning, err := commonrepo.NewScanningColl().Find(project, identifyName)
		if err != nil {
			return false, fmt.Errorf("find scanning error: %v", err)
		}
		clusterID = scanning.AdvancedSetting.ClusterID
	default:
		return false, fmt.Errorf("job type %s is not supported", jobType)
	}
	if clusterID == "" {
		clusterID = setting.LocalClusterID
	}
	return checkClusterShareStorage(clusterID)
}

func checkClusterShareStorage(id string) (bool, error) {
	cluster, err := commonrepo.NewK8SClusterColl().Get(id)
	if err != nil {
		return false, fmt.Errorf("find cluter error: %v", err)
	}
	if cluster.ShareStorage.NFSProperties.PVC != "" {
		return true, nil
	}
	return false, nil
}

func ListAllAvailableWorkflows(projects []string, log *zap.SugaredLogger) ([]*Workflow, error) {
	resp := make([]*Workflow, 0)
	allProductWorkflows, err := commonrepo.NewWorkflowColl().ListWorkflowsByProjects(projects)
	if err != nil {
		log.Errorf("failed to get all product workflows, error: %s", err)
		return nil, err
	}
	for _, productWorkflow := range allProductWorkflows {
		resp = append(resp, &Workflow{
			Name:         productWorkflow.Name,
			DisplayName:  productWorkflow.DisplayName,
			ProjectName:  productWorkflow.ProductTmplName,
			UpdateTime:   productWorkflow.UpdateTime,
			CreateTime:   productWorkflow.CreateTime,
			UpdateBy:     productWorkflow.UpdateBy,
			WorkflowType: "",
			Description:  productWorkflow.Description,
			BaseName:     productWorkflow.BaseName,
		})
	}

	allCustomWorkflows, err := commonrepo.NewWorkflowV4Coll().ListByProjectNames(projects)
	if err != nil {
		log.Errorf("failed to get all custom workflows, error: %s", err)
		return nil, err
	}

	for _, customWorkflow := range allCustomWorkflows {
		resp = append(resp, &Workflow{
			Name:         customWorkflow.Name,
			DisplayName:  customWorkflow.DisplayName,
			ProjectName:  customWorkflow.Project,
			UpdateTime:   customWorkflow.UpdateTime,
			CreateTime:   customWorkflow.CreateTime,
			UpdateBy:     customWorkflow.UpdatedBy,
			WorkflowType: setting.CustomWorkflowType,
			Description:  customWorkflow.Description,
			BaseName:     customWorkflow.BaseName,
		})
	}

	return resp, nil
}

func GetLatestTaskInfo(workflowInfo *Workflow) (startTime int64, creator, status string) {
	// if we found it is a custom workflow, search it in the custom workflow task
	if workflowInfo.WorkflowType == setting.CustomWorkflowType {
		taskInfo, err := getLatestWorkflowTaskV4(workflowInfo.Name)
		if err != nil {
			return 0, "", ""
		}
		return taskInfo.StartTime, taskInfo.TaskCreator, string(taskInfo.Status)
	} else {
		// otherwise it is a product workflow
		taskInfo, err := getLatestWorkflowTask(workflowInfo.Name)
		if err != nil {
			return 0, "", ""
		}
		return taskInfo.StartTime, taskInfo.TaskCreator, string(taskInfo.Status)
	}
}
