/*
Copyright 2026 The KodeRover Authors.

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

package service

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/koderover/zadig/v2/pkg/config"
	aslanconfig "github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/code/client"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/code/client/open"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	workflowservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/workflow/service/workflow"
	workflowController "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/workflow/service/workflow/controller"
	"github.com/koderover/zadig/v2/pkg/shared/client/systemconfig"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	"github.com/koderover/zadig/v2/pkg/tool/larkplugin"
	"github.com/koderover/zadig/v2/pkg/types"
	"github.com/koderover/zadig/v2/pkg/util"
	sdkcore "github.com/larksuite/project-oapi-sdk-golang/core"
	"github.com/larksuite/project-oapi-sdk-golang/service/project"
	"github.com/larksuite/project-oapi-sdk-golang/v2/service/workitem"
	"go.mongodb.org/mongo-driver/mongo"
	"k8s.io/apimachinery/pkg/util/sets"
)

type GetLarkWorkflowConfigV2Resp struct {
	StageName       string                                     `json:"stage_name"`
	WorkspaceID     string                                     `json:"workspace_id"`
	UpdateTime      int64                                      `json:"update_time"`
	CodeSource      string                                     `json:"code_source"`
	WorkItemTypeKey string                                     `json:"work_item_type_key"`
	WorkItemType    string                                     `json:"work_item_type"`
	BranchFilter    string                                     `json:"branch_filter"`
	TargetBranch    string                                     `json:"target_branch"`
	Nodes           []*commonmodels.LarkPluginWorkflowConfigV2 `json:"nodes"`
}

func GetLarkWorkflowConfigV2(ctx *internalhandler.Context, workspaceID, stageName string) (*GetLarkWorkflowConfigV2Resp, error) {
	cfg, err := mongodb.NewLarkPluginStageConfigV2Coll().GetByStage(workspaceID, stageName)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return &GetLarkWorkflowConfigV2Resp{
				StageName:   stageName,
				WorkspaceID: workspaceID,
				Nodes:       make([]*commonmodels.LarkPluginWorkflowConfigV2, 0),
			}, nil
		}
		return nil, fmt.Errorf("failed to get lark plugin workflow config v2: %w", err)
	}

	nodes, err := mongodb.NewLarkPluginWorkflowConfigV2Coll().GetByStage(workspaceID, stageName)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			nodes = make([]*commonmodels.LarkPluginWorkflowConfigV2, 0)
		} else {
			return nil, fmt.Errorf("failed to get lark plugin workflow config nodes v2: %w", err)
		}
	}

	return &GetLarkWorkflowConfigV2Resp{
		StageName:    cfg.StageName,
		WorkspaceID:  cfg.WorkspaceID,
		UpdateTime:   cfg.UpdateTime,
		CodeSource:   cfg.CodeSource,
		WorkItemTypeKey:   cfg.WorkItemTypeKey,
		WorkItemType:    cfg.WorkItemType,
		BranchFilter:    cfg.BranchFilter,
		TargetBranch:    cfg.TargetBranch,
		Nodes:           nodes,
	}, nil
}

type UpdateLarkWorkflowConfigV2Req struct {
	WorkspaceID     string                                     `json:"workspace_id"`
	CodeSource      string                                     `json:"code_source"`
	WorkItemTypeKey string                                     `json:"work_item_type_key"`
	WorkItemType    string                                     `json:"work_item_type"`
	BranchFilter    string                                     `json:"branch_filter"`
	TargetBranch    string                                     `json:"target_branch"`
	Nodes           []*commonmodels.LarkPluginWorkflowConfigV2 `json:"nodes"`
}

func UpdateLarkWorkflowConfigV2(ctx *internalhandler.Context, stageName string, req *UpdateLarkWorkflowConfigV2Req) error {
	validatedWorkflows := sets.New[string]()
	for _, node := range req.Nodes {
		if node.WorkflowName == "" || validatedWorkflows.Has(node.WorkflowName) {
			continue
		}
		if err := validateWorkflowForLarkPlugin(node.WorkflowName); err != nil {
			return err
		}
		validatedWorkflows.Insert(node.WorkflowName)
	}

	cfg := &commonmodels.LarkPluginStageConfigV2{
		StageName:       stageName,
		WorkspaceID:     req.WorkspaceID,
		CodeSource:      req.CodeSource,
		WorkItemTypeKey: req.WorkItemTypeKey,
		WorkItemType:    req.WorkItemType,
		BranchFilter:    req.BranchFilter,
		TargetBranch:    req.TargetBranch,
		UpdateTime:      time.Now().Unix(),
	}

	if err := mongodb.NewLarkPluginStageConfigV2Coll().Upsert(cfg); err != nil {
		return fmt.Errorf("failed to upsert lark plugin workflow config v2: %w", err)
	}

	if err := mongodb.NewLarkPluginWorkflowConfigV2Coll().ReplaceByStage(req.WorkspaceID, stageName, req.Nodes); err != nil {
		return fmt.Errorf("failed to replace lark plugin workflow config nodes v2: %w", err)
	}

	return nil
}

type GetLarkWorkitemServicesV2Resp struct {
	Services []*commonmodels.ServiceWithModule `json:"services"`
}

func GetLarkWorkitemServicesV2(ctx *internalhandler.Context, workspaceID, workItemType, workItemID string) (*GetLarkWorkitemServicesV2Resp, error) {
	workflow, err := getWorkflowFromWorkItem(ctx, workspaceID, workItemType, workItemID)
	if err != nil {
		return nil, fmt.Errorf("failed to find workflow: %w", err)
	}

	// Find the first build job in the workflow
	var buildSpec *commonmodels.ZadigBuildJobSpec
	for _, stage := range workflow.Stages {
		for _, job := range stage.Jobs {
			if job.JobType == aslanconfig.JobZadigBuild {
				spec := &commonmodels.ZadigBuildJobSpec{}
				if err := commonmodels.IToiYaml(job.Spec, spec); err != nil {
					continue
				}
				buildSpec = spec
				break
			}
		}
		if buildSpec != nil {
			break
		}
	}
	if buildSpec == nil {
		return nil, fmt.Errorf("no build job found in workflow %s", workflow.Name)
	}

	// Get available services from ServiceAndBuildsOptions (service + module)
	services := make([]*commonmodels.ServiceWithModule, 0, len(buildSpec.ServiceAndBuildsOptions))
	for _, opt := range buildSpec.ServiceAndBuildsOptions {
		if opt == nil {
			continue
		}
		services = append(services, &commonmodels.ServiceWithModule{
			ServiceName:   opt.ServiceName,
			ServiceModule: opt.ServiceModule,
		})
	}

	return &GetLarkWorkitemServicesV2Resp{Services: services}, nil
}

type GetLarkWorkitemPRsV2Resp struct {
	PRs []*client.PullRequest `json:"prs"`
}

func GetLarkWorkitemPRsV2(ctx *internalhandler.Context, workspaceID, workItemType, workItemID, serviceName, serviceModule string, page, perPage int) (*GetLarkWorkitemPRsV2Resp, error) {
	templateID, nodeID, err := getWorkItemInfo(ctx, workspaceID, workItemType, workItemID)
	if err != nil {
		return nil, fmt.Errorf("failed to get work item info: %w", err)
	}

	workflowConfig, err := mongodb.NewLarkPluginWorkflowConfigV2Coll().Find(workspaceID, workItemType, templateID, nodeID)
	if err != nil {
		return nil, fmt.Errorf("failed to find workflow config: %w", err)
	}

	stageConfig, err := mongodb.NewLarkPluginStageConfigV2Coll().GetByStage(workspaceID, workflowConfig.StageName)
	if err != nil {
		return nil, fmt.Errorf("failed to get stage config: %w", err)
	}

	workflow, err := mongodb.NewWorkflowV4Coll().Find(workflowConfig.WorkflowName)
	if err != nil {
		return nil, fmt.Errorf("failed to find workflow: %w", err)
	}

	repo, err := getFirstRepoFromWorkflow(workflow, serviceName, serviceModule)
	if err != nil {
		return nil, fmt.Errorf("failed to get first repo: %w", err)
	}

	ch, err := systemconfig.New().GetCodeHost(repo.CodehostID)
	if err != nil {
		return nil, fmt.Errorf("failed to get codehost info: %w", err)
	}

	codehostClient, err := open.OpenClient(ch, ctx.Logger)
	if err != nil {
		return nil, fmt.Errorf("failed to open codehost client: %w", err)
	}

	namespace := repo.RepoNamespace
	if namespace == "" {
		namespace = repo.RepoOwner
	}

	prs, err := codehostClient.ListPrs(client.ListOpt{
		Namespace:    strings.Replace(namespace, "%2F", "/", -1),
		ProjectName:  repo.RepoName,
		TargetBranch: stageConfig.TargetBranch,
		Page:         page,
		PerPage:      perPage,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list PRs: %w", err)
	}

	return &GetLarkWorkitemPRsV2Resp{PRs: prs}, nil
}

type GetLarkWorkitemBranchesV2Resp struct {
	Branches []*client.Branch `json:"branches"`
}

func GetLarkWorkitemBranchesV2(ctx *internalhandler.Context, workspaceID, workItemType, workItemID, serviceName, serviceModule string, page, perPage int) (*GetLarkWorkitemBranchesV2Resp, error) {
	templateID, nodeID, err := getWorkItemInfo(ctx, workspaceID, workItemType, workItemID)
	if err != nil {
		return nil, fmt.Errorf("failed to get work item info: %w", err)
	}

	workflowConfig, err := mongodb.NewLarkPluginWorkflowConfigV2Coll().Find(workspaceID, workItemType, templateID, nodeID)
	if err != nil {
		return nil, fmt.Errorf("failed to find workflow config: %w", err)
	}

	stageConfig, err := mongodb.NewLarkPluginStageConfigV2Coll().GetByStage(workspaceID, workflowConfig.StageName)
	if err != nil {
		return nil, fmt.Errorf("failed to get stage config: %w", err)
	}

	workflow, err := mongodb.NewWorkflowV4Coll().Find(workflowConfig.WorkflowName)
	if err != nil {
		return nil, fmt.Errorf("failed to find workflow: %w", err)
	}

	repo, err := getFirstRepoFromWorkflow(workflow, serviceName, serviceModule)
	if err != nil {
		return nil, fmt.Errorf("failed to get first repo: %w", err)
	}

	ch, err := systemconfig.New().GetCodeHost(repo.CodehostID)
	if err != nil {
		return nil, fmt.Errorf("failed to get codehost info: %w", err)
	}

	codehostClient, err := open.OpenClient(ch, ctx.Logger)
	if err != nil {
		return nil, fmt.Errorf("failed to open codehost client: %w", err)
	}

	namespace := repo.RepoNamespace
	if namespace == "" {
		namespace = repo.RepoOwner
	}

	branches, err := codehostClient.ListBranches(client.ListOpt{
		Namespace:     strings.Replace(namespace, "%2F", "/", -1),
		ProjectName:   repo.RepoName,
		Page:          page,
		PerPage:       perPage,
		MatchBranches: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list branches: %w", err)
	}

	if stageConfig.BranchFilter != "" {
		filtered := make([]*client.Branch, 0)
		for _, branch := range branches {
			matched, err := regexp.MatchString(stageConfig.BranchFilter, branch.Name)
			if err != nil {
				return nil, fmt.Errorf("invalid branch filter regex %q: %w", stageConfig.BranchFilter, err)
			}
			if matched {
				filtered = append(filtered, branch)
			}
		}
		branches = filtered
	}

	return &GetLarkWorkitemBranchesV2Resp{Branches: branches}, nil
}

type GetLarkStageServiceConfigV2Resp struct {
	WorkflowName string                                                     `json:"workflow_name"`
	Configs      []*commonmodels.LarkPluginWorkItemStageWorkflowInputConfig `json:"configs"`
}

func GetLarkStageServiceConfigV2(ctx *internalhandler.Context, workspaceID, stageName, workItemTypeKey, workItemID string) (*GetLarkStageServiceConfigV2Resp, error) {
	templateID, _, err := getWorkItemInfo(ctx, workspaceID, workItemTypeKey, workItemID)
	if err != nil {
		return nil, fmt.Errorf("failed to get work item info: %w", err)
	}
	
	workflowCfg, err := mongodb.NewLarkPluginWorkflowConfigV2Coll().List(&mongodb.ListWorkflowConfigV2Args{
		WorkspaceID: workspaceID,
		WorkItemTypeKey: workItemTypeKey,
		StageName: stageName,
		TemplateID: templateID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list workflow configs: %w", err)
	}
	if len(workflowCfg) == 0 {
		return nil, fmt.Errorf("no workflow config found")
	}
	if len(workflowCfg) > 1 {
		return nil, fmt.Errorf("multiple workflow configs found")
	}
	workflowName := workflowCfg[0].WorkflowName

	configs, err := mongodb.NewLarkPluginWorkItemStageWorkflowInputConfigColl().GetByWorkItem(workspaceID, stageName, workItemTypeKey, workItemID)
	if err != nil {
		return nil, fmt.Errorf("failed to get stage service configs: %w", err)
	}

	return &GetLarkStageServiceConfigV2Resp{
		WorkflowName: workflowName,
		Configs:      configs,
	}, nil
}

type UpdateLarkWorkItemStageWorkflowInputV2Req struct {
	Configs     []*struct {
		ServiceName     string `json:"service_name"`
		ServiceModule   string `json:"service_module"`
		Branch          string `json:"branch"`
		PRs             []int  `json:"prs"`
	} `json:"configs"`
}

func UpdateLarkWorkItemStageWorkflowInputV2(ctx *internalhandler.Context, stageName, workspaceID, workItemTypeKey, workItemID string, req *UpdateLarkWorkItemStageWorkflowInputV2Req) error {
	inputConfig := make([]*commonmodels.LarkPluginWorkItemStageWorkflowInputConfig, 0)

	for _, item := range req.Configs {
		inputConfig = append(inputConfig, &commonmodels.LarkPluginWorkItemStageWorkflowInputConfig{
			StageName: stageName,
			WorkspaceID: workspaceID,
			WorkItemTypeKey: workItemTypeKey,
			WorkItemID: workItemID,
			
			ServiceName: item.ServiceName,
			ServiceModule: item.ServiceModule,
			Branch: item.Branch,
			PRs: item.PRs,
			UpdateBy: ctx.UserName,
		})
	}
	if err := mongodb.NewLarkPluginWorkItemStageWorkflowInputConfigColl().ReplaceByWorkItem(workspaceID, stageName, workItemTypeKey, workItemID, inputConfig); err != nil {
		return fmt.Errorf("failed to update stage service configs: %w", err)
	}
	return nil
}

func ExecuteLarkWorkitemWorkflowV2(ctx *internalhandler.Context, workspaceID, workItemTypeKey, workItemID string) error {
	templateID, nodeID, err := getWorkItemInfo(ctx, workspaceID, workItemTypeKey, workItemID)
	if err != nil {
		return fmt.Errorf("failed to get work item info: %w", err)
	}

	workflowConfig, err := mongodb.NewLarkPluginWorkflowConfigV2Coll().Find(workspaceID, workItemTypeKey, templateID, nodeID)
	if err != nil {
		return fmt.Errorf("failed to find workflow config: %w", err)
	}

	stageConfig, err := mongodb.NewLarkPluginStageConfigV2Coll().GetByStage(workspaceID, workflowConfig.StageName)
	if err != nil {
		return fmt.Errorf("failed to get stage config: %w", err)
	}

	workflow, err := mongodb.NewWorkflowV4Coll().Find(workflowConfig.WorkflowName)
	if err != nil {
		return fmt.Errorf("failed to find workflow: %w", err)
	}

	var inputConfigs []*commonmodels.LarkPluginWorkItemStageWorkflowInputConfig

	if workflowConfig.StageName != "release" {
		inputConfigs, err = mongodb.NewLarkPluginWorkItemStageWorkflowInputConfigColl().GetByWorkItem(workspaceID, workflowConfig.StageName, workItemTypeKey, workItemID)
		if err != nil {
			return fmt.Errorf("failed to get input configs: %w", err)
		}
	} else {
		binds, err := mongodb.NewLarkPluginReleaseWorkItemBindColl().ListReleaseBindItems(workspaceID, workItemID)
		if err != nil {
			return fmt.Errorf("failed to list release bind items: %w", err)
		}

		seen := make(map[string]struct{})
		for _, bind := range binds {
			configs, err := mongodb.NewLarkPluginWorkItemStageWorkflowInputConfigColl().GetByWorkItem(workspaceID, "dev", bind.WorkItemTypeKey, bind.WorkItemID)
			if err != nil {
				return fmt.Errorf("failed to get service configs for bound workitem %s: %w", bind.WorkItemID, err)
			}
			for _, cfg := range configs {
				key := cfg.ServiceName + "/" + cfg.ServiceModule
				if _, exists := seen[key]; exists {
					continue
				}
				seen[key] = struct{}{}
				inputConfigs = append(inputConfigs, &commonmodels.LarkPluginWorkItemStageWorkflowInputConfig{
					ServiceName:   cfg.ServiceName,
					ServiceModule: cfg.ServiceModule,
					Branch:        stageConfig.TargetBranch,
				})
			}
		}
	}

	// Get Lark project info for task metadata
	larkClient := larkplugin.NewClient(config.LarkPluginID(), config.LarkPluginSecret(), ctx.LarkPlugin.LarkType)
	projectResp, err := larkClient.Client.Project.GetProjectDetail(ctx, project.NewGetProjectDetailReqBuilder().
		ProjectKeys([]string{workspaceID}).
		Build(),
		sdkcore.WithAccessToken(ctx.LarkPlugin.PluginAccessToken),
		sdkcore.WithUserKey(ctx.LarkPlugin.UserKey),
	)
	if err != nil {
		return fmt.Errorf("failed to get project detail: %w", err)
	}
	if projectResp.Code() != 0 {
		return fmt.Errorf("failed to get project detail, code: %d, message: %s", projectResp.Code(), projectResp.ErrMsg)
	}
	if projectResp.Data[workspaceID] == nil {
		return fmt.Errorf("project not found")
	}

	projectWorkItemTypes, err := larkClient.Client.Project.ListProjectWorkItemType(ctx, project.NewListProjectWorkItemTypeReqBuilder().
		ProjectKey(workspaceID).
		Build(),
		sdkcore.WithAccessToken(ctx.LarkPlugin.PluginAccessToken),
		sdkcore.WithUserKey(ctx.LarkPlugin.UserKey),
	)
	if err != nil {
		return fmt.Errorf("failed to list project work item type: %w", err)
	}

	workitemTypeApiName := workItemTypeKey
	for _, workitemType := range projectWorkItemTypes.Data {
		if workitemType.TypeKey == workItemTypeKey {
			workitemTypeApiName = workitemType.APIName
			break
		}
	}

	// Build lookup map from user-selected service configs
	serviceConfigMap := make(map[string]*commonmodels.LarkPluginWorkItemStageWorkflowInputConfig)
	for _, sc := range inputConfigs {
		serviceConfigMap[sc.ServiceName+"/"+sc.ServiceModule] = sc
	}

	// Resolve workflow job parameters
	buildSvc := commonservice.NewBuildService()

	workflowCtrl := workflowController.CreateWorkflowController(workflow)

	err = workflowCtrl.SetPreset(nil)
	if err != nil {
		return fmt.Errorf("failed to set preset: %w", err)
	}

	for _, stage := range workflowCtrl.WorkflowV4.Stages {
		for _, job := range stage.Jobs {
			switch job.JobType {
			case aslanconfig.JobZadigBuild:
				buildSpec := &commonmodels.ZadigBuildJobSpec{}
				if err := commonmodels.IToiYaml(job.Spec, buildSpec); err != nil {
					return fmt.Errorf("failed to parse build job spec: %w", err)
				}

				serviceAndBuilds := make([]*commonmodels.ServiceAndBuild, 0)
				for _, opt := range buildSpec.ServiceAndBuildsOptions {
					if opt == nil {
						continue
					}
					sc, ok := serviceConfigMap[opt.ServiceName+"/"+opt.ServiceModule]
					if !ok {
						continue
					}

					buildInfo, err := buildSvc.GetBuild(opt.BuildName, opt.ServiceName, opt.ServiceModule)
					if err != nil {
						return fmt.Errorf("failed to get build %s for %s/%s: %w", opt.BuildName, opt.ServiceName, opt.ServiceModule, err)
					}

					repos := make([]*types.Repository, len(buildInfo.Repos))
					for i, repo := range buildInfo.Repos {
						repoCopy := *repo
						if i == 0 {
							repoCopy.Branch = sc.Branch
							repoCopy.PRs = sc.PRs
						}
						repos[i] = &repoCopy
					}

					serviceAndBuilds = append(serviceAndBuilds, &commonmodels.ServiceAndBuild{
						ServiceName:   opt.ServiceName,
						ServiceModule: opt.ServiceModule,
						BuildName:     opt.BuildName,
						ImageName:     opt.ImageName,
						KeyVals:       opt.KeyVals,
						Repos:         repos,
					})
				}
				buildSpec.ServiceAndBuilds = serviceAndBuilds
				job.Spec = buildSpec

			case aslanconfig.JobZadigDeploy:
				deploySpec := &commonmodels.ZadigDeployJobSpec{}
				if err := commonmodels.IToiYaml(job.Spec, deploySpec); err != nil {
					return fmt.Errorf("failed to parse deploy job spec: %w", err)
				}

				if deploySpec.Source != aslanconfig.SourceFromJob {
					return fmt.Errorf("deploy job %s must use source 'fromjob', got '%s'", job.Name, deploySpec.Source)
				}

				selectedServiceNames := sets.New[string]()
				for _, sc := range inputConfigs {
					selectedServiceNames.Insert(sc.ServiceName)
				}

				existingServiceMap := make(map[string]*commonmodels.DeployServiceInfo)
				for _, svc := range deploySpec.Services {
					existingServiceMap[svc.ServiceName] = svc
				}

				filteredServices := make([]*commonmodels.DeployServiceInfo, 0)
				for _, name := range selectedServiceNames.UnsortedList() {
					if existing, ok := existingServiceMap[name]; ok {
						filteredServices = append(filteredServices, existing)
					} else {
						filteredServices = append(filteredServices, &commonmodels.DeployServiceInfo{
							DeployBasicInfo: commonmodels.DeployBasicInfo{
								ServiceName: name,
							},
						})
					}
				}
				deploySpec.Services = filteredServices
				job.Spec = deploySpec

			case aslanconfig.JobZadigTesting:
				testSpec := &commonmodels.ZadigTestingJobSpec{}
				if err := commonmodels.IToiYaml(job.Spec, testSpec); err != nil {
					return fmt.Errorf("failed to parse test job spec: %w", err)
				}

				if testSpec.TestType == aslanconfig.ServiceTestType {
					filtered := make([]*commonmodels.ServiceAndTest, 0)
					for _, sat := range testSpec.ServiceTestOptions {
						if _, ok := serviceConfigMap[sat.ServiceName+"/"+sat.ServiceModule]; ok {
							filtered = append(filtered, sat)
						}
					}
					testSpec.ServiceAndTests = filtered
				}
				// ProductTestType: keep all defaults
				job.Spec = testSpec
			}
		}
	}

	_, err = workflowservice.CreateWorkflowTaskV4(&workflowservice.CreateWorkflowTaskV4Args{
		Name:                  ctx.UserName,
		Account:               ctx.Account,
		UserID:                ctx.UserID,
		LarkProjectKey:        workspaceID,
		LarkProjectSimpleName: projectResp.Data[workspaceID].SimpleName,
		LarkWorkItemTypeKey:   workItemTypeKey,
		LarkWorkItemAPIName:   workitemTypeApiName,
		LarkWorkItemID:        workItemID,
	}, workflowCtrl.WorkflowV4, ctx.Logger)
	if err != nil {
		return fmt.Errorf("failed to create workflow task: %w", err)
	}

	return nil
}

type LarkWorkItemResp struct {
	ID              int64 `json:"id"`
	Name            string `json:"name"`
	WorkItemTypeKey string `json:"work_item_type_key,omitempty"`
	WorkItemStatus  string `json:"status,omitempty"`
}

func GetLarkReleaseWorkItemsV2(ctx *internalhandler.Context, workspaceID, stageName string) ([]*LarkWorkItemResp, error) {
	if stageName != "release" {
		return nil, fmt.Errorf("stage name must be release")
	}	
	stageConfig, err := mongodb.NewLarkPluginStageConfigV2Coll().GetByStage(workspaceID, stageName)
	if err != nil {
		return nil, fmt.Errorf("failed to get stage config: %w", err)
	}

	larkClient := larkplugin.NewClient(config.LarkPluginID(), config.LarkPluginSecret(), ctx.LarkPlugin.LarkType)
	larkResp, err := larkClient.ClientV2.WorkItem.Filter(ctx, workitem.NewFilterReqBuilder().
		ProjectKey(workspaceID).
		WorkItemTypeKeys([]string{stageConfig.WorkItemTypeKey}).
		Build(),
		sdkcore.WithAccessToken(ctx.LarkPlugin.PluginAccessToken),
		sdkcore.WithUserKey(ctx.LarkPlugin.UserKey),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get lark workitem: %w", err)
	}
	if larkResp.Code() != 0 {
		return nil, fmt.Errorf("failed to get lark workitem, code: %d, message: %s", larkResp.Code(), larkResp.ErrMsg)
	}
	if len(larkResp.Data) == 0 {
		return nil, fmt.Errorf("workitem could not be found")
	}

	resp := make([]*LarkWorkItemResp, 0)

	for _, item := range larkResp.Data {
		resp = append(resp, &LarkWorkItemResp{
			ID: util.GetInt64FromPointer(item.ID),
			Name: util.GetStringFromPointer(item.Name),
			WorkItemTypeKey: util.GetStringFromPointer(item.WorkItemTypeKey),
			WorkItemStatus: util.GetStringFromPointer(item.WorkItemStatus.StateKey),
		})
	}

	return resp, nil
}

type BindLarkWorkitemToReleaseV2Req struct {
	RealeaseItemTypeKey string `json:"realease_item_type_key"`
	RealeaseItemID      string `json:"realease_item_id"`

	WorkItemName string `json:"work_item_name"`
}

func BindLarkWorkitemToReleaseV2(ctx *internalhandler.Context, workspaceID, workItemTypeKey, workItemID string, req *BindLarkWorkitemToReleaseV2Req) error {
	if req.RealeaseItemTypeKey == "" {
		return fmt.Errorf("realease_item_type_key is required")
	}
	if req.RealeaseItemID == "" {
		return fmt.Errorf("realease_item_id is required")
	}

	err := mongodb.NewLarkPluginReleaseWorkItemBindColl().CreateWorkItemBind(&commonmodels.LarkPluginReleaseWorkItemBind{
		WorkspaceID:        workspaceID,
		WorkItemTypeKey:    workItemTypeKey,
		WorkItemID:         workItemID,
		WorkItemName:       req.WorkItemName,
		ReleaseItemID:      req.RealeaseItemID,
		ReleaseItemTypeKey: req.RealeaseItemTypeKey,
	})
	
	if err != nil {
		return fmt.Errorf("failed to create workitem bind: %w", err)
	}

	return nil
}

type GetLarkWorkitemBindV2Resp struct {
	Bound bool `json:"bound"`

	RealeaseItemTypeKey string `json:"work_item_type_key"`
	RealeaseItemID      string `json:"realease_item_id"`
}

func GetLarkWorkitemBindV2(ctx *internalhandler.Context, workspaceID, workItemTypeKey, workItemID string) (*GetLarkWorkitemBindV2Resp, error) {
	bind, err := mongodb.NewLarkPluginReleaseWorkItemBindColl().GetWorkItemBind(workspaceID, workItemTypeKey, workItemID)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return &GetLarkWorkitemBindV2Resp{
				Bound: false,
			}, nil
		}
		return nil, fmt.Errorf("failed to get workitem bind: %w", err)
	}

	return &GetLarkWorkitemBindV2Resp{
		Bound: true,
		RealeaseItemTypeKey: bind.ReleaseItemTypeKey,
		RealeaseItemID:      bind.ReleaseItemID,
	}, nil
}

func DeleteLarkWorkitemBindV2(ctx *internalhandler.Context, workspaceID, workItemTypeKey, workItemID string) error {
	err := mongodb.NewLarkPluginReleaseWorkItemBindColl().DeleteWorkItemBind(workspaceID, workItemTypeKey, workItemID)
	if err != nil {
		return fmt.Errorf("failed to delete workitem bind: %w", err)
	}
	return nil
}

type ListLarkReleaseBindItemsV2Resp struct {
	Items []*LarkReleaseBindItem `json:"items"`
}

type LarkReleaseBindItem struct {
	WorkItemTypeKey string `json:"work_item_type_key"`
	WorkItemID      string `json:"work_item_id"`
	WorkItemName    string `json:"work_item_name"`

	Branch          string `json:"branch"`
	Services        []*commonmodels.ServiceWithModule `json:"services"`
}

func ListLarkReleaseBindItemsV2(ctx *internalhandler.Context, workspaceID, releaseItemID string) (*ListLarkReleaseBindItemsV2Resp, error) {
	binds, err := mongodb.NewLarkPluginReleaseWorkItemBindColl().ListReleaseBindItems(workspaceID, releaseItemID)
	if err != nil {
		return nil, fmt.Errorf("failed to list release bind items: %w", err)
	}

	releaseStageSetting, err := mongodb.NewLarkPluginStageConfigV2Coll().GetByStage(workspaceID, "release")
	if err != nil {
		return nil, fmt.Errorf("failed to get release stage setting: %w", err)
	}

	resp := make([]*LarkReleaseBindItem, 0)

	for _, bind := range binds {
		item := &LarkReleaseBindItem{
			WorkItemTypeKey: bind.WorkItemTypeKey,
			WorkItemID: bind.WorkItemID,
			WorkItemName: bind.WorkItemName,
			Branch: releaseStageSetting.TargetBranch,
		}

		services := make([]*commonmodels.ServiceWithModule, 0)

		configs, err := mongodb.NewLarkPluginWorkItemStageWorkflowInputConfigColl().GetByWorkItem(workspaceID, "dev", bind.WorkItemTypeKey, bind.WorkItemID)
		if err != nil {
			return nil, fmt.Errorf("failed to get stage service configs: %w", err)
		}
		
		for _, cfg := range configs {
			services = append(services, &commonmodels.ServiceWithModule{
				ServiceName:   cfg.ServiceName,
				ServiceModule: cfg.ServiceModule,
			})
		}

		item.Services = services
		resp = append(resp, item)
	}

	return &ListLarkReleaseBindItemsV2Resp{
		Items: resp,
	}, nil
}

type LarkWorkitemStage struct {
	// stageName/workspaceID is the foreign key combination to link to the LarkPluginWorkflowConfigV2
	StageName    string             `json:"stage_name"`
	WorkspaceID  string             `json:"workspace_id"`

	Items        []*LarkWorkItemSpec `json:"items"`
}

type LarkWorkItemSpec struct {
	// when the stage name is not release, work item type/templateID/nodeID together forms the unique node setting
	WorkItemTypeKey   string             `json:"work_item_type_key"`
	WorkItemType      string             `json:"work_item_type"`
	TemplateID        int64              `json:"template_id"`
	TemplateName      string             `json:"template_name"`
	NodeID            string             `json:"node_id"`
	NodeName          string             `json:"node_name"`
}

func ListLarkWorkitemStagesV2(ctx *internalhandler.Context, workspaceID, workItemTypeKey, workItemID string) ([]*LarkWorkitemStage, error) {
	templateID, _, err := getWorkItemInfo(ctx, workspaceID, workItemTypeKey, workItemID)
	if err != nil {
		return nil, fmt.Errorf("failed to get work item info: %w", err)
	}

	workflowConfigs, err := mongodb.NewLarkPluginWorkflowConfigV2Coll().List(&mongodb.ListWorkflowConfigV2Args{
		WorkspaceID: workspaceID,
		WorkItemTypeKey: workItemTypeKey,
		TemplateID: templateID,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to find workflow config: %w", err)
	}

	stageMap := make(map[string]*LarkWorkitemStage)
	for _, cfg := range workflowConfigs {
		stage, ok := stageMap[cfg.StageName]
		if !ok {
			stage = &LarkWorkitemStage{
				StageName:   cfg.StageName,
				WorkspaceID: cfg.WorkspaceID,
				Items:       make([]*LarkWorkItemSpec, 0),
			}
			stageMap[cfg.StageName] = stage
		}
		stage.Items = append(stage.Items, &LarkWorkItemSpec{
			WorkItemTypeKey: cfg.WorkItemTypeKey,
			WorkItemType:    cfg.WorkItemType,
			TemplateID:      cfg.TemplateID,
			TemplateName:    cfg.TemplateName,
			NodeID:          cfg.NodeID,
			NodeName:        cfg.NodeName,
		})
	}

	resp := make([]*LarkWorkitemStage, 0, len(stageMap))
	for _, stage := range stageMap {
		resp = append(resp, stage)
	}

	return resp, nil
}

type GetLarkWorkitemInfoV2Resp struct {
	TemplateID int64  `json:"template_id"`
	NodeID     string `json:"node_id"`
}

func GetLarkWorkitemInfoV2(ctx *internalhandler.Context, workspaceID, workItemTypeKey, workItemID string) (*GetLarkWorkitemInfoV2Resp, error) {
	templateID, nodeID, err := getWorkItemInfo(ctx, workspaceID, workItemTypeKey, workItemID)
	if err != nil {
		return nil, fmt.Errorf("failed to get work item info: %w", err)
	}

	return &GetLarkWorkitemInfoV2Resp{
		TemplateID: templateID,
		NodeID: nodeID,
	}, nil
}