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

package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/koderover/zadig/v2/pkg/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/plugin/service"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	"github.com/koderover/zadig/v2/pkg/tool/cache"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/types"
)

// @summary 飞书插件登录
// @description 飞书插件登录
// @tags 	plugin
// @accept 	json
// @produce json
// @Param   body 			body 		 service.LarkLoginRequest 							         false 	"登录请求"
// @success 200             {object}     service.LarkLoginResponse
// @router /api/plugin/lark/login [post]
func LarkLogin(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	err := commonutil.CheckZadigEnterpriseLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	req := new(service.LarkLoginRequest)
	if err := c.BindJSON(req); err != nil {
		ctx.RespErr = err
		return
	}

	workspaceID := c.GetHeader("X-WORKSPACE-ID")
	if workspaceID == "" {
		ctx.RespErr = fmt.Errorf("missing X-WORKSPACE-ID")
		return
	}

	ctx.Resp, ctx.RespErr = service.LarkLogin(ctx, workspaceID, req)
}

// @summary 获取飞书插件授权配置
// @description 获取飞书插件授权配置
// @tags 	plugin
// @accept 	json
// @produce json
// @Param   workspace_id 	query 		 string 											         true 	"工作空间ID"
// @success 200             {object}     models.LarkPluginAuthConfig
// @router /api/plugin/lark/config/auth [get]
func GetLarkAuthConfig(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	err = commonutil.CheckZadigEnterpriseLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	workspaceID := c.Query("workspace_id")
	if workspaceID == "" {
		ctx.RespErr = fmt.Errorf("workspace_id is required")
		return
	}

	ctx.Resp, ctx.RespErr = service.GetLarkAuthConfig(ctx, workspaceID)
}

// @summary 更新飞书插件授权配置
// @description 更新飞书插件授权配置
// @tags 	plugin
// @accept 	json
// @produce json
// @Param   body 	body 		 models.LarkPluginAuthConfig 							         false 	"授权配置"
// @success 200
// @router /api/plugin/lark/config/auth [put]
func UpdateLarkAuthConfig(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	err := commonutil.CheckZadigEnterpriseLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	req := new(models.LarkPluginAuthConfig)
	if err := c.BindJSON(req); err != nil {
		ctx.RespErr = err
		return
	}

	ctx.RespErr = service.UpdateLarkAuthConfig(ctx, req)
}

// @summary 获取飞书插件工作流配置
// @description 获取飞书插件工作流配置
// @tags 	plugin
// @accept 	json
// @produce json
// @Param   workspace_id 	query 		 string 											         true 	"工作空间ID"
// @success 200             {object}     service.GetLarkWorkflowConfigResp
// @router /api/plugin/lark/config/workflow [get]
func GetLarkWorkflowConfig(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	err = commonutil.CheckZadigEnterpriseLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	err = CheckLarkAuth(c, ctx)
	if err != nil {
		ctx.RespErr = err
		ctx.UnAuthorized = true
		return
	}

	ctx.Resp, ctx.RespErr = service.GetLarkWorkflowConfig(ctx)
}

// @summary 更新飞书插件工作流配置
// @description 获取飞书插件工作流配置
// @tags 	plugin
// @accept 	json
// @produce json
// @Param   configs 	body 		 service.UpdateLarkWorkflowConfigRequest 							         false 	"工作流配置"
// @success 200
// @router /api/plugin/lark/config/workflow [put]
func UpdateLarkWorkflowConfig(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	err = commonutil.CheckZadigEnterpriseLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	err = CheckLarkAuth(c, ctx)
	if err != nil {
		ctx.RespErr = err
		ctx.UnAuthorized = true
		return
	}

	req := new(service.UpdateLarkWorkflowConfigRequest)
	if err := c.BindJSON(req); err != nil {
		ctx.RespErr = err
		return
	}

	ctx.RespErr = service.UpdateLarkWorkflowConfig(ctx, req)
}

// @summary 获取飞书插件工作项类型
// @description 获取飞书插件工作项类型
// @tags 	plugin
// @accept 	json
// @produce json
// @success 200             {object}     service.GetLarkWorkitemTypeResponse
// @router /api/plugin/lark/workitem/type [get]
func GetLarkWorkitemType(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	err = commonutil.CheckZadigEnterpriseLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	err = CheckLarkAuth(c, ctx)
	if err != nil {
		ctx.RespErr = err
		ctx.UnAuthorized = true
		return
	}

	ctx.Resp, ctx.RespErr = service.GetLarkWorkitemType(ctx)
}

// @summary 获取飞书插件工作项模版
// @description 获取飞书插件工作项模版
// @tags 	plugin
// @accept 	json
// @produce json
// @Param 	workitemTypeKey path		 string							true	"workitem type key"
// @success 200             {object}     service.GetLarkWorkitemTypeTemplateResponse
// @router /api/plugin/lark/workitem/type/:workitemTypeKey/template [get]
func GetLarkWorkitemTypeTemplate(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	err = commonutil.CheckZadigEnterpriseLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	err = CheckLarkAuth(c, ctx)
	if err != nil {
		ctx.RespErr = err
		ctx.UnAuthorized = true
		return
	}

	workitemTypeKey := c.Param("workitemTypeKey")
	if workitemTypeKey == "" {
		ctx.RespErr = fmt.Errorf("workitemTypeKey is required")
		return
	}

	ctx.Resp, ctx.RespErr = service.GetLarkWorkitemTypeTemplate(ctx, workitemTypeKey)
}

