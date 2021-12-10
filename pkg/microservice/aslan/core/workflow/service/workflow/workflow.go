/*
Copyright 2021 The KodeRover Authors.

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
	"fmt"
	"strings"
	"sync"

	"github.com/hashicorp/go-multierror"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb/template"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/webhook"
	"github.com/koderover/zadig/pkg/setting"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/types"
)

var mut sync.Mutex

type EnvStatus struct {
	EnvName    string `json:"env_name,omitempty"`
	Status     string `json:"status"`
	ErrMessage string `json:"err_message"`
}

type Workflow struct {
	Name                 string                     `json:"name"`
	ProjectName          string                     `json:"projectName"`
	UpdateTime           int64                      `json:"updateTime"`
	CreateTime           int64                      `json:"createTime"`
	UpdateBy             string                     `json:"updateBy,omitempty"`
	Schedules            *commonmodels.ScheduleCtrl `json:"schedules,omitempty"`
	SchedulerEnabled     bool                       `json:"schedulerEnabled"`
	EnabledStages        []string                   `json:"enabledStages"`
	IsFavorite           bool                       `json:"isFavorite"`
	RecentTask           *TaskInfo                  `json:"recentTask"`
	RecentSuccessfulTask *TaskInfo                  `json:"recentSuccessfulTask"`
	RecentFailedTask     *TaskInfo                  `json:"recentFailedTask"`
	AverageExecutionTime float64                    `json:"averageExecutionTime"`
	SuccessRate          float64                    `json:"successRate"`
	Description          string                     `json:"description,omitempty"`
}

type TaskInfo struct {
	TaskID       int64  `json:"taskID"`
	PipelineName string `json:"pipelineName"`
	Status       string `json:"status"`
}

type workflowCreateArg struct {
	name                 string
	envName              string
	buildStageEnabled    bool
	ArtifactStageEnabled bool
}

type workflowCreateArgs struct {
	productName string
	argsMap     map[string]*workflowCreateArg
}

func (args *workflowCreateArgs) addWorkflowArg(envName string, buildStageEnabled, artifactStageEnabled bool) {
	wName := fmt.Sprintf("%s-workflow-%s", args.productName, envName)
	if artifactStageEnabled {
		wName = fmt.Sprintf("%s-%s-workflow", args.productName, "ops")
	}
	// The hosting env workflow name is not bound to the environment
	if !artifactStageEnabled && envName == "" {
		wName = fmt.Sprintf("%s-workflow", args.productName)
	}
	args.argsMap[wName] = &workflowCreateArg{
		name:                 wName,
		envName:              envName,
		buildStageEnabled:    buildStageEnabled,
		ArtifactStageEnabled: artifactStageEnabled,
	}
}

func (args *workflowCreateArgs) initDefaultWorkflows() {
	args.addWorkflowArg("dev", true, false)
	args.addWorkflowArg("qa", true, false)
	args.addWorkflowArg("", false, true)
}

func (args *workflowCreateArgs) clear() {
	args.argsMap = make(map[string]*workflowCreateArg)
}

func AutoCreateWorkflow(productName string, log *zap.SugaredLogger) *EnvStatus {
	productTmpl, err := template.NewProductColl().Find(productName)
	if err != nil {
		errMsg := fmt.Sprintf("[ProductTmpl.Find] %s error: %v", productName, err)
		log.Error(errMsg)
		return &EnvStatus{Status: setting.ProductStatusFailed, ErrMessage: errMsg}
	}
	errList := new(multierror.Error)
	mut.Lock()
	defer func() {
		mut.Unlock()
	}()

	createArgs := &workflowCreateArgs{
		productName: productName,
		argsMap:     make(map[string]*workflowCreateArg),
	}
	createArgs.initDefaultWorkflows()

	// helm project may have customized products, use the real created products
	if productTmpl.ProductFeature != nil && productTmpl.ProductFeature.DeployType == setting.HelmDeployType {
		productList, err := commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{
			Name: productName,
		})
		if err != nil {
			log.Errorf("fialed to list products, projectName %s, err %s", productName, err)
		}
		createArgs.clear()
		for _, product := range productList {
			createArgs.addWorkflowArg(product.EnvName, true, false)
		}
		createArgs.addWorkflowArg("", false, true)
	}

	// Only one workflow is created in the hosting environment
	if productTmpl.ProductFeature != nil && productTmpl.ProductFeature.CreateEnvType == setting.SourceFromExternal {
		createArgs.clear()
		createArgs.addWorkflowArg("", true, false)
	}

	workflowSlice := sets.NewString()
	for workflowName, _ := range createArgs.argsMap {
		_, err := FindWorkflow(workflowName, log)
		if err == nil {
			workflowSlice.Insert(workflowName)
		}
	}

	if len(workflowSlice) < len(createArgs.argsMap) {
		preSetResps, err := PreSetWorkflow(productName, log)
		if err != nil {
			errList = multierror.Append(errList, err)
		}
		buildModules := make([]*commonmodels.BuildModule, 0)
		artifactModules := make([]*commonmodels.ArtifactModule, 0)
		for _, preSetResp := range preSetResps {
			buildModule := &commonmodels.BuildModule{
				Target:         preSetResp.Target,
				BuildModuleVer: setting.Version,
			}
			buildModules = append(buildModules, buildModule)

			artifactModule := &commonmodels.ArtifactModule{
				Target: preSetResp.Target,
			}
			artifactModules = append(artifactModules, artifactModule)
		}

		for workflowName, workflowArg := range createArgs.argsMap {
			if workflowSlice.Has(workflowName) {
				continue
			}
			if _, err := commonrepo.NewWorkflowColl().Find(workflowName); err == nil {
				errList = multierror.Append(errList, fmt.Errorf("workflow [%s] 在项目 [%s] 中已经存在", workflowName, productName))
			}
			workflow := new(commonmodels.Workflow)
			workflow.Enabled = true
			workflow.ProductTmplName = productName
			workflow.Name = workflowName
			workflow.CreateBy = setting.SystemUser
			workflow.UpdateBy = setting.SystemUser
			workflow.EnvName = workflowArg.envName
			workflow.BuildStage = &commonmodels.BuildStage{
				Enabled: workflowArg.buildStageEnabled,
				Modules: buildModules,
			}

			//如果是开启artifactStage，则关闭buildStage
			if workflowArg.ArtifactStageEnabled {
				workflow.ArtifactStage = &commonmodels.ArtifactStage{
					Enabled: true,
					Modules: artifactModules,
				}
				workflow.EnvName = "ops" //TODO necessary to set a fake env name?
			}

			workflow.Schedules = &commonmodels.ScheduleCtrl{
				Enabled: false,
				Items:   []*commonmodels.Schedule{},
			}
			workflow.TestStage = &commonmodels.TestStage{
				Enabled:   false,
				TestNames: []string{},
			}
			workflow.NotifyCtl = &commonmodels.NotifyCtl{
				Enabled:       false,
				NotifyTypes:   []string{},
				WeChatWebHook: "",
			}
			workflow.HookCtl = &commonmodels.WorkflowHookCtrl{
				Enabled: false,
				Items:   []*commonmodels.WorkflowHook{},
			}
			workflow.DistributeStage = &commonmodels.DistributeStage{
				Enabled:     false,
				S3StorageID: "",
				ImageRepo:   "",
				JumpBoxHost: "",
				Releases:    []commonmodels.RepoImage{},
				Distributes: []*commonmodels.ProductDistribute{},
			}

			if err := commonrepo.NewWorkflowColl().Create(workflow); err != nil {
				errList = multierror.Append(errList, err)
			}
		}
		if err = errList.ErrorOrNil(); err != nil {
			return &EnvStatus{Status: setting.ProductStatusFailed, ErrMessage: err.Error()}
		}
		return &EnvStatus{Status: setting.ProductStatusCreating}
	} else if len(workflowSlice) == len(createArgs.argsMap) {
		return &EnvStatus{Status: setting.ProductStatusSuccess}
	}
	return nil
}

func FindWorkflow(workflowName string, log *zap.SugaredLogger) (*commonmodels.Workflow, error) {
	resp, err := commonrepo.NewWorkflowColl().Find(workflowName)
	if err != nil {
		log.Errorf("Workflow.Find error: %v", err)
		return resp, e.ErrFindWorkflow.AddDesc(err.Error())
	}
	if resp.Schedules == nil {
		schedules, err := commonrepo.NewCronjobColl().List(&commonrepo.ListCronjobParam{
			ParentName: resp.Name,
			ParentType: config.WorkflowCronjob,
		})
		if err != nil {
			log.Errorf("cannot list cron job list, the error is: %v", err)
			return nil, e.ErrFindWorkflow.AddDesc(err.Error())
		}

		scheduleList := []*commonmodels.Schedule{}
		for _, v := range schedules {
			scheduleList = append(scheduleList, &commonmodels.Schedule{
				ID:           v.ID,
				Number:       v.Number,
				Frequency:    v.Frequency,
				Time:         v.Time,
				MaxFailures:  v.MaxFailure,
				TaskArgs:     v.TaskArgs,
				WorkflowArgs: v.WorkflowArgs,
				TestArgs:     v.TestArgs,
				Type:         config.ScheduleType(v.JobType),
				Cron:         v.Cron,
				Enabled:      v.Enabled,
			})
		}
		schedule := commonmodels.ScheduleCtrl{
			Enabled: resp.ScheduleEnabled,
			Items:   scheduleList,
		}
		resp.Schedules = &schedule
	}
	return resp, nil
}

type PreSetResp struct {
	Target          *commonmodels.ServiceModuleTarget `json:"target"`
	BuildModuleVers []string                          `json:"build_module_vers"`
	Deploy          []DeployEnv                       `json:"deploy"`
	Repos           []*types.Repository               `json:"repos"`
}

type DeployEnv struct {
	Env         string `json:"env"`
	Type        string `json:"type"`
	ProductName string `json:"product_name,omitempty"`
}

func PreSetWorkflow(productName string, log *zap.SugaredLogger) ([]*PreSetResp, error) {
	resp := make([]*PreSetResp, 0)
	targets := make(map[string][]DeployEnv)
	productTmpl, err := template.NewProductColl().Find(productName)
	if err != nil {
		log.Errorf("[%s] ProductTmpl.Find error: %v", productName, err)
		return resp, e.ErrGetTemplate.AddDesc(err.Error())
	}
	services, err := commonrepo.NewServiceColl().ListMaxRevisionsForServices(productTmpl.AllServiceInfos(), "")
	if err != nil {
		log.Errorf("ServiceTmpl.ListMaxRevisions error: %v", err)
		return resp, e.ErrListTemplate.AddDesc(err.Error())
	}

	for _, serviceTmpl := range services {
		switch serviceTmpl.Type {
		case setting.K8SDeployType:
			for _, container := range serviceTmpl.Containers {
				deployEnv := DeployEnv{Env: serviceTmpl.ServiceName + "/" + container.Name, Type: setting.K8SDeployType, ProductName: serviceTmpl.ProductName}
				target := fmt.Sprintf("%s%s%s%s%s", serviceTmpl.ProductName, SplitSymbol, serviceTmpl.ServiceName, SplitSymbol, container.Name)
				targets[target] = append(targets[target], deployEnv)
			}
		case setting.PMDeployType:
			deployEnv := DeployEnv{Env: serviceTmpl.ServiceName, Type: setting.PMDeployType, ProductName: productName}
			target := fmt.Sprintf("%s%s%s%s%s", serviceTmpl.ProductName, SplitSymbol, serviceTmpl.ServiceName, SplitSymbol, serviceTmpl.ServiceName)
			targets[target] = append(targets[target], deployEnv)
		case setting.HelmDeployType:
			for _, container := range serviceTmpl.Containers {
				deployEnv := DeployEnv{Env: serviceTmpl.ServiceName + "/" + container.Name, Type: setting.HelmDeployType, ProductName: serviceTmpl.ProductName}
				target := fmt.Sprintf("%s%s%s%s%s", serviceTmpl.ProductName, SplitSymbol, serviceTmpl.ServiceName, SplitSymbol, container.Name)
				targets[target] = append(targets[target], deployEnv)
			}
		}
	}

	moList, err := commonrepo.NewBuildColl().List(&commonrepo.BuildListOption{})
	if err != nil {
		log.Errorf("[Build.List] error: %v", err)
		return nil, e.ErrListBuildModule.AddErr(err)
	}
	for k, v := range targets {
		// 选择了一个特殊字符在项目名称、服务名称以及服务组件名称里面都不允许的特殊字符，避免出现异常
		targetArr := strings.Split(k, SplitSymbol)
		if len(targetArr) != 3 {
			continue
		}

		preSet := &PreSetResp{
			Target: &commonmodels.ServiceModuleTarget{
				ProductName:   targetArr[0],
				ServiceName:   targetArr[1],
				ServiceModule: targetArr[2],
			},
			Deploy:          v,
			BuildModuleVers: []string{},
			Repos:           make([]*types.Repository, 0),
		}

		for _, mo := range moList {
			for _, moTarget := range mo.Targets {
				moduleTargetStr := fmt.Sprintf("%s%s%s%s%s", moTarget.ProductName, SplitSymbol, moTarget.ServiceName, SplitSymbol, moTarget.ServiceModule)
				if moduleTargetStr == k {
					if len(mo.Repos) == 0 {
						preSet.Repos = make([]*types.Repository, 0)
					} else {
						preSet.Repos = mo.Repos
					}
				}
			}
		}
		resp = append(resp, preSet)
	}
	return resp, nil
}

func CreateWorkflow(workflow *commonmodels.Workflow, log *zap.SugaredLogger) error {
	_, err := commonrepo.NewWorkflowColl().Find(workflow.Name)
	if err == nil {
		errStr := fmt.Sprintf("workflow [%s] 在项目 [%s] 中已经存在!", workflow.Name, workflow.ProductTmplName)
		return e.ErrUpsertWorkflow.AddDesc(errStr)
	}

	if !checkWorkflowSubModule(workflow) {
		return e.ErrUpsertWorkflow.AddDesc("workflow中没有子模块，请设置子模块")
	}

	if err := validateWorkflowHookNames(workflow); err != nil {
		return e.ErrUpsertWorkflow.AddDesc(err.Error())
	}

	err = commonservice.ProcessWebhook(workflow.HookCtl.Items, nil, webhook.WorkflowPrefix+workflow.Name, log)
	if err != nil {
		log.Errorf("Failed to process webhook, err: %s", err)
		return e.ErrUpsertWorkflow.AddDesc(err.Error())
	}

	err = HandleCronjob(workflow, log)
	if err != nil {
		return e.ErrUpsertWorkflow.AddDesc(err.Error())
	}

	if err := commonrepo.NewWorkflowColl().Create(workflow); err != nil {
		log.Errorf("Workflow.Create error: %v", err)
		return e.ErrUpsertWorkflow.AddDesc(err.Error())
	}

	go CreateGerritWebhook(workflow, log)

	return nil
}

func checkWorkflowSubModule(workflow *commonmodels.Workflow) bool {
	if workflow.Schedules != nil && workflow.Schedules.Enabled {
		return true
	}
	if workflow.Slack != nil && workflow.Slack.Enabled {
		return true
	}
	if workflow.BuildStage != nil && workflow.BuildStage.Enabled {
		return true
	}
	if workflow.ArtifactStage != nil && workflow.ArtifactStage.Enabled {
		return true
	}
	if workflow.TestStage != nil && workflow.TestStage.Enabled {
		return true
	}
	if workflow.SecurityStage != nil && workflow.SecurityStage.Enabled {
		return true
	}
	if workflow.DistributeStage != nil && workflow.DistributeStage.Enabled {
		return true
	}
	if workflow.NotifyCtl != nil && workflow.NotifyCtl.Enabled {
		return true
	}
	if workflow.HookCtl != nil && workflow.HookCtl.Enabled {
		return true
	}

	return false
}

func UpdateWorkflow(workflow *commonmodels.Workflow, log *zap.SugaredLogger) error {
	if !checkWorkflowSubModule(workflow) {
		return e.ErrUpsertWorkflow.AddDesc("workflow中没有子模块，请设置子模块")
	}

	currentWorkflow, err := commonrepo.NewWorkflowColl().Find(workflow.Name)
	if err != nil {
		log.Errorf("Can not find workflow %s, err: %s", workflow.Name, err)
		return e.ErrUpsertWorkflow.AddDesc(err.Error())
	}

	if err := validateWorkflowHookNames(workflow); err != nil {
		return e.ErrUpsertWorkflow.AddDesc(err.Error())
	}

	err = commonservice.ProcessWebhook(workflow.HookCtl.Items, currentWorkflow.HookCtl.Items, webhook.WorkflowPrefix+workflow.Name, log)
	if err != nil {
		log.Errorf("Failed to process webhook, err: %s", err)
		return e.ErrUpsertWorkflow.AddDesc(err.Error())
	}

	err = HandleCronjob(workflow, log)
	if err != nil {
		return e.ErrUpsertWorkflow.AddDesc(err.Error())
	}

	if err := UpdateGerritWebhook(workflow, log); err != nil {
		log.Errorf("UpdateGerritWebhook error: %v", err)
	}

	if workflow.TestStage != nil {
		for _, test := range workflow.TestStage.Tests {
			if test.Envs == nil {
				test.Envs = make([]*commonmodels.KeyVal, 0)
			}
		}
	}

	if err = commonrepo.NewWorkflowColl().Replace(workflow); err != nil {
		log.Errorf("Workflow.Update error: %v", err)
		return e.ErrUpsertWorkflow.AddDesc(err.Error())
	}

	return nil
}

func validateWorkflowHookNames(w *commonmodels.Workflow) error {
	if w == nil || w.HookCtl == nil {
		return nil
	}

	var names []string
	for _, hook := range w.HookCtl.Items {
		names = append(names, hook.MainRepo.Name)
	}

	return validateHookNames(names)
}

func ListWorkflows(projects []string, userID string, log *zap.SugaredLogger) ([]*Workflow, error) {
	existingProjects, err := template.NewProductColl().ListNames(projects)
	if err != nil {
		log.Errorf("Failed to list projects, err: %s", err)
		return nil, e.ErrListWorkflow.AddDesc(err.Error())
	}

	workflows, err := commonrepo.NewWorkflowColl().List(&commonrepo.ListWorkflowOption{Projects: existingProjects})
	if err != nil {
		log.Errorf("Failed to list workflows, err: %s", err)
		return nil, e.ErrListWorkflow.AddDesc(err.Error())
	}

	var workflowNames []string
	var res []*Workflow
	for _, w := range workflows {
		stages := make([]string, 0, 4)
		if w.BuildStage != nil && w.BuildStage.Enabled {
			stages = append(stages, "build")
		}
		if w.ArtifactStage != nil && w.ArtifactStage.Enabled {
			stages = append(stages, "artifact")
		}
		if w.TestStage != nil && w.TestStage.Enabled {
			stages = append(stages, "test")
		}
		if w.DistributeStage != nil && w.DistributeStage.Enabled {
			stages = append(stages, "distribute")
		}

		res = append(res, &Workflow{
			Name:             w.Name,
			ProjectName:      w.ProductTmplName,
			UpdateTime:       w.UpdateTime,
			CreateTime:       w.CreateTime,
			UpdateBy:         w.UpdateBy,
			SchedulerEnabled: w.ScheduleEnabled,
			Schedules:        w.Schedules,
			EnabledStages:    stages,
			Description:      w.Description,
		})

		workflowNames = append(workflowNames, w.Name)
	}

	favorites, err := commonrepo.NewFavoriteColl().List(&commonrepo.FavoriteArgs{UserID: userID, Type: string(config.WorkflowType)})
	if err != nil {
		log.Warnf("Failed to list favorites, err: %s", err)
	}
	favoriteSet := sets.NewString()
	for _, f := range favorites {
		favoriteSet.Insert(f.Name)
	}

	workflowStats, err := commonrepo.NewWorkflowStatColl().FindWorkflowStat(&commonrepo.WorkflowStatArgs{Names: workflowNames, Type: string(config.WorkflowType)})
	if err != nil {
		log.Warnf("Failed to list workflow stats, err: %s", err)
	}
	workflowStatMap := make(map[string]*commonmodels.WorkflowStat)
	for _, s := range workflowStats {
		workflowStatMap[s.Name] = s
	}

	recentTaskMap := getTaskMap(workflowNames, "", log)
	recentSuccessfulTaskMap := getTaskMap(workflowNames, config.StatusPassed, log)
	recentFailedTaskMap := getTaskMap(workflowNames, config.StatusFailed, log)
	for _, r := range res {
		if t, ok := recentTaskMap[r.Name]; ok {
			r.RecentTask = t
		}
		if t, ok := recentSuccessfulTaskMap[r.Name]; ok {
			r.RecentSuccessfulTask = t
		}
		if t, ok := recentFailedTaskMap[r.Name]; ok {
			r.RecentFailedTask = t
		}

		if favoriteSet.Has(r.Name) {
			r.IsFavorite = true
		}

		if s, ok := workflowStatMap[r.Name]; ok {
			total := float64(s.TotalSuccess + s.TotalFailure)
			successful := float64(s.TotalSuccess)
			totalDuration := float64(s.TotalDuration)

			r.AverageExecutionTime = totalDuration / total
			r.SuccessRate = successful / total
		}
	}

	return res, nil
}

func getTaskMap(names []string, status config.Status, logger *zap.SugaredLogger) map[string]*TaskInfo {
	res := make(map[string]*TaskInfo)

	tasks, err := commonrepo.NewTaskColl().ListRecentTasks(&commonrepo.ListTaskOption{PipelineNames: names, Type: config.WorkflowType, Status: status})
	if err != nil {
		logger.Warnf("Failed to list tasks, err: %s", err)
		return res
	}

	for _, t := range tasks {
		res[t.PipelineName] = &TaskInfo{
			TaskID:       t.TaskID,
			PipelineName: t.PipelineName,
			Status:       t.Status,
		}
	}

	return res
}

func ListTestWorkflows(testName string, projects []string, log *zap.SugaredLogger) (workflows []*commonmodels.Workflow, err error) {
	allWorkflows, err := commonrepo.NewWorkflowColl().ListWorkflowsByProjects(projects)
	if err != nil {
		return nil, err
	}
	for _, workflow := range allWorkflows {
		if workflow.TestStage != nil {
			testNames := sets.NewString(workflow.TestStage.TestNames...)
			if testNames.Has(testName) {
				continue
			}
			for _, testEntity := range workflow.TestStage.Tests {
				if testEntity.Name == testName {
					continue
				}
			}
		}
		workflows = append(workflows, workflow)
	}
	return workflows, nil
}

func CopyWorkflow(oldWorkflowName, newWorkflowName, username string, log *zap.SugaredLogger) error {
	oldWorkflow, err := commonrepo.NewWorkflowColl().Find(oldWorkflowName)
	if err != nil {
		log.Error(err)
		return e.ErrGetPipeline.AddErr(err)
	}
	_, err = commonrepo.NewWorkflowColl().Find(newWorkflowName)
	if err == nil {
		log.Error("new workflow already exists")
		return e.ErrExistsPipeline
	}
	oldWorkflow.UpdateBy = username
	oldWorkflow.Name = newWorkflowName
	oldWorkflow.ID = primitive.NewObjectID()

	return commonrepo.NewWorkflowColl().Create(oldWorkflow)
}
