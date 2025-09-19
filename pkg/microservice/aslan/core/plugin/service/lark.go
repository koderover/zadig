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
	"fmt"
	"strconv"
	"time"

	sdkcore "github.com/larksuite/project-oapi-sdk-golang/core"
	"github.com/larksuite/project-oapi-sdk-golang/v2/service/workitem"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/koderover/zadig/v2/pkg/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	workflowservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/workflow/service/workflow"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	"github.com/koderover/zadig/v2/pkg/tool/cache"
	"github.com/koderover/zadig/v2/pkg/tool/larkplugin"
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
	resp, err := lark.Client.Plugin.GetPluginToken(ctx, 1)
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

	err = cache.NewRedisCache(config.RedisCommonCacheTokenDB()).Write(fmt.Sprintf("lark_plugin_user_token_%s_%s", workspaceID, resp2.Data.UserKey), string(tokenData), time.Duration(resp3.PluginAccessTokenExpireTime-5)*time.Second)
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
	for _, config := range req.Configs {
		config.WorkspaceID = req.WorkspaceID
	}

	err := mongodb.NewLarkPluginWorkflowConfigColl().Update(req.Configs)
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

type GetLarkWorkitemTypeNodesResponse struct {
	Nodes []*LarkWorkitemTypeNode `json:"nodes"`
}

type LarkWorkitemTypeNode struct {
	// 节点 ID
	StateKey string `json:"state_key"`
	Name     string `json:"name"`
}

func GetLarkWorkitemTypeNodes(ctx *internalhandler.Context, workitemTypeKey string) (*GetLarkWorkitemTypeNodesResponse, error) {
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
		return nil, fmt.Errorf("failed to get lark workitem type node: %w", err)
	}

	if larkResp.Code() != 0 {
		return nil, fmt.Errorf("failed to get lark workitem type node, code: %d, message: %s", larkResp.Code(), larkResp.ErrMsg)
	}

	templateMap := make(map[string]*workitem.TemplateConf)
	for _, template := range larkResp.Data {
		if _, ok := templateMap[*template.TemplateID]; !ok {
			templateMap[*template.TemplateID] = &template
		} else {
			if *template.Version > *templateMap[*template.TemplateID].Version {
				templateMap[*template.TemplateID] = &template
			}
		}
	}

	nodeList := make([]*LarkWorkitemTypeNode, 0)
	for _, template := range templateMap {
		templateID, err := strconv.ParseInt(*template.TemplateID, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse template id: %w", err)
		}

		larkResp2, err := client.ClientV2.WorkItem.QueryTemplateDetail(ctx, workitem.NewQueryTemplateDetailReqBuilder().
			TemplateID(templateID).
			Build(),
			sdkcore.WithAccessToken(ctx.LarkPlugin.PluginAccessToken),
			sdkcore.WithUserKey(ctx.LarkPlugin.UserKey),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to query template detail: %w", err)
		}

		if larkResp2.Code() != 0 {
			return nil, fmt.Errorf("failed to query template detail, code: %d, message: %s", larkResp2.Code(), larkResp2.ErrMsg)
		}

		for _, workflowConf := range larkResp2.Data.WorkflowConfs {
			node := &LarkWorkitemTypeNode{
				StateKey: *workflowConf.StateKey,
				Name:     *workflowConf.Name,
			}
			nodeList = append(nodeList, node)
		}
	}

	resp := &GetLarkWorkitemTypeNodesResponse{
		Nodes: nodeList,
	}

	return resp, nil
}

type GetLarkWorkitemWorkflowResponse struct {
	Workflows []*WorkflowWithAction `json:"workflows"`
}

type Node struct {
	ID        string
	Name      string
	IsCurrent bool
}

type WorkflowWithAction struct {
	Node       *Node
	Workflow   *models.WorkflowV4
	CanExecute bool
}

