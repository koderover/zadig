/*
Copyright 2024 The KodeRover Authors.

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
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	sdkcore "github.com/larksuite/project-oapi-sdk-golang/core"
	"github.com/larksuite/project-oapi-sdk-golang/service/project"
	"github.com/larksuite/project-oapi-sdk-golang/v2/service/workitem"
	"go.mongodb.org/mongo-driver/mongo"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/koderover/zadig/v2/pkg/config"
	aslanconfig "github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/code/client"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/code/client/open"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	workflowservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/workflow/service/workflow"
	workflowController "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/workflow/service/workflow/controller"
	"github.com/koderover/zadig/v2/pkg/shared/client/systemconfig"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	"github.com/koderover/zadig/v2/pkg/tool/cache"
	"github.com/koderover/zadig/v2/pkg/tool/larkplugin"
	"github.com/koderover/zadig/v2/pkg/tool/meego"
	"github.com/koderover/zadig/v2/pkg/types"
)

type LarkLoginRequest struct {
	LarkType string `json:"lark_type"`
	Code     string `json:"code"`
}

type LarkLoginResponse struct {
	UserAccessTokenExpireTime   int    `json:"user_access_token_expire_time"`
	UserAccessToken             string `json:"user_access_token"`
	RefreshToken                string `json:"refresh_token"`
	RefreshTokenExpireTime      int    `json:"refresh_token_expire_time"`
	UserKey                     string `json:"user_key"`
	TenantKey                   string `json:"saas_tenant_key"`
	LarkType                    string `json:"lark_type"`
	PluginAccessToken           string `json:"plugin_access_token"`
	PluginAccessTokenExpireTime int    `json:"plugin_access_token_expire_time"`
}

func LarkLogin(ctx *internalhandler.Context, workspaceID string, req *LarkLoginRequest) (*LarkLoginResponse, error) {
	lark := larkplugin.NewClient(config.LarkPluginID(), config.LarkPluginSecret(), req.LarkType)
	resp, err := lark.Client.Plugin.GetPluginToken(ctx, config.LarkPluginAccessTokenType())
	if err != nil {
		return nil, fmt.Errorf("failed to get lark plugin token: %w", err)
	}

	if resp.Error != nil && resp.Error.Code != 0 {
		return nil, fmt.Errorf("failed to get lark plugin token, error code: %d, message: %s", resp.Error.Code, resp.Error.Msg)
	}

	resp2, err := lark.Client.Plugin.GetUserPluginToken(ctx, req.Code, sdkcore.WithAccessToken(resp.Data.Token))
	if err != nil {
		return nil, fmt.Errorf("failed to get lark user plugin token: %w", err)
	}

	if resp2.Error != nil && resp2.Error.Code != 0 {
		return nil, fmt.Errorf("failed to get lark user plugin token, error code: %d, message: %s", resp2.Error.Code, resp2.Error.Msg)
	}

	resp3 := &LarkLoginResponse{
		UserAccessTokenExpireTime:   resp2.Data.ExpireTime,
		UserAccessToken:             resp2.Data.Token,
		RefreshToken:                resp2.Data.RefreshToken,
		RefreshTokenExpireTime:      resp2.Data.RefreshTokenExpireTime,
		UserKey:                     resp2.Data.UserKey,
		TenantKey:                   resp2.Data.TenantKey,
		PluginAccessToken:           resp.Data.Token,
		PluginAccessTokenExpireTime: resp.Data.ExpireTime,
		LarkType:                    req.LarkType,
	}

	tokenData, err := json.Marshal(resp3)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal lark user plugin token: %w", err)
	}

	err = cache.NewRedisCache(config.RedisCommonCacheTokenDB()).Write(fmt.Sprintf("lark-plugin-user-token-%s-%s", workspaceID, resp2.Data.UserKey), string(tokenData), time.Duration(resp3.PluginAccessTokenExpireTime-5)*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to write lark user plugin token to cache: %w", err)
	}

	return resp3, nil
}

func GetLarkAuthConfig(ctx *internalhandler.Context, workspaceID string) (*commonmodels.LarkPluginAuthConfig, error) {
	config, err := mongodb.NewLarkPluginAuthConfigColl().Get(workspaceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get lark plugin auth config: %w", err)
	}
	return config, nil
}

func UpdateLarkAuthConfig(ctx *internalhandler.Context, config *commonmodels.LarkPluginAuthConfig) error {
	err := mongodb.NewLarkPluginAuthConfigColl().Update(config)
	if err != nil {
		return fmt.Errorf("failed to update lark plugin auth config: %w", err)
	}
	return nil
}

type GetLarkWorkflowConfigResp struct {
	Configs []*commonmodels.LarkPluginWorkflowConfig `json:"configs"`
}

func GetLarkWorkflowConfig(ctx *internalhandler.Context) (*GetLarkWorkflowConfigResp, error) {
	workspaceID := ctx.LarkPlugin.ProjectKey
	configs, err := mongodb.NewLarkPluginWorkflowConfigColl().Get(workspaceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get lark plugin workflow config: %w", err)
	}

	resp := &GetLarkWorkflowConfigResp{
		Configs: configs,
	}

	return resp, nil
}

type UpdateLarkWorkflowConfigRequest struct {
	WorkspaceID string                                   `json:"workspace_id"`
	Configs     []*commonmodels.LarkPluginWorkflowConfig `json:"configs"`
}

func UpdateLarkWorkflowConfig(ctx *internalhandler.Context, req *UpdateLarkWorkflowConfigRequest) error {
	updateWorkitemTypeKeySet := sets.Set[string]{}
	for _, config := range req.Configs {
		config.WorkspaceID = req.WorkspaceID
		updateWorkitemTypeKeySet.Insert(config.WorkItemTypeKey)
	}

	origConfigs, err := mongodb.NewLarkPluginWorkflowConfigColl().Get(req.WorkspaceID)
	if err != nil {
		return fmt.Errorf("failed to get lark plugin workflow config: %w", err)
	}

	// delete the configs that are not in the request
	deleteWorkitemTypeKeys := make([]string, 0)
	for _, config := range origConfigs {
		if !updateWorkitemTypeKeySet.Has(config.WorkItemTypeKey) {
			deleteWorkitemTypeKeys = append(deleteWorkitemTypeKeys, config.WorkItemTypeKey)
		}
	}

	err = mongodb.NewLarkPluginWorkflowConfigColl().Delete(deleteWorkitemTypeKeys)
	if err != nil {
		return fmt.Errorf("failed to delete lark plugin workflow config: %w", err)
	}

	err = mongodb.NewLarkPluginWorkflowConfigColl().Update(req.Configs)
	if err != nil {
		return fmt.Errorf("failed to update lark plugin workflow config: %w", err)
	}

	return nil
}

type GetLarkWorkitemTypeResponse struct {
	WorkItemTypes []workitem.WorkItemKeyType `json:"work_item_types"`
}

func GetLarkWorkitemType(ctx *internalhandler.Context) (*GetLarkWorkitemTypeResponse, error) {
	projectKey := ctx.LarkPlugin.ProjectKey
	client := larkplugin.NewClient(config.LarkPluginID(), config.LarkPluginSecret(), ctx.LarkPlugin.LarkType)
	larkResp, err := client.ClientV2.WorkItem.QueryAWorkItemTypes(ctx, workitem.NewQueryAWorkItemTypesReqBuilder().
		ProjectKey(projectKey).
		Build(),
		sdkcore.WithAccessToken(ctx.LarkPlugin.PluginAccessToken),
		sdkcore.WithUserKey(ctx.LarkPlugin.UserKey),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get lark workitem type: %w", err)
	}

	if larkResp.Code() != 0 {
		return nil, fmt.Errorf("failed to get lark workitem type, code: %d, message: %s", larkResp.Code(), larkResp.ErrMsg)
	}

	resp := &GetLarkWorkitemTypeResponse{
		WorkItemTypes: larkResp.Data,
	}

	return resp, nil
}

type GetLarkWorkitemTypeDetailResponse struct {
	Name           string `json:"name"`
	APIName        string `json:"api_name"`
	TypeKey        string `json:"type_key"`
	FlowMode       string `json:"flow_mode"`
	IsPinned       bool   `json:"is_pinned"`
	Description    string `json:"description"`
	IsDisabled     bool   `json:"is_disabled"`
	EnableSchedule bool   `json:"enable_schedule"`
}

func GetLarkWorkitemTypeDetail(ctx *internalhandler.Context, workitemTypeKey string) (*GetLarkWorkitemTypeDetailResponse, error) {
	projectKey := ctx.LarkPlugin.ProjectKey
	client := larkplugin.NewClient(config.LarkPluginID(), config.LarkPluginSecret(), ctx.LarkPlugin.LarkType)
	larkResp, err := client.ClientV2.WorkItem.GetWorkItemTypeInfoByKey(ctx, workitem.NewGetWorkItemTypeInfoByKeyReqBuilder().
		ProjectKey(projectKey).
		WorkItemTypeKey(workitemTypeKey).
		Build(),
		sdkcore.WithAccessToken(ctx.LarkPlugin.PluginAccessToken),
		sdkcore.WithUserKey(ctx.LarkPlugin.UserKey),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get lark workitem type: %w", err)
	}

	if larkResp.Code() != 0 {
		return nil, fmt.Errorf("failed to get lark workitem type, code: %d, message: %s", larkResp.Code(), larkResp.ErrMsg)
	}

	resp := &GetLarkWorkitemTypeDetailResponse{
		Name:           *larkResp.Data.Name,
		APIName:        *larkResp.Data.APIName,
		TypeKey:        *larkResp.Data.TypeKey,
		FlowMode:       *larkResp.Data.FlowMode,
		IsPinned:       *larkResp.Data.IsPinned,
		Description:    *larkResp.Data.Description,
		IsDisabled:     *larkResp.Data.IsDisabled,
		EnableSchedule: *larkResp.Data.EnableSchedule,
	}

	return resp, nil
}

type GetLarkWorkitemTypeTemplateResponse struct {
	Templates []*LarkWorkitemTypeTemplate `json:"templates"`
}

type LarkWorkitemTypeTemplate struct {
	TemplateID   string `json:"template_id"`
	TemplateName string `json:"template_name"`
	IsDisabled   int32  `json:"is_disabled"`
	Version      int64  `json:"version"`
	UniqueKey    string `json:"unique_key"`
	TemplateKey  string `json:"template_key"`
}

func GetLarkWorkitemTypeTemplate(ctx *internalhandler.Context, workitemTypeKey string) (*GetLarkWorkitemTypeTemplateResponse, error) {
	projectKey := ctx.LarkPlugin.ProjectKey
	client := larkplugin.NewClient(config.LarkPluginID(), config.LarkPluginSecret(), ctx.LarkPlugin.LarkType)
	larkResp, err := client.ClientV2.WorkItem.ListTemplateConf(ctx, workitem.NewListTemplateConfReqBuilder().
		ProjectKey(projectKey).
		WorkItemTypeKey(workitemTypeKey).
		Build(),
		sdkcore.WithAccessToken(ctx.LarkPlugin.PluginAccessToken),
		sdkcore.WithUserKey(ctx.LarkPlugin.UserKey),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get lark workitem type template: %w", err)
	}

	if larkResp.Code() != 0 {
		return nil, fmt.Errorf("failed to get lark workitem type template, code: %d, message: %s", larkResp.Code(), larkResp.ErrMsg)
	}

	templates := make([]*LarkWorkitemTypeTemplate, 0)
	for _, template := range larkResp.Data {
		templates = append(templates, &LarkWorkitemTypeTemplate{
			TemplateID:   *template.TemplateID,
			TemplateName: *template.TemplateName,
			IsDisabled:   *template.IsDisabled,
			Version:      *template.Version,
			UniqueKey:    *template.UniqueKey,
			TemplateKey:  *template.TemplateKey,
		})
	}

	resp := &GetLarkWorkitemTypeTemplateResponse{
		Templates: templates,
	}

	return resp, nil
}

type GetLarkWorkitemTypeNodesResponse struct {
	Nodes []*LarkWorkitemTypeNode `json:"nodes"`
}

type LarkWorkitemTypeNode struct {
	// 节点 ID
	StateKey string `json:"state_key"`
	Name     string `json:"name"`
	Pattern  string `json:"pattern"`
}

func GetLarkWorkitemTypeNodes(ctx *internalhandler.Context, workitemTypeKey string, templateID int64) (*GetLarkWorkitemTypeNodesResponse, error) {
	projectKey := ctx.LarkPlugin.ProjectKey
	client := larkplugin.NewClient(config.LarkPluginID(), config.LarkPluginSecret(), ctx.LarkPlugin.LarkType)
	larkResp, err := client.ClientV2.WorkItem.QueryTemplateDetail(ctx, (*workitem.QueryTemplateDetailReq)(workitem.NewQueryTemplateDetailReqBuilder().
		TemplateID(templateID).
		ProjectKey(projectKey).
		Build()),
		sdkcore.WithAccessToken(ctx.LarkPlugin.PluginAccessToken),
		sdkcore.WithUserKey(ctx.LarkPlugin.UserKey),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query template detail: %w", err)
	}

	if larkResp.Code() != 0 {
		return nil, fmt.Errorf("failed to query template detail, code: %d, message: %s", larkResp.Code(), larkResp.ErrMsg)
	}

	nodeList := make([]*LarkWorkitemTypeNode, 0)
	for _, workflowConf := range larkResp.Data.WorkflowConfs {
		node := &LarkWorkitemTypeNode{
			StateKey: *workflowConf.StateKey,
			Name:     *workflowConf.Name,
			Pattern:  string(meego.WorkItemPatternNode),
		}
		nodeList = append(nodeList, node)
	}

	for _, stateFlowConfs := range larkResp.Data.StateFlowConfs {
		node := &LarkWorkitemTypeNode{
			StateKey: *stateFlowConfs.StateKey,
			Name:     *stateFlowConfs.Name,
			Pattern:  string(meego.WorkItemPatternState),
		}
		nodeList = append(nodeList, node)
	}

	resp := &GetLarkWorkitemTypeNodesResponse{
		Nodes: nodeList,
	}

	return resp, nil
}

type GetLarkWorkitemWorkflowResponse struct {
	Nodes []*NodeWorkflows `json:"nodes"`
}

type Node struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	IsCurrent bool   `json:"is_current"`
}

type NodeWorkflowWithAction struct {
	Workflow   *models.WorkflowV4 `json:"workflow"`
	CanExecute bool               `json:"can_execute"`
}

type NodeWorkflows struct {
	Node      *Node                     `json:"node"`
	Workflows []*NodeWorkflowWithAction `json:"workflows"`
}

func GetLarkWorkitemWorkflow(ctx *internalhandler.Context, workItemType, workItemID string, isAdmin bool, authProjects, authWorkflows sets.Set[string]) (*GetLarkWorkitemWorkflowResponse, error) {
	workItemIDInt, err := strconv.ParseInt(workItemID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse workitem id: %w", err)
	}

	client := larkplugin.NewClient(config.LarkPluginID(), config.LarkPluginSecret(), ctx.LarkPlugin.LarkType)
	larkResp, err := client.ClientV2.WorkItem.GetWorkItemsByIds(ctx, workitem.NewGetWorkItemsByIdsReqBuilder().
		ProjectKey(ctx.LarkPlugin.ProjectKey).
		WorkItemTypeKey(workItemType).
		WorkItemIDs([]int64{workItemIDInt}).
		Build(),
		sdkcore.WithAccessToken(ctx.LarkPlugin.PluginAccessToken),
		sdkcore.WithUserKey(ctx.LarkPlugin.UserKey),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get lark workitem type node: %w", err)
	}

	if larkResp.Code() != 0 {
		return nil, fmt.Errorf("failed to get lark workitem type node, code: %d, message: %s", larkResp.Code(), larkResp.ErrMsg)
	}

	if len(larkResp.Data) == 0 {
		return nil, fmt.Errorf("workitem could not be found")
	}

	workItem := larkResp.Data[0]

	larkResp2, err := client.ClientV2.WorkItem.QueryTemplateDetail(ctx, workitem.NewQueryTemplateDetailReqBuilder().
		TemplateID(*workItem.TemplateID).
		ProjectKey(ctx.LarkPlugin.ProjectKey).
		Build(),
		sdkcore.WithAccessToken(ctx.LarkPlugin.PluginAccessToken),
		sdkcore.WithUserKey(ctx.LarkPlugin.UserKey),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query template detail: %w", err)
	}
	if larkResp2.Code() != 0 {
		return nil, fmt.Errorf("failed to query template detail, code: %d, error: %s", larkResp2.Code(), larkResp2.ErrMsg)
	}

	templateWorkflowNodes := []*Node{}
	if *workItem.Pattern == string(meego.WorkItemPatternNode) {
		for _, workflowConf := range larkResp2.Data.WorkflowConfs {
			node := &Node{
				ID: *workflowConf.StateKey,
			}
			templateWorkflowNodes = append(templateWorkflowNodes, node)
		}
	} else if *workItem.Pattern == string(meego.WorkItemPatternState) {
		for _, stateFlow := range larkResp2.Data.StateFlowConfs {
			node := &Node{
				ID: *stateFlow.StateKey,
			}
			templateWorkflowNodes = append(templateWorkflowNodes, node)
		}
	}

	workflowConfig, err := mongodb.NewLarkPluginWorkflowConfigColl().GetWorkItemTypeConfig(ctx.LarkPlugin.ProjectKey, workItemType)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return &GetLarkWorkitemWorkflowResponse{
				Nodes: make([]*NodeWorkflows, 0),
			}, nil
		}
		return nil, fmt.Errorf("failed to get lark plugin workflow config: %w", err)
	}

	// 节点ID和节点工作流的map
	NodeIDWorkflowMap := make(map[string]*NodeWorkflows)
	// 工作流名称和节点ID的map
	NodeIDWorkflowNamesMap := make(map[string][]string)
	// 用于搜索数据库中节点配置的工作流
	nodeConfigWorkflows := make([]mongodb.WorkflowV4, 0)
	for _, node := range templateWorkflowNodes {
		for _, workflowConfigNode := range workflowConfig.Nodes {
			if workflowConfigNode.TemplateID != *workItem.TemplateID {
				continue
			}

			// 仅列出配置了工作流的节点
			if node.ID == workflowConfigNode.NodeID {
				isCurrent := false

				if *workItem.Pattern == string(meego.WorkItemPatternNode) {
					for _, currentNode := range workItem.CurrentNodes {
						if node.ID == *currentNode.ID {
							isCurrent = true
							break
						}
					}
				} else if *workItem.Pattern == string(meego.WorkItemPatternState) {
					if *workItem.WorkItemStatus.StateKey == node.ID {
						isCurrent = true
					}
				} else {
					return nil, fmt.Errorf("Unsupport pattern %s", *workItem.Pattern)
				}

				NodeIDWorkflowMap[workflowConfigNode.NodeID] = &NodeWorkflows{
					Node: &Node{
						ID:        workflowConfigNode.NodeID,
						Name:      workflowConfigNode.NodeName,
						IsCurrent: isCurrent,
					},
					Workflows: make([]*NodeWorkflowWithAction, 0),
				}

				nodeConfigWorkflows = append(nodeConfigWorkflows, mongodb.WorkflowV4{
					Name:        workflowConfigNode.WorkflowName,
					ProjectName: workflowConfigNode.ProjectKey,
				})

				NodeIDWorkflowNamesMap[workflowConfigNode.NodeID] = append(NodeIDWorkflowNamesMap[workflowConfigNode.NodeID], workflowConfigNode.WorkflowName)
			}
		}
	}

	workflows, err := mongodb.NewWorkflowV4Coll().ListByWorkflows(mongodb.ListWorkflowV4Opt{
		Workflows: nodeConfigWorkflows,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list workflows: %w", err)
	}

	workflowMap := make(map[string]*models.WorkflowV4)
	for _, workflow := range workflows {
		workflowMap[workflow.Name] = workflow
	}

	for nodeID, node := range NodeIDWorkflowMap {
		workflowNames := NodeIDWorkflowNamesMap[nodeID]
		for _, workflowName := range workflowNames {
			workflow, ok := workflowMap[workflowName]
			if !ok {
				continue
			}

			workflowWithAction := &NodeWorkflowWithAction{
				Workflow: workflow,
			}

			if isAdmin {
				workflowWithAction.CanExecute = true
			} else if authProjects.Has(workflow.Project) {
				workflowWithAction.CanExecute = true
			} else if authWorkflows.Has(workflow.Name) {
				workflowWithAction.CanExecute = true
			}

			node.Workflows = append(node.Workflows, workflowWithAction)
		}
	}

	// 按照templateWorkflowConfig的顺序对NodeIDWorkflowMap进行排序
	resp := make([]*NodeWorkflows, 0)
	for _, node := range templateWorkflowNodes {
		if nodeWorkflows, exists := NodeIDWorkflowMap[node.ID]; exists {
			resp = append(resp, nodeWorkflows)
		}
	}

	return &GetLarkWorkitemWorkflowResponse{
		Nodes: resp,
	}, nil
}

type LarkWorkitemWorkflowTask struct {
	TaskID              int64                             `json:"task_id"`
	TaskCreator         string                            `json:"task_creator"`
	ProjectName         string                            `json:"project_name"`
	WorkflowName        string                            `json:"workflow_name"`
	WorkflowDisplayName string                            `json:"workflow_display_name"`
	Remark              string                            `json:"remark"`
	Status              aslanconfig.Status                `json:"status"`
	Reverted            bool                              `json:"reverted"`
	CreateTime          int64                             `json:"create_time,omitempty"`
	StartTime           int64                             `json:"start_time,omitempty"`
	EndTime             int64                             `json:"end_time,omitempty"`
	Hash                string                            `json:"hash"`
	Repos               []*types.Repository               `json:"repos"`
	ServiceModules      []*commonmodels.ServiceWithModule `json:"service_modules"`
	DeployEnvs          []*commonmodels.WorkflowEnv       `json:"deploy_envs"`
}

type ListLarkWorkitemWorkflowTaskResponse struct {
	Tasks []*LarkWorkitemWorkflowTask `json:"tasks"`
	Count int64                       `json:"count"`
}

func ListLarkWorkitemWorkflowTask(ctx *internalhandler.Context, workItemTypeKey, workItemID, workflowName string, pageNum, pageSize int) (*ListLarkWorkitemWorkflowTaskResponse, error) {
	tasks, count, err := mongodb.NewworkflowTaskv4Coll().List(&mongodb.ListWorkflowTaskV4Option{
		WorkflowName:        workflowName,
		LarkProjectKey:      ctx.LarkPlugin.ProjectKey,
		LarkWorkItemTypeKey: workItemTypeKey,
		LarkWorkItemID:      workItemID,
		Skip:                (pageNum - 1) * pageSize,
		Limit:               pageSize,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list workflow task: %w", err)
	}

	envMap := make(map[string]*commonmodels.Product)
	resp := make([]*LarkWorkitemWorkflowTask, 0)
	for _, task := range tasks {
		respTask := &LarkWorkitemWorkflowTask{
			TaskID:              task.TaskID,
			TaskCreator:         task.TaskCreator,
			ProjectName:         task.ProjectName,
			WorkflowName:        task.WorkflowName,
			WorkflowDisplayName: task.WorkflowDisplayName,
			Remark:              task.Remark,
			Status:              task.Status,
			Reverted:            task.Reverted,
			CreateTime:          task.CreateTime,
			StartTime:           task.StartTime,
			EndTime:             task.EndTime,
			Hash:                task.Hash,
		}

		repoSet := sets.New[string]()
		serviceModuleSet := sets.New[string]()
		deployEnvSet := sets.New[string]()

		repos := make([]*types.Repository, 0)
		serviceModules := make([]*commonmodels.ServiceWithModule, 0)
		deployEnvs := make([]*commonmodels.WorkflowEnv, 0)

		updateRepos := func(repo *types.Repository) {
			if repoSet.Has(repo.GetKey()) {
				return
			}
			repoSet.Insert(repo.GetKey())
			repos = append(repos, repo)
		}

		updateServiceModules := func(serviceModule *commonmodels.ServiceWithModule) {
			if serviceModuleSet.Has(serviceModule.GetKey()) {
				return
			}
			serviceModuleSet.Insert(serviceModule.GetKey())
			serviceModules = append(serviceModules, serviceModule)
		}

		updateDeployEnvs := func(deployEnv *commonmodels.WorkflowEnv) {
			if deployEnvSet.Has(deployEnv.EnvName) {
				return
			}
			deployEnvSet.Insert(deployEnv.EnvName)
			deployEnvs = append(deployEnvs, deployEnv)
		}

		for _, stage := range task.WorkflowArgs.Stages {
			for _, job := range stage.Jobs {
				if job.Skipped {
					continue
				}

				switch job.JobType {
				case aslanconfig.JobZadigBuild:
					build := new(commonmodels.ZadigBuildJobSpec)
					if err := commonmodels.IToi(job.Spec, build); err != nil {
						return nil, fmt.Errorf("failed to convert job spec to build job spec: %w", err)
					}

					for _, serviceAndBuild := range build.ServiceAndBuilds {
						sm := &commonmodels.ServiceWithModule{
							ServiceName:   serviceAndBuild.ServiceName,
							ServiceModule: serviceAndBuild.ServiceModule,
						}
						updateServiceModules(sm)

						for _, repo := range serviceAndBuild.Repos {
							updateRepos(repo)
						}
					}
				case aslanconfig.JobZadigDeploy:
					deploy := new(commonmodels.ZadigDeployJobSpec)
					if err := commonmodels.IToi(job.Spec, deploy); err != nil {
						return nil, fmt.Errorf("failed to convert job spec to deploy job spec: %w", err)
					}

					for _, svc := range deploy.Services {
						for _, module := range svc.Modules {
							sm := &commonmodels.ServiceWithModule{
								ServiceName:   svc.ServiceName,
								ServiceModule: module.ServiceModule,
							}
							updateServiceModules(sm)
						}
					}
					env := &commonmodels.WorkflowEnv{
						EnvName:    deploy.Env,
						Production: deploy.Production,
						EnvAlias:   commonutil.GetEnvAlias(commonutil.GetEnvInfoNoErr(task.ProjectName, deploy.Env, envMap)),
					}
					updateDeployEnvs(env)
				}
			}
		}

		respTask.Repos = repos
		respTask.ServiceModules = serviceModules
		respTask.DeployEnvs = deployEnvs

		resp = append(resp, respTask)
	}

	return &ListLarkWorkitemWorkflowTaskResponse{
		Tasks: resp,
		Count: count,
	}, nil
}

func ExecuteLarkWorkitemWorkflow(ctx *internalhandler.Context, workItemTypeKey, workItemID string, args *models.WorkflowV4) error {
	projectKey := ctx.LarkPlugin.ProjectKey
	client := larkplugin.NewClient(config.LarkPluginID(), config.LarkPluginSecret(), ctx.LarkPlugin.LarkType)
	resp, err := client.Client.Project.GetProjectDetail(ctx, project.NewGetProjectDetailReqBuilder().
		ProjectKeys([]string{projectKey}).
		Build(),
		sdkcore.WithAccessToken(ctx.LarkPlugin.PluginAccessToken),
		sdkcore.WithUserKey(ctx.LarkPlugin.UserKey),
	)
	if err != nil {
		return fmt.Errorf("failed to get project detail: %w", err)
	}
	if resp.Code() != 0 {
		return fmt.Errorf("failed to get project detail, code: %d, message: %s", resp.Code(), resp.ErrMsg)
	}
	if resp.Data[projectKey] == nil {
		return fmt.Errorf("project not found")
	}

	projectWorkItemTypes, err := client.Client.Project.ListProjectWorkItemType(ctx, project.NewListProjectWorkItemTypeReqBuilder().
		ProjectKey(projectKey).
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

	_, err = workflowservice.CreateWorkflowTaskV4(&workflowservice.CreateWorkflowTaskV4Args{
		Name:                  ctx.UserName,
		Account:               ctx.Account,
		UserID:                ctx.UserID,
		LarkProjectKey:        projectKey,
		LarkProjectSimpleName: resp.Data[projectKey].SimpleName,
		LarkWorkItemTypeKey:   workItemTypeKey,
		LarkWorkItemAPIName:   workitemTypeApiName,
		LarkWorkItemID:        workItemID,
	}, args, ctx.Logger)
	if err != nil {
		return fmt.Errorf("failed to create workflow task: %w", err)
	}

	return nil
}

type GetLarkWorkflowConfigV2Resp struct {
	StageName    string                                         `json:"stage_name"`
	WorkspaceID  string                                         `json:"workspace_id"`
	UpdateTime   int64                                          `json:"update_time"`
	CodeSource   string                                         `json:"code_source"`
	TemplateID   int64                                          `json:"template_id"`
	TemplateName string                                         `json:"template_name"`
	BranchFilter string                                         `json:"branch_filter"`
	TargetBranch string                                         `json:"target_branch"`
	Nodes        []*commonmodels.LarkPluginWorkflowConfigNodeV2 `json:"nodes"`
}

func GetLarkWorkflowConfigV2(ctx *internalhandler.Context, workspaceID, stageName string) (*GetLarkWorkflowConfigV2Resp, error) {

	cfg, err := mongodb.NewLarkPluginWorkflowConfigV2Coll().GetByStage(workspaceID, stageName)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return &GetLarkWorkflowConfigV2Resp{
				StageName:   stageName,
				WorkspaceID: workspaceID,
				Nodes:       make([]*commonmodels.LarkPluginWorkflowConfigNodeV2, 0),
			}, nil
		}
		return nil, fmt.Errorf("failed to get lark plugin workflow config v2: %w", err)
	}

	nodes, err := mongodb.NewLarkPluginWfConfigNodeV2Coll().GetByStage(workspaceID, stageName)
	if err != nil {
		return nil, fmt.Errorf("failed to get lark plugin workflow config nodes v2: %w", err)
	}

	return &GetLarkWorkflowConfigV2Resp{
		StageName:    cfg.StageName,
		WorkspaceID:  cfg.WorkspaceID,
		UpdateTime:   cfg.UpdateTime,
		CodeSource:   cfg.CodeSource,
		TemplateID:   cfg.TemplateID,
		TemplateName: cfg.TemplateName,
		BranchFilter: cfg.BranchFilter,
		TargetBranch: cfg.TargetBranch,
		Nodes:        nodes,
	}, nil
}

type UpdateLarkWorkflowConfigV2Req struct {
	WorkspaceID  string                                         `json:"workspace_id"`
	CodeSource   string                                         `json:"code_source"`
	TemplateID   int64                                          `json:"template_id"`
	TemplateName string                                         `json:"template_name"`
	BranchFilter string                                         `json:"branch_filter"`
	TargetBranch string                                         `json:"target_branch"`
	Nodes        []*commonmodels.LarkPluginWorkflowConfigNodeV2 `json:"nodes"`
}

func validateWorkflowForLarkPlugin(workflowName string) error {
	workflow, err := mongodb.NewWorkflowV4Coll().Find(workflowName)
	if err != nil {
		return fmt.Errorf("failed to find workflow %s: %w", workflowName, err)
	}

	var buildCount, deployCount, testCount int
	buildIdx, deployIdx := -1, -1
	jobIdx := 0

	for _, stage := range workflow.Stages {
		for _, job := range stage.Jobs {
			switch job.JobType {
			case aslanconfig.JobZadigBuild:
				buildCount++
				buildIdx = jobIdx
			case aslanconfig.JobZadigDeploy:
				deployCount++
				deployIdx = jobIdx
			case aslanconfig.JobZadigTesting:
				testCount++
			default:
				return fmt.Errorf("workflow %s contains unsupported job type %s, only build, deploy, and test jobs are allowed", workflowName, job.JobType)
			}
			jobIdx++
		}
	}

	if buildCount != 1 {
		return fmt.Errorf("workflow %s must have exactly 1 build job, got %d", workflowName, buildCount)
	}
	if deployCount != 1 {
		return fmt.Errorf("workflow %s must have exactly 1 deploy job, got %d", workflowName, deployCount)
	}
	if testCount > 1 {
		return fmt.Errorf("workflow %s can have at most 1 test job, got %d", workflowName, testCount)
	}

	if deployIdx < buildIdx {
		return fmt.Errorf("workflow %s has invalid job order: deploy job must come after build job", workflowName)
	}

	return nil
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

	cfg := &commonmodels.LarkPluginWorkflowConfigV2{
		StageName:    stageName,
		WorkspaceID:  req.WorkspaceID,
		CodeSource:   req.CodeSource,
		TemplateID:   req.TemplateID,
		TemplateName: req.TemplateName,
		BranchFilter: req.BranchFilter,
		TargetBranch: req.TargetBranch,
	}

	if err := mongodb.NewLarkPluginWorkflowConfigV2Coll().Upsert(cfg); err != nil {
		return fmt.Errorf("failed to upsert lark plugin workflow config v2: %w", err)
	}

	if err := mongodb.NewLarkPluginWfConfigNodeV2Coll().ReplaceByStage(req.WorkspaceID, stageName, req.Nodes); err != nil {
		return fmt.Errorf("failed to replace lark plugin workflow config nodes v2: %w", err)
	}

	return nil
}

func getWorkitemConfigNodes(ctx *internalhandler.Context, workspaceID, workItemType, workItemID string) ([]*commonmodels.LarkPluginWorkflowConfigNodeV2, error) {
	workItemIDInt, err := strconv.ParseInt(workItemID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse workitem id: %w", err)
	}

	larkClient := larkplugin.NewClient(config.LarkPluginID(), config.LarkPluginSecret(), ctx.LarkPlugin.LarkType)
	larkResp, err := larkClient.ClientV2.WorkItem.GetWorkItemsByIds(ctx, workitem.NewGetWorkItemsByIdsReqBuilder().
		ProjectKey(ctx.LarkPlugin.ProjectKey).
		WorkItemTypeKey(workItemType).
		WorkItemIDs([]int64{workItemIDInt}).
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

	workItem := larkResp.Data[0]

	var currentNodeIDs []string
	if *workItem.Pattern == string(meego.WorkItemPatternNode) {
		for _, currentNode := range workItem.CurrentNodes {
			currentNodeIDs = append(currentNodeIDs, *currentNode.ID)
		}
	} else if *workItem.Pattern == string(meego.WorkItemPatternState) {
		currentNodeIDs = append(currentNodeIDs, *workItem.WorkItemStatus.StateKey)
	} else {
		return nil, fmt.Errorf("unsupported pattern %s", *workItem.Pattern)
	}

	if len(currentNodeIDs) == 0 {
		return nil, fmt.Errorf("no current node found")
	}

	configNodes, err := mongodb.NewLarkPluginWfConfigNodeV2Coll().GetByWorkItem(workspaceID, *workItem.TemplateID, currentNodeIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow config nodes: %w", err)
	}

	return configNodes, nil
}

type GetLarkWorkitemServicesV2Resp struct {
	Services []*commonmodels.ServiceWithModule `json:"services"`
}

func GetLarkWorkitemServicesV2(ctx *internalhandler.Context, workspaceID, workItemType, workItemID string) (*GetLarkWorkitemServicesV2Resp, error) {
	configNodes, err := getWorkitemConfigNodes(ctx, workspaceID, workItemType, workItemID)
	if err != nil {
		return nil, err
	}
	if len(configNodes) == 0 {
		return nil, fmt.Errorf("no workflow config nodes found")
	}

	workflow, err := mongodb.NewWorkflowV4Coll().Find(configNodes[0].WorkflowName)
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

type workitemRepoContext struct {
	Repo       *types.Repository
	ConfigNode *commonmodels.LarkPluginWorkflowConfigNodeV2
}

func getWorkitemFirstRepo(ctx *internalhandler.Context, workspaceID, workItemType, workItemID, serviceName, serviceModule string) (*workitemRepoContext, error) {
	configNodes, err := getWorkitemConfigNodes(ctx, workspaceID, workItemType, workItemID)
	if err != nil {
		return nil, err
	}
	if len(configNodes) == 0 {
		return nil, fmt.Errorf("no workflow config nodes found")
	}

	workflow, err := mongodb.NewWorkflowV4Coll().Find(configNodes[0].WorkflowName)
	if err != nil {
		return nil, fmt.Errorf("failed to find workflow: %w", err)
	}

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

	var matchedBuild *commonmodels.ServiceAndBuild
	for _, opt := range buildSpec.ServiceAndBuildsOptions {
		if opt != nil && opt.ServiceName == serviceName && opt.ServiceModule == serviceModule {
			matchedBuild = opt
			break
		}
	}
	if matchedBuild == nil {
		return nil, fmt.Errorf("service %s/%s not found in workflow build options", serviceName, serviceModule)
	}

	buildSvc := commonservice.NewBuildService()
	buildInfo, err := buildSvc.GetBuild(matchedBuild.BuildName, serviceName, serviceModule)
	if err != nil {
		return nil, fmt.Errorf("failed to get build %s for %s/%s: %w", matchedBuild.BuildName, serviceName, serviceModule, err)
	}

	if len(buildInfo.Repos) == 0 {
		return nil, fmt.Errorf("no repos configured in build %s", matchedBuild.BuildName)
	}

	return &workitemRepoContext{
		Repo:       buildInfo.Repos[0],
		ConfigNode: configNodes[0],
	}, nil
}

type GetLarkWorkitemPRsV2Resp struct {
	PRs []*client.PullRequest `json:"prs"`
}

func GetLarkWorkitemPRsV2(ctx *internalhandler.Context, workspaceID, workItemType, workItemID, serviceName, serviceModule string, page, perPage int) (*GetLarkWorkitemPRsV2Resp, error) {
	repoCtx, err := getWorkitemFirstRepo(ctx, workspaceID, workItemType, workItemID, serviceName, serviceModule)
	if err != nil {
		return nil, err
	}

	stageConfig, err := mongodb.NewLarkPluginWorkflowConfigV2Coll().GetByStage(workspaceID, repoCtx.ConfigNode.StageName)
	if err != nil {
		return nil, fmt.Errorf("failed to get stage config: %w", err)
	}

	repo := repoCtx.Repo
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
	repoCtx, err := getWorkitemFirstRepo(ctx, workspaceID, workItemType, workItemID, serviceName, serviceModule)
	if err != nil {
		return nil, err
	}

	stageConfig, err := mongodb.NewLarkPluginWorkflowConfigV2Coll().GetByStage(workspaceID, repoCtx.ConfigNode.StageName)
	if err != nil {
		return nil, fmt.Errorf("failed to get stage config: %w", err)
	}

	repo := repoCtx.Repo
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
	WorkflowName string                                       `json:"workflow_name"`
	Configs      []*commonmodels.LarkPluginStageServiceConfig `json:"configs"`
}

func GetLarkStageServiceConfigV2(ctx *internalhandler.Context, workspaceID, stageName, workItemTypeKey, workItemID string) (*GetLarkStageServiceConfigV2Resp, error) {
	configNodes, err := getWorkitemConfigNodes(ctx, workspaceID, workItemTypeKey, workItemID)
	if err != nil {
		return nil, err
	}

	var workflowName string
	if len(configNodes) > 0 {
		workflowName = configNodes[0].WorkflowName
	}

	configs, err := mongodb.NewLarkPluginStageServiceConfigColl().GetByWorkItem(workspaceID, stageName, workItemTypeKey, workItemID)
	if err != nil {
		return nil, fmt.Errorf("failed to get stage service configs: %w", err)
	}

	return &GetLarkStageServiceConfigV2Resp{
		WorkflowName: workflowName,
		Configs:      configs,
	}, nil
}

type UpdateLarkStageServiceConfigV2Req struct {
	WorkspaceID string                                       `json:"workspace_id"`
	Configs     []*commonmodels.LarkPluginStageServiceConfig `json:"configs"`
}

func UpdateLarkStageServiceConfigV2(ctx *internalhandler.Context, stageName, workItemTypeKey, workItemID string, req *UpdateLarkStageServiceConfigV2Req) error {
	if err := mongodb.NewLarkPluginStageServiceConfigColl().ReplaceByWorkItem(req.WorkspaceID, stageName, workItemTypeKey, workItemID, req.Configs); err != nil {
		return fmt.Errorf("failed to update stage service configs: %w", err)
	}
	return nil
}

func ExecuteLarkWorkitemWorkflowV2(ctx *internalhandler.Context, workspaceID, workItemTypeKey, workItemID string) error {
	configNodes, err := getWorkitemConfigNodes(ctx, workspaceID, workItemTypeKey, workItemID)
	if err != nil {
		return err
	}
	if len(configNodes) == 0 {
		return fmt.Errorf("no workflow config nodes found for this workitem")
	}

	configNode := configNodes[0]
	workflowName := configNode.WorkflowName
	stageName := configNode.StageName

	workflow, err := mongodb.NewWorkflowV4Coll().Find(workflowName)
	if err != nil {
		return fmt.Errorf("failed to find workflow %s: %w", workflowName, err)
	}

	serviceConfigs, err := mongodb.NewLarkPluginStageServiceConfigColl().GetByWorkItem(workspaceID, stageName, workItemTypeKey, workItemID)
	if err != nil {
		return fmt.Errorf("failed to get user service configs: %w", err)
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
	serviceConfigMap := make(map[string]*commonmodels.LarkPluginStageServiceConfig)
	for _, sc := range serviceConfigs {
		serviceConfigMap[sc.ServiceName+"/"+sc.ServiceModule] = sc
	}

	// Resolve workflow job parameters
	buildSvc := commonservice.NewBuildService()

	workflowCtrl := workflowController.CreateWorkflowController(workflow)
	if err != nil {
		return fmt.Errorf("failed to create workflow controller: %w", err)
	}

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
				// Use default env and settings — no modifications needed

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
