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
	"sort"
	"time"

	"github.com/hashicorp/go-multierror"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb/template"
	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/collaboration"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/webhook"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/cache"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/types"
	"github.com/koderover/zadig/v2/pkg/util"
)

type EnvStatus struct {
	EnvName    string `json:"env_name,omitempty"`
	Status     string `json:"status"`
	ErrMessage string `json:"err_message"`
}

type Workflow struct {
	Name                 string                     `json:"name"`
	DisplayName          string                     `json:"display_name"`
	ProjectName          string                     `json:"projectName"`
	Disabled             bool                       `json:"disabled"`
	UpdateTime           int64                      `json:"updateTime"`
	CreateTime           int64                      `json:"createTime"`
	UpdateBy             string                     `json:"updateBy,omitempty"`
	Schedules            *commonmodels.ScheduleCtrl `json:"schedules,omitempty"`
	SchedulerEnabled     bool                       `json:"schedulerEnabled"`
	EnabledStages        []string                   `json:"enabledStages"`
	IsFavorite           bool                       `json:"isFavorite"`
	WorkflowType         string                     `json:"workflow_type"`
	RecentTask           *TaskInfo                  `json:"recentTask"`
	RecentTasks          []*TaskInfo                `json:"recentTasks"`
	RecentSuccessfulTask *TaskInfo                  `json:"recentSuccessfulTask"`
	RecentFailedTask     *TaskInfo                  `json:"recentFailedTask"`
	AverageExecutionTime float64                    `json:"averageExecutionTime"`
	SuccessRate          float64                    `json:"successRate"`
	Description          string                     `json:"description,omitempty"`
	BaseName             string                     `json:"base_name"`
	BaseRefs             []string                   `json:"base_refs"`
	NeverRun             bool                       `json:"never_run"`
	EnableApprovalTicket bool                       `json:"enable_approval_ticket"`
}

type TaskInfo struct {
	TaskID       int64  `json:"taskID"`
	PipelineName string `json:"pipelineName,omitempty"`
	Status       string `json:"status"`
	TaskCreator  string `json:"task_creator"`
	CreateTime   int64  `json:"create_time"`
	RunningTime  int64  `json:"running_time,omitempty"`
	StartTime    int64  `json:"start_time,omitempty"`
	EndTime      int64  `json:"end_time,omitempty"`
}

type workflowCreateArg struct {
	name              string
	envName           string
	buildStageEnabled bool
	dockerRegistryID  string
}

type workflowCreateArgs struct {
	productName string
	argsMap     map[string]*workflowCreateArg
}

func FindWorkflowRaw(name string, logger *zap.SugaredLogger) (*commonmodels.Workflow, error) {
	workflow, err := commonrepo.NewWorkflowColl().Find(name)
	if err != nil {
		logger.Errorf("Failed to find Product Workflow: %s, the error is: %v", name, err)
		return workflow, e.ErrFindWorkflow.AddErr(err)
	}
	return workflow, err
}

func (args *workflowCreateArgs) addWorkflowArg(envName, dockerRegistryID string, buildStageEnabled bool) {
	wName := fmt.Sprintf("%s-workflow-%s", args.productName, envName)
	// The hosting env workflow name is not bound to the environment
	if envName == "" {
		wName = fmt.Sprintf("%s-workflow", args.productName)
	}
	if !buildStageEnabled {
		wName = fmt.Sprintf("%s-%s-workflow", args.productName, "ops")
	}
	args.argsMap[wName] = &workflowCreateArg{
		name:              wName,
		envName:           envName,
		buildStageEnabled: buildStageEnabled,
		dockerRegistryID:  dockerRegistryID,
	}
}

func (args *workflowCreateArgs) initDefaultWorkflows() {
	args.addWorkflowArg("dev", "", true)
	args.addWorkflowArg("qa", "", true)
	args.addWorkflowArg("", "", false)
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

	mut := cache.NewRedisLock(fmt.Sprintf("auto_create_product:%s", productName))

	mut.Lock()
	defer func() {
		mut.Unlock()
	}()

	createArgs := &workflowCreateArgs{
		productName: productName,
		argsMap:     make(map[string]*workflowCreateArg),
	}
	createArgs.initDefaultWorkflows()

	s3storageID := ""
	s3storage, err := commonrepo.NewS3StorageColl().FindDefault()
	if err != nil {
		log.Errorf("S3Storage.FindDefault error: %v", err)
	} else {
		projectSet := sets.NewString(s3storage.Projects...)
		if projectSet.Has(productName) || projectSet.Has(setting.AllProjects) {
			s3storageID = s3storage.ID.Hex()
		}
	}

	productList, err := commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{
		Name:       productName,
		Production: util.GetBoolPointer(false),
	})
	if err != nil {
		log.Errorf("fialed to list products, projectName %s, err %s", productName, err)
	}

	createArgs.clear()
	for _, product := range productList {
		createArgs.addWorkflowArg(product.EnvName, product.RegistryID, true)
	}
	if !productTmpl.IsHostProduct() {
		createArgs.addWorkflowArg("", "", false)
	}

	workflowSet := sets.NewString()
	for workflowName := range createArgs.argsMap {
		_, err := FindWorkflowV4Raw(workflowName, log)
		if err == nil {
			workflowSet.Insert(workflowName)
		}
	}

	if len(workflowSet) < len(createArgs.argsMap) {
		services, err := commonrepo.NewServiceColl().ListMaxRevisionsForServices(productTmpl.AllTestServiceInfos(), "")
		if err != nil {
			log.Errorf("ServiceTmpl.ListMaxRevisionsByProject error: %v", err)
			errList = multierror.Append(errList, err)
		}
		buildList, err := commonrepo.NewBuildColl().List(&commonrepo.BuildListOption{
			ProductName: productName,
		})
		if err != nil {
			log.Errorf("[Build.List] error: %v", err)
			errList = multierror.Append(errList, err)
		}
		buildMap := map[string]*commonmodels.Build{}
		for _, build := range buildList {
			for _, target := range build.Targets {
				buildMap[target.ServiceName] = build
			}
		}

		for workflowName, workflowArg := range createArgs.argsMap {
			if workflowSet.Has(workflowName) {
				continue
			}
			if dupWorkflow, err := commonrepo.NewWorkflowV4Coll().Find(workflowName); err == nil {
				errList = multierror.Append(errList, fmt.Errorf("workflow [%s] 在项目 [%s] 中已经存在", workflowName, dupWorkflow.Project))
			}
			workflow := new(commonmodels.WorkflowV4)
			workflow.Project = productName
			workflow.Name = workflowName
			workflow.DisplayName = workflowName
			workflow.CreatedBy = setting.SystemUser
			workflow.UpdatedBy = setting.SystemUser
			workflow.CreateTime = time.Now().Unix()
			workflow.UpdateTime = time.Now().Unix()
			workflow.ConcurrencyLimit = 1

			buildJobName := ""
			if workflowArg.buildStageEnabled {
				buildTargetSet := sets.NewString()
				serviceAndBuilds := []*commonmodels.ServiceAndBuild{}
				for _, serviceTmpl := range services {
					if build, ok := buildMap[serviceTmpl.ServiceName]; ok {
						for _, target := range build.Targets {
							key := fmt.Sprintf("%s-%s", target.ServiceName, target.ServiceModule)
							if buildTargetSet.Has(key) {
								continue
							}
							buildTargetSet.Insert(key)

							serviceAndBuild := &commonmodels.ServiceAndBuild{
								ServiceName:   target.ServiceName,
								ServiceModule: target.ServiceModule,
								BuildName:     build.Name,
							}
							serviceAndBuilds = append(serviceAndBuilds, serviceAndBuild)
						}
					}
				}
				buildJobName = "构建"
				buildJob := &commonmodels.Job{
					Name:    buildJobName,
					JobType: config.JobZadigBuild,
					Spec: &commonmodels.ZadigBuildJobSpec{
						DockerRegistryID:        workflowArg.dockerRegistryID,
						ServiceAndBuildsOptions: serviceAndBuilds,
					},
				}
				stage := &commonmodels.WorkflowStage{
					Name:     "构建",
					Parallel: true,
					Jobs:     []*commonmodels.Job{buildJob},
				}
				workflow.Stages = append(workflow.Stages, stage)
			}

			if productTmpl.IsCVMProduct() {
				spec := &commonmodels.ZadigVMDeployJobSpec{
					Env:         workflowArg.envName,
					Source:      config.SourceRuntime,
					S3StorageID: s3storageID,
				}
				if workflowArg.buildStageEnabled {
					spec.Source = config.SourceFromJob
					spec.JobName = buildJobName
				}
				deployJob := &commonmodels.Job{
					Name:    "主机部署",
					JobType: config.JobZadigVMDeploy,
					Spec:    spec,
				}
				stage := &commonmodels.WorkflowStage{
					Name:     "主机部署",
					Parallel: true,
					Jobs:     []*commonmodels.Job{deployJob},
				}
				workflow.Stages = append(workflow.Stages, stage)
			} else {
				spec := &commonmodels.ZadigDeployJobSpec{
					Env: workflowArg.envName,
					DeployContents: []config.DeployContent{
						config.DeployImage,
					},
					Source:     config.SourceRuntime,
					Production: true,
				}
				if workflowArg.buildStageEnabled {
					spec.Source = config.SourceFromJob
					spec.JobName = buildJobName
					spec.Production = false
				}
				deployJob := &commonmodels.Job{
					Name:    "部署",
					JobType: config.JobZadigDeploy,
					Spec:    spec,
				}
				stage := &commonmodels.WorkflowStage{
					Name:     "部署",
					Parallel: true,
					Jobs:     []*commonmodels.Job{deployJob},
				}
				workflow.Stages = append(workflow.Stages, stage)
			}

			if _, err := commonrepo.NewWorkflowV4Coll().Create(workflow); err != nil {
				errList = multierror.Append(errList, err)
			}
		}
		if err = errList.ErrorOrNil(); err != nil {
			return &EnvStatus{Status: setting.ProductStatusFailed, ErrMessage: err.Error()}
		}
		return &EnvStatus{Status: setting.ProductStatusCreating}
	} else if len(workflowSet) == len(createArgs.argsMap) {
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
			ParentType: setting.WorkflowCronjob,
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

	services, err := commonrepo.NewServiceColl().ListMaxRevisionsByProduct(resp.ProductTmplName)
	if err != nil {
		log.Errorf("ServiceTmpl.ListMaxRevisionsByProject error: %v", err)
		return resp, e.ErrListTemplate.AddDesc(err.Error())
	}

	if resp.BuildStage.Enabled {
		// make a map of current target modules
		buildMap := map[string]*commonmodels.BuildModule{}

		moList, err := commonrepo.NewBuildColl().List(&commonrepo.BuildListOption{})
		if err != nil {
			return resp, e.ErrListTemplate.AddDesc(err.Error())
		}
		for _, build := range resp.BuildStage.Modules {
			key := fmt.Sprintf("%s-%s-%s", build.Target.ProductName, build.Target.ServiceName, build.Target.ServiceModule)
			buildMap[key] = build
			if build.Target.BuildName == "" {
				build.Target.BuildName = findBuildName(key, moList)
			}
		}

		buildModules := []*commonmodels.BuildModule{}
		for _, serviceTmpl := range services {

			if serviceTmpl.Type == setting.PMDeployType {
				serviceTmpl.Containers = append(serviceTmpl.Containers, &commonmodels.Container{
					Name: serviceTmpl.ServiceName,
				})
			}

			switch serviceTmpl.Type {
			case setting.K8SDeployType, setting.HelmDeployType, setting.PMDeployType:
				for _, container := range serviceTmpl.Containers {
					key := fmt.Sprintf("%s-%s-%s", serviceTmpl.ProductName, serviceTmpl.ServiceName, container.Name)
					// if no target info is found for this container, meaning that this is a new service for that workflow
					// then we need to add it to the response
					if mod, ok := buildMap[key]; !ok {
						buildModules = append(buildModules, &commonmodels.BuildModule{
							HideServiceModule: false,
							BuildModuleVer:    "stable",
							Target: &commonmodels.ServiceModuleTarget{
								ProductName: serviceTmpl.ProductName,
								ServiceWithModule: commonmodels.ServiceWithModule{
									ServiceName:   serviceTmpl.ServiceName,
									ServiceModule: container.Name,
								},
								BuildName: findBuildName(key, moList),
							},
						})
					} else {
						buildModules = append(buildModules, mod)
					}
				}
			}
		}
		resp.BuildStage.Modules = buildModules
	}

	for _, module := range resp.BuildStage.Modules {
		for _, bf := range module.BranchFilter {
			bf.RepoNamespace = bf.GetNamespace()
		}
	}
	if resp.ArtifactStage != nil && resp.ArtifactStage.Enabled {
		// make a map of current target modules
		artifactMap := map[string]*commonmodels.ArtifactModule{}

		for _, artifact := range resp.ArtifactStage.Modules {
			key := fmt.Sprintf("%s-%s-%s", artifact.Target.ProductName, artifact.Target.ServiceName, artifact.Target.ServiceModule)
			artifactMap[key] = artifact
		}

		artifactModules := []*commonmodels.ArtifactModule{}
		for _, serviceTmpl := range services {
			switch serviceTmpl.Type {
			case setting.PMDeployType:
				// PM service does not have such logic
				artifactModules = resp.ArtifactStage.Modules
				break

			case setting.K8SDeployType, setting.HelmDeployType:
				for _, container := range serviceTmpl.Containers {
					key := fmt.Sprintf("%s-%s-%s", serviceTmpl.ProductName, serviceTmpl.ServiceName, container.Name)
					// if no target info is found for this container, meaning that this is a new service for that workflow
					// then we need to add it to the response
					if mod, ok := artifactMap[key]; !ok {
						artifactModules = append(artifactModules, &commonmodels.ArtifactModule{
							HideServiceModule: false,
							Target: &commonmodels.ServiceModuleTarget{
								ProductName: serviceTmpl.ProductName,
								ServiceWithModule: commonmodels.ServiceWithModule{
									ServiceName:   serviceTmpl.ServiceName,
									ServiceModule: container.Name,
								},
							},
						})
					} else {
						artifactModules = append(artifactModules, mod)
					}
				}
			}
		}
		resp.ArtifactStage.Modules = artifactModules
	}

	for _, test := range resp.TestStage.Tests {
		testModule, err := commonrepo.NewTestingColl().Find(test.Name, "")
		if err != nil {
			log.Errorf("test module: %s not found", test.Name)
			continue
		}
		test.Project = testModule.ProductName
	}
	return resp, nil
}