func GetLarkWorkitemWorkflow(ctx *internalhandler.Context, workItemType, workItemID string, authProjects, authWorkflows sets.Set[string]) (*GetLarkWorkitemWorkflowResponse, error) {
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
		Build(),
		sdkcore.WithAccessToken(ctx.LarkPlugin.PluginAccessToken),
		sdkcore.WithUserKey(ctx.LarkPlugin.UserKey),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query template detail: %w", err)
	}
	if larkResp2.Error() != "" {
		return nil, fmt.Errorf("failed to query template detail, error: %s", larkResp2.Error())
	}

	templateWorkflowConfig := larkResp2.Data.WorkflowConfs

	workflowConfig, err := mongodb.NewLarkPluginWorkflowConfigColl().GetWorkItemTypeConfig(ctx.LarkPlugin.ProjectKey, workItemType)
	if err != nil {
		return nil, fmt.Errorf("failed to get lark plugin workflow config: %w", err)
	}

	workflowNodeMap := make(map[string]*Node)
	nodeWorkflow := make([]mongodb.WorkflowV4, 0)
	for _, node := range templateWorkflowConfig {
		for _, workflowConfigNode := range workflowConfig.Nodes {
			// 仅列出配置了工作流的节点
			if *node.StateKey == workflowConfigNode.ID {
				isCurrent := false
				for _, currentNode := range workItem.CurrentNodes {
					if *node.StateKey == *currentNode.ID {
						isCurrent = true
					}
				}

				workflowNodeMap[workflowConfigNode.Name] = &Node{
					ID:        workflowConfigNode.ID,
					Name:      workflowConfigNode.Name,
					IsCurrent: isCurrent,
				}

				nodeWorkflow = append(nodeWorkflow, mongodb.WorkflowV4{
					Name:        workflowConfigNode.WorkflowName,
					ProjectName: workflowConfigNode.ProjectKey,
				})
			}
		}
	}

	workflows, err := mongodb.NewWorkflowV4Coll().ListByWorkflows(mongodb.ListWorkflowV4Opt{
		Workflows: nodeWorkflow,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list workflows: %w", err)
	}

	workflowsWithAction := make([]*WorkflowWithAction, 0)
	for _, workflow := range workflows {
		if authProjects.Has(workflow.Project) || authWorkflows.Has(workflow.Name) {
			workflowAction := &WorkflowWithAction{
				Workflow: workflow,
			}

			if authProjects.Has(workflow.Project) {
				workflowAction.CanExecute = true
			}
			if authWorkflows.Has(workflow.Name) {
				workflowAction.CanExecute = true
			}

			node, ok := workflowNodeMap[workflow.Name]
			if ok {
				workflowAction.Node = node
			}

			workflowsWithAction = append(workflowsWithAction, workflowAction)
		}
	}

	return &GetLarkWorkitemWorkflowResponse{
		Workflows: workflowsWithAction,
	}, nil
}

type ListLarkWorkitemWorkflowTaskResponse struct {
	Tasks []*models.WorkflowTask `json:"tasks"`
	Count int64                  `json:"count"`
}

func ListLarkWorkitemWorkflowTask(ctx *internalhandler.Context, workItemTypeKey, workItemID, workflowName string, pageNum, pageSize int64) (*ListLarkWorkitemWorkflowTaskResponse, error) {
	workitemTasks, count, err := mongodb.NewLarkPluginWorkitemTaskColl().List(ctx, &mongodb.LarkPluginWorkitemTaskListOption{
		WorkflowName:    workflowName,
		WorkItemID:      workItemID,
		WorkItemTypeKey: workItemTypeKey,
		PageNum:         pageNum,
		PageSize:        pageSize,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create lark plugin workitem task: %w", err)
	}

	query := mongodb.ListWorkflowTaskV4ByNameAndIDsOption{
		WorkflowNameAndIDs: make([]mongodb.WorkflowNameAndID, 0),
	}
	for _, workitemTask := range workitemTasks {
		query.WorkflowNameAndIDs = append(query.WorkflowNameAndIDs, mongodb.WorkflowNameAndID{
			WorkflowName: workitemTask.WorkflowName,
			TaskID:       workitemTask.WorkflowTaskID,
		})
	}
	workflowTasks, err := mongodb.NewworkflowTaskv4Coll().ListByNameAndIDs(&query)
	if err != nil {
		return nil, fmt.Errorf("failed to list workflow task: %w", err)
	}

	return &ListLarkWorkitemWorkflowTaskResponse{
		Tasks: workflowTasks,
		Count: count,
	}, nil
}

func ExecuteLarkWorkitemWorkflow(ctx *internalhandler.Context, workItemTypeKey, workItemID string, args *models.WorkflowV4) (*GetLarkWorkitemWorkflowResponse, error) {
	task, err := workflowservice.CreateWorkflowTaskV4(&workflowservice.CreateWorkflowTaskV4Args{
		Name:    ctx.UserName,
		Account: ctx.Account,
		UserID:  ctx.UserID,
	}, args, ctx.Logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create workflow task: %w", err)
	}

	wokritemTask := &models.LarkPluginWorkitemTask{
		LarkWorkItemTypeKey: workItemTypeKey,
		LarkWorkItemID:      workItemID,
		WorkflowName:        args.Name,
		WorkflowTaskID:      task.TaskID,
		Hash:                args.Hash,
		Creator:             ctx.GenUserBriefInfo(),
	}
	err = mongodb.NewLarkPluginWorkitemTaskColl().Create(ctx, wokritemTask)
	if err != nil {
		return nil, fmt.Errorf("failed to create lark plugin workitem task: %w", err)
	}

	return &GetLarkWorkitemWorkflowResponse{}, nil
}

func GetLarkWorkitemWorkflowTask(ctx *internalhandler.Context, workItemType, workItemID string) (*GetLarkWorkitemWorkflowResponse, error) {

	return &GetLarkWorkitemWorkflowResponse{
		// Workflows: workflowsWithAction,
	}, nil
}