// @summary 获取飞书插件工作项节点
// @description 获取飞书插件工作项节点
// @tags 	plugin
// @accept 	json
// @produce json
// @Param 	workitemTypeKey path		string							true	"workitem type key"
// @success 200             {object}     service.GetLarkWorkitemTypeNodesResponse
// @router /api/plugin/lark/workitem/type/:workitemTypeKey/template/:templateID/node [get]
func GetLarkWorkitemTypeNodes(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	err = commonutil.CheckZadigEnterpriseLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	err = CheckLarkAuth(c, ctx)
	if err != nil {
		ctx.RespErr = err
		ctx.UnAuthorized = true
		return
	}

	workitemTypeKey := c.Param("workitemTypeKey")
	if workitemTypeKey == "" {
		ctx.RespErr = fmt.Errorf("workitemTypeKey is required")
		return
	}

	templateIDStr := c.Param("templateID")
	if templateIDStr == "" {
		ctx.RespErr = fmt.Errorf("templateID is required")
		return
	}

	templateID, err := strconv.ParseInt(templateIDStr, 10, 64)
	if err != nil {
		ctx.RespErr = fmt.Errorf("templateID is invalid, error: %w", err)
		return
	}

	ctx.Resp, ctx.RespErr = service.GetLarkWorkitemTypeNodes(ctx, workitemTypeKey, templateID)
}

// @summary 获取飞书插件工作项的工作流
// @description 获取飞书插件工作项的工作流
// @tags 	plugin
// @accept 	json
// @produce json
// @Param 	workitemTypeKey path		string							true	"workitem type key"
// @Param 	workItemID 		path		string							true	"workitem id"
// @success 200             {object}     service.GetLarkWorkitemWorkflowResponse
// @router /api/plugin/lark/workitem/:workitemTypeKey/:workItemID/workflow [get]
func GetLarkWorkitemWorkflow(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	err = commonutil.CheckZadigEnterpriseLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	err = CheckLarkAuth(c, ctx)
	if err != nil {
		ctx.RespErr = err
		ctx.UnAuthorized = true
		return
	}

	workflowTypeKey := c.Param("workitemTypeKey")
	if workflowTypeKey == "" {
		ctx.RespErr = fmt.Errorf("workflowTypeKey is required")
		return
	}

	workItemID := c.Param("workItemID")
	if workItemID == "" {
		ctx.RespErr = fmt.Errorf("workItemID is required")
		return
	}

	collModeWorkflowsWithVerb, err := internalhandler.ListAuthorizedWorkflowWithVerb(ctx.UserID, "")
	if err != nil {
		ctx.Logger.Errorf("failed to list collaboration mode authorized workflow resource, error: %s", err)
		ctx.RespErr = err
		return
	}

	authProjects := sets.Set[string]{}
	authWorkflows := sets.Set[string]{}

	for projectName, project := range ctx.Resources.ProjectAuthInfo {
		if project.IsProjectAdmin || project.Workflow.Execute {
			authProjects.Insert(projectName)
		}
	}

	for _, workflowMap := range collModeWorkflowsWithVerb.ProjectWorkflowActionsMap {
		for workflowName, workflowAction := range workflowMap {
			if workflowAction.Execute {
				authWorkflows.Insert(workflowName)
			}
		}
	}

	ctx.Resp, ctx.RespErr = service.GetLarkWorkitemWorkflow(ctx, workflowTypeKey, workItemID, ctx.Resources.IsSystemAdmin, authProjects, authWorkflows)
}

// @summary 执行飞书插件工作项的工作流
// @description 执行飞书插件工作项的工作流
// @tags 	plugin
// @accept 	json
// @produce json
// @Param 	workitemTypeKey path		string							true	"workitem type key"
// @Param 	workItemID 		path		string							true	"workitem id"
// @success 200             {object}     service.GetLarkWorkitemWorkflowResponse
// @router /api/plugin/lark/workitem/:workitemTypeKey/:workItemID/workflow [post]
func ExecuteLarkWorkitemWorkflow(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	err = commonutil.CheckZadigEnterpriseLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	err = CheckLarkAuth(c, ctx)
	if err != nil {
		ctx.RespErr = err
		ctx.UnAuthorized = true
		return
	}

	workItemTypeKey := c.Param("workitemTypeKey")
	if workItemTypeKey == "" {
		ctx.RespErr = fmt.Errorf("workitemTypeKey is required")
		return
	}

	workItemID := c.Param("workItemID")
	if workItemID == "" {
		ctx.RespErr = fmt.Errorf("workItemID is required")
		return
	}

	args := new(models.WorkflowV4)
	err = c.BindJSON(args)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(fmt.Errorf("get bind json error: %v", err))
		return
	}

	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[args.Project]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if !ctx.Resources.ProjectAuthInfo[args.Project].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[args.Project].Workflow.Execute {
			// check if the permission is given by collaboration mode
			permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, args.Project, types.ResourceTypeWorkflow, args.Name, types.WorkflowActionRun)
			if err != nil || !permitted {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	ctx.RespErr = service.ExecuteLarkWorkitemWorkflow(ctx, workItemTypeKey, workItemID, args)
}