func findBuildName(key string, moList []*commonmodels.Build) string {
	for _, mo := range moList {
		for _, moTarget := range mo.Targets {
			moduleTargetStr := fmt.Sprintf("%s-%s-%s", moTarget.ProductName, moTarget.ServiceName, moTarget.ServiceModule)
			if key == moduleTargetStr {
				return mo.Name
			}
		}
	}
	return ""
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
	services, err := commonrepo.NewServiceColl().ListMaxRevisionsForServices(productTmpl.AllTestServiceInfos(), "")
	if err != nil {
		log.Errorf("ServiceTmpl.ListMaxRevisionsByProject error: %v", err)
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
	for _, mo := range moList {
		for _, moTarget := range mo.Targets {
			target := fmt.Sprintf("%s%s%s%s%s", moTarget.ProductName, SplitSymbol, moTarget.ServiceName, SplitSymbol, moTarget.ServiceModule)
			if v, ok := targets[target]; ok {
				preSet := &PreSetResp{
					Target: &commonmodels.ServiceModuleTarget{
						ProductName: moTarget.ProductName,
						ServiceWithModule: commonmodels.ServiceWithModule{
							ServiceName:   moTarget.ServiceName,
							ServiceModule: moTarget.ServiceModule,
						},
						BuildName: mo.Name,
					},
					Deploy:          v,
					BuildModuleVers: []string{},
					Repos:           make([]*types.Repository, 0),
				}
				if mo.TemplateID != "" {
					preSet.Repos = moTarget.Repos
				} else {
					preSet.Repos = mo.SafeRepos()
				}
				resp = append(resp, preSet)
			}
		}
	}
	return resp, nil
}

func CreateWorkflow(workflow *commonmodels.Workflow, log *zap.SugaredLogger) error {
	existedWorkflow, err := commonrepo.NewWorkflowColl().Find(workflow.Name)
	if err == nil {
		errStr := fmt.Sprintf("与项目 [%s] 中的工作流 [%s] 标识相同", existedWorkflow.ProductTmplName, existedWorkflow.DisplayName)
		return e.ErrUpsertWorkflow.AddDesc(errStr)
	}
	existedWorkflows, _ := commonrepo.NewWorkflowColl().List(&commonrepo.ListWorkflowOption{Projects: []string{workflow.ProductTmplName}, DisplayName: workflow.DisplayName})
	if len(existedWorkflows) > 0 {
		errStr := fmt.Sprintf("当前项目已存在工作流 [%s]", workflow.DisplayName)
		return e.ErrUpsertWorkflow.AddDesc(errStr)
	}

	if !checkWorkflowSubModule(workflow) {
		return e.ErrUpsertWorkflow.AddDesc("未检测到构建部署或交付物部署，请配置一项")
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
		return e.ErrUpsertWorkflow.AddDesc("未检测到构建部署或交付物部署，请配置一项")
	}

	currentWorkflow, err := commonrepo.NewWorkflowColl().Find(workflow.Name)
	if err != nil {
		log.Errorf("Can not find workflow %s, err: %s", workflow.Name, err)
		return e.ErrUpsertWorkflow.AddDesc(err.Error())
	}
	if workflow.DisplayName != currentWorkflow.DisplayName {
		existedWorkflows, _ := commonrepo.NewWorkflowColl().List(&commonrepo.ListWorkflowOption{Projects: []string{workflow.ProductTmplName}, DisplayName: workflow.DisplayName})
		if len(existedWorkflows) > 0 {
			errStr := fmt.Sprintf("workflow [%s] 展示名称在当前项目下重复!", workflow.DisplayName)
			return e.ErrUpsertWorkflow.AddDesc(errStr)
		}
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

func ListWorkflows(projects []string, userID string, names []string, log *zap.SugaredLogger) ([]*Workflow, error) {
	existingProjects, err := template.NewProductColl().ListNames(projects)
	if err != nil {
		log.Errorf("Failed to list projects, err: %s", err)
		return nil, e.ErrListWorkflow.AddDesc(err.Error())
	}

	workflows, err := commonrepo.NewWorkflowColl().List(&commonrepo.ListWorkflowOption{Projects: existingProjects, Names: names})
	if err != nil {
		log.Errorf("Failed to list workflows, err: %s", err)
		return nil, e.ErrListWorkflow.AddDesc(err.Error())
	}

	workflowNames := []string{}
	var res []*Workflow
	workflowCMMap, err := collaboration.GetWorkflowCMMap(projects, log)
	if err != nil {
		return nil, err
	}
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
		var baseRefs []string
		if cmSet, ok := workflowCMMap[collaboration.BuildWorkflowCMMapKey(w.ProductTmplName, w.Name)]; ok {
			for _, cm := range cmSet.List() {
				baseRefs = append(baseRefs, cm)
			}
		}
		res = append(res, &Workflow{
			Name:             w.Name,
			DisplayName:      w.DisplayName,
			ProjectName:      w.ProductTmplName,
			UpdateTime:       w.UpdateTime,
			CreateTime:       w.CreateTime,
			UpdateBy:         w.UpdateBy,
			SchedulerEnabled: w.ScheduleEnabled,
			Schedules:        w.Schedules,
			EnabledStages:    stages,
			Description:      w.Description,
			BaseName:         w.BaseName,
			BaseRefs:         baseRefs,
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

	workflowStatMap := getWorkflowStatMap(workflowNames, config.WorkflowType)

	tasks, err := commonrepo.NewTaskColl().ListPreview(workflowNames)
	if err != nil {
		log.Warnf("Failed to list workflow tasks, err: %s", err)
	}
	for _, r := range res {
		getRecentTaskInfo(r, tasks)

		if favoriteSet.Has(r.Name) {
			r.IsFavorite = true
		}
		setWorkflowStat(r, workflowStatMap)
	}

	return res, nil
}

func getRecentTaskInfo(workflow *Workflow, tasks []*commonrepo.TaskPreview) {
	recentTask := &commonrepo.TaskPreview{}
	recentFailedTask := &commonrepo.TaskPreview{}
	recentSucceedTask := &commonrepo.TaskPreview{}
	workflow.NeverRun = true
	var workflowList []*commonrepo.TaskPreview
	var recentTenTask []*commonrepo.TaskPreview
	for _, task := range tasks {
		if task.PipelineName != workflow.Name {
			continue
		}
		workflow.NeverRun = false
		workflowList = append(workflowList, task)
	}
	sort.Slice(workflowList, func(i, j int) bool {
		return workflowList[i].TaskID > workflowList[j].TaskID
	})
	for _, task := range workflowList {
		if recentSucceedTask.TaskID != 0 && recentFailedTask.TaskID != 0 && len(recentTenTask) == 10 {
			break
		}
		if recentTask.TaskID == 0 {
			recentTask = task
		}
		if task.Status == config.StatusPassed && recentSucceedTask.TaskID == 0 {
			recentSucceedTask = task
		}
		if task.Status == config.StatusFailed && recentFailedTask.TaskID == 0 {
			recentFailedTask = task
		}
		if len(recentTenTask) < 10 {
			recentTenTask = append(recentTenTask, task)
		}
	}

	if recentTask.TaskID > 0 {
		workflow.RecentTask = &TaskInfo{
			TaskID:       recentTask.TaskID,
			PipelineName: recentTask.PipelineName,
			Status:       string(recentTask.Status),
			TaskCreator:  recentTask.TaskCreator,
			CreateTime:   recentTask.CreateTime,
		}
	}
	if recentSucceedTask.TaskID > 0 {
		workflow.RecentSuccessfulTask = &TaskInfo{
			TaskID:       recentSucceedTask.TaskID,
			PipelineName: recentSucceedTask.PipelineName,
			Status:       string(recentSucceedTask.Status),
			TaskCreator:  recentSucceedTask.TaskCreator,
			CreateTime:   recentSucceedTask.CreateTime,
		}
	}
	if recentFailedTask.TaskID > 0 {
		workflow.RecentFailedTask = &TaskInfo{
			TaskID:       recentFailedTask.TaskID,
			PipelineName: recentFailedTask.PipelineName,
			Status:       string(recentFailedTask.Status),
			TaskCreator:  recentFailedTask.TaskCreator,
			CreateTime:   recentFailedTask.CreateTime,
		}
	}
	if len(recentTenTask) > 0 {
		for _, task := range recentTenTask {
			workflow.RecentTasks = append(workflow.RecentTasks, &TaskInfo{
				TaskID:      task.TaskID,
				Status:      string(task.Status),
				TaskCreator: task.TaskCreator,
				CreateTime:  task.CreateTime,
				StartTime:   task.StartTime,
				EndTime:     task.EndTime,
			})
		}
	}
}

func ListTestWorkflows(testName string, projects []string, log *zap.SugaredLogger) (workflows []*commonmodels.Workflow, err error) {
	var allWorkflows []*commonmodels.Workflow
	if len(projects) != 0 {
		allWorkflows, err = commonrepo.NewWorkflowColl().ListWorkflowsByProjects(projects)
		if err != nil {
			return nil, err
		}
	} else {
		allWorkflows, err = commonrepo.NewWorkflowColl().List(&commonrepo.ListWorkflowOption{})
		if err != nil {
			return nil, err
		}
	}
LOOP:
	for _, workflow := range allWorkflows {
		if workflow.TestStage != nil {
			testNames := sets.NewString(workflow.TestStage.TestNames...)
			if testNames.Has(testName) {
				continue
			}
			for _, testEntity := range workflow.TestStage.Tests {
				if testEntity.Name == testName {
					continue LOOP
				}
			}
		}
		workflows = append(workflows, workflow)
	}
	return workflows, nil
}

func CopyWorkflow(oldWorkflowName, newWorkflowName, newWorkflowDisplayName, username string, log *zap.SugaredLogger) error {
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
	existedWorkflows, _ := commonrepo.NewWorkflowColl().List(&commonrepo.ListWorkflowOption{Projects: []string{oldWorkflow.ProductTmplName}, DisplayName: newWorkflowDisplayName})
	if len(existedWorkflows) > 0 {
		errStr := fmt.Sprintf("当前项目已存在工作流 [%s]", newWorkflowDisplayName)
		return e.ErrUpsertWorkflow.AddDesc(errStr)
	}
	oldWorkflow.UpdateBy = username
	oldWorkflow.Name = newWorkflowName
	oldWorkflow.DisplayName = newWorkflowDisplayName
	oldWorkflow.ID = primitive.NewObjectID()

	return commonrepo.NewWorkflowColl().Create(oldWorkflow)
}

type WorkflowCopyItem struct {
	ProjectName    string `json:"project_name"`
	Old            string `json:"old"`
	New            string `json:"new"`
	NewDisplayName string `json:"new_display_name"`
	BaseName       string `json:"base_name"`
}

type BulkCopyWorkflowArgs struct {
	Items []WorkflowCopyItem `json:"items"`
}

func BulkCopyWorkflow(args BulkCopyWorkflowArgs, username string, log *zap.SugaredLogger) error {
	var workflows []commonrepo.Workflow

	for _, item := range args.Items {
		workflows = append(workflows, commonrepo.Workflow{
			ProjectName: item.ProjectName,
			Name:        item.Old,
		})
	}
	oldWorkflows, err := commonrepo.NewWorkflowColl().ListByWorkflows(commonrepo.ListWorkflowOpt{
		Workflows: workflows,
	})
	if err != nil {
		log.Error(err)
		return e.ErrGetPipeline.AddErr(err)
	}
	workflowMap := make(map[string]*commonmodels.Workflow)
	for _, workflow := range oldWorkflows {
		workflowMap[workflow.ProductTmplName+"-"+workflow.Name] = workflow
	}
	var newWorkflows []*commonmodels.Workflow
	for _, workflow := range args.Items {
		if item, ok := workflowMap[workflow.ProjectName+"-"+workflow.Old]; ok {
			newItem := *item
			newItem.UpdateBy = username
			newItem.Name = workflow.New
			newItem.DisplayName = workflow.NewDisplayName
			newItem.BaseName = workflow.BaseName
			newItem.ID = primitive.NewObjectID()

			newWorkflows = append(newWorkflows, &newItem)
		} else {
			log.Errorf("workflow:%s not exist", workflow.ProjectName+"-"+workflow.Old)
			return fmt.Errorf("workflow:%s not exist", workflow.ProjectName+"-"+workflow.Old)
		}
	}
	return commonrepo.NewWorkflowColl().BulkCreate(newWorkflows)
}

func getWorkflowStatMap(workflowNames []string, workflowType config.PipelineType) map[string]*commonmodels.WorkflowStat {
	workflowStats, err := commonrepo.NewWorkflowStatColl().FindWorkflowStat(&commonrepo.WorkflowStatArgs{Names: workflowNames, Type: string(workflowType)})
	if err != nil {
		log.Warnf("Failed to list workflow stats, err: %s", err)
	}
	workflowStatMap := make(map[string]*commonmodels.WorkflowStat)
	for _, s := range workflowStats {
		workflowStatMap[s.Name] = s
	}
	return workflowStatMap
}

func setWorkflowStat(workflow *Workflow, statMap map[string]*commonmodels.WorkflowStat) {
	if s, ok := statMap[workflow.Name]; ok {
		total := float64(s.TotalSuccess + s.TotalFailure)
		successful := float64(s.TotalSuccess)
		totalDuration := float64(s.TotalDuration)

		workflow.AverageExecutionTime = totalDuration / total
		workflow.SuccessRate = successful / total
	}
}