// @summary 列出飞书插件工作项的工作流任务
// @description 列出飞书插件工作项的工作流任务
// @tags 	plugin
// @accept 	json
// @produce json
// @Param 	workflowName 	path		string							true	"workflow name"
// @Param 	workitemTypeKey path		string							true	"workitem type key"
// @Param 	workItemID 		path		string							true	"workitem id"
// @Param 	pageNum 		query		int								true	"page num"
// @param   pageSize 		query		int								true	"page size"
// @success 200             {object}     service.ListLarkWorkitemWorkflowTaskResponse
// @router /api/plugin/lark/workitem/:workitemTypeKey/:workItemID/workflow/:workflowName/task [get]
func ListLarkWorkitemWorkflowTask(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	err = commonutil.CheckZadigEnterpriseLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	err = CheckLarkAuth(c, ctx)
	if err != nil {
		ctx.RespErr = err
		ctx.UnAuthorized = true
		return
	}

	workflowTypeKey := c.Param("workitemTypeKey")
	if workflowTypeKey == "" {
		ctx.RespErr = fmt.Errorf("workflowTypeKey is required")
		return
	}

	workItemID := c.Param("workItemID")
	if workItemID == "" {
		ctx.RespErr = fmt.Errorf("workItemID is required")
		return
	}

	workflowName := c.Param("workflowName")
	if workflowName == "" {
		ctx.RespErr = fmt.Errorf("workflowName is required")
		return
	}

	pageNumStr := c.Query("pageNum")
	if pageNumStr == "" {
		ctx.RespErr = fmt.Errorf("pageNum is required")
		return
	}
	pageNum, err := strconv.Atoi(pageNumStr)
	if err != nil {
		ctx.RespErr = fmt.Errorf("pageNum is invalid, error: %w", err)
		return
	}

	pageSizeStr := c.Query("pageSize")
	if pageSizeStr == "" {
		ctx.RespErr = fmt.Errorf("pageSize is required")
		return
	}
	pageSize, err := strconv.Atoi(pageSizeStr)
	if err != nil {
		ctx.RespErr = fmt.Errorf("pageSize is invalid, error: %w", err)
		return
	}

	ctx.Resp, ctx.RespErr = service.ListLarkWorkitemWorkflowTask(ctx, workflowTypeKey, workItemID, workflowName, pageNum, pageSize)
}

func CheckLarkAuth(c *gin.Context, ctx *internalhandler.Context) (err error) {
	defer func() {
		if err != nil {
			ctx.Logger.Errorf("CheckLarkAuth failed: %s", err)
		}
	}()

	token := c.GetHeader("X-PLUGIN-TOKEN")
	if token == "" {
		err = fmt.Errorf("Unauthorized, missing X-PLUGIN-TOKEN")
		return
	}

	userKey := c.GetHeader("X-USER-KEY")
	if userKey == "" {
		err = fmt.Errorf("Unauthorized, missing X-USER-KEY")
		return
	}

	workspaceID := c.GetHeader("X-WORKSPACE-ID")
	if workspaceID == "" {
		err = fmt.Errorf("Unauthorized, missing X-USER-KEY")
		return
	}

	userTokenStr, err := cache.NewRedisCache(config.RedisCommonCacheTokenDB()).GetString(fmt.Sprintf("lark-plugin-user-token-%s-%s", workspaceID, userKey))
	if err != nil {
		if errors.Is(err, redis.Nil) {
			err = fmt.Errorf("Unauthorized, user token not found")
			return
		}
		err = fmt.Errorf("Unauthorized, failed to get user token from cache: %w", err)
		return
	}

	if userTokenStr == "" {
		err = fmt.Errorf("Unauthorized, user token not found")
		return
	}

	userToken := &service.LarkLoginResponse{}
	err = json.Unmarshal([]byte(userTokenStr), userToken)
	if err != nil {
		err = fmt.Errorf("Unauthorized, missing X-PLUGIN-TOKEN")
		return
	}

	if userToken.UserAccessToken != token {
		err = fmt.Errorf("Unauthorized, token mismatch")
		return
	}

	if userToken.UserKey != userKey {
		err = fmt.Errorf("Unauthorized, user key mismatch")
		return
	}

	ctx.LarkPlugin = &internalhandler.LarkPluginContext{
		PluginAccessToken: userToken.PluginAccessToken,
		UserKey:           userKey,
		ProjectKey:        workspaceID,
		LarkType:          userToken.LarkType,
	}

	return nil
}
