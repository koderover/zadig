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

package handler

import (
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/plugin/service"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
)

func GetLarkWorkflowConfigV2(c *gin.Context) {
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

	workspaceID := c.Query("workspace_id")
	if workspaceID == "" {
		ctx.RespErr = fmt.Errorf("workspace_id is required")
		return
	}

	stageName := c.Param("stage")
	if stageName == "" {
		ctx.RespErr = fmt.Errorf("stage is required")
		return
	}

	ctx.Resp, ctx.RespErr = service.GetLarkWorkflowConfigV2(ctx, workspaceID, stageName)
}

func UpdateLarkWorkflowConfigV2(c *gin.Context) {
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

	workspaceID := c.Query("workspace_id")
	if workspaceID == "" {
		ctx.RespErr = fmt.Errorf("workspace_id is required")
		return
	}

	stageName := c.Param("stage")
	if stageName == "" {
		ctx.RespErr = fmt.Errorf("stage is required")
		return
	}

	req := new(service.UpdateLarkWorkflowConfigV2Req)
	if err := c.BindJSON(req); err != nil {
		ctx.RespErr = err
		return
	}

	if req.WorkspaceID != workspaceID {
		ctx.RespErr = fmt.Errorf("workspace_id in request body does not match query parameter")
		return
	}

	if err := validateUpdateLarkWorkflowConfigV2(req); err != nil {
		ctx.RespErr = err
		return
	}

	ctx.RespErr = service.UpdateLarkWorkflowConfigV2(ctx, stageName, req)
}

func validateUpdateLarkWorkflowConfigV2(req *service.UpdateLarkWorkflowConfigV2Req) error {
	seen := make(map[string]struct{})
	for _, node := range req.Nodes {
		key := fmt.Sprintf("%d-%s", node.TemplateID, node.NodeID)
		if _, exists := seen[key]; exists {
			return fmt.Errorf("duplicate node config: template_id=%d, node_id=%s", node.TemplateID, node.NodeID)
		}
		seen[key] = struct{}{}
	}
	return nil
}

func GetLarkWorkitemServicesV2(c *gin.Context) {
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

	workspaceID := c.Query("workspace_id")
	if workspaceID == "" {
		ctx.RespErr = fmt.Errorf("workspace_id is required")
		return
	}

	workitemTypeKey := c.Param("workitemTypeKey")
	if workitemTypeKey == "" {
		ctx.RespErr = fmt.Errorf("workitemTypeKey is required")
		return
	}

	workItemID := c.Param("workItemID")
	if workItemID == "" {
		ctx.RespErr = fmt.Errorf("workItemID is required")
		return
	}

	ctx.Resp, ctx.RespErr = service.GetLarkWorkitemServicesV2(ctx, workspaceID, workitemTypeKey, workItemID)
}

func GetLarkWorkitemPRsV2(c *gin.Context) {
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

	workspaceID := c.Query("workspace_id")
	if workspaceID == "" {
		ctx.RespErr = fmt.Errorf("workspace_id is required")
		return
	}

	workitemTypeKey := c.Param("workitemTypeKey")
	if workitemTypeKey == "" {
		ctx.RespErr = fmt.Errorf("workitemTypeKey is required")
		return
	}

	workItemID := c.Param("workItemID")
	if workItemID == "" {
		ctx.RespErr = fmt.Errorf("workItemID is required")
		return
	}

	serviceName := c.Query("service_name")
	if serviceName == "" {
		ctx.RespErr = fmt.Errorf("service_name is required")
		return
	}

	serviceModule := c.Query("service_module")
	if serviceModule == "" {
		ctx.RespErr = fmt.Errorf("service_module is required")
		return
	}

	page := 1
	if pageStr := c.Query("page"); pageStr != "" {
		page, err = strconv.Atoi(pageStr)
		if err != nil {
			ctx.RespErr = fmt.Errorf("invalid page: %w", err)
			return
		}
	}

	perPage := 20
	if perPageStr := c.Query("per_page"); perPageStr != "" {
		perPage, err = strconv.Atoi(perPageStr)
		if err != nil {
			ctx.RespErr = fmt.Errorf("invalid per_page: %w", err)
			return
		}
	}

	ctx.Resp, ctx.RespErr = service.GetLarkWorkitemPRsV2(ctx, workspaceID, workitemTypeKey, workItemID, serviceName, serviceModule, page, perPage)
}

func GetLarkWorkitemBranchesV2(c *gin.Context) {
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

	workspaceID := c.Query("workspace_id")
	if workspaceID == "" {
		ctx.RespErr = fmt.Errorf("workspace_id is required")
		return
	}

	workitemTypeKey := c.Param("workitemTypeKey")
	if workitemTypeKey == "" {
		ctx.RespErr = fmt.Errorf("workitemTypeKey is required")
		return
	}

	workItemID := c.Param("workItemID")
	if workItemID == "" {
		ctx.RespErr = fmt.Errorf("workItemID is required")
		return
	}

	serviceName := c.Query("service_name")
	if serviceName == "" {
		ctx.RespErr = fmt.Errorf("service_name is required")
		return
	}

	serviceModule := c.Query("service_module")
	if serviceModule == "" {
		ctx.RespErr = fmt.Errorf("service_module is required")
		return
	}

	page := 1
	if pageStr := c.Query("page"); pageStr != "" {
		page, err = strconv.Atoi(pageStr)
		if err != nil {
			ctx.RespErr = fmt.Errorf("invalid page: %w", err)
			return
		}
	}

	perPage := 20
	if perPageStr := c.Query("per_page"); perPageStr != "" {
		perPage, err = strconv.Atoi(perPageStr)
		if err != nil {
			ctx.RespErr = fmt.Errorf("invalid per_page: %w", err)
			return
		}
	}

	ctx.Resp, ctx.RespErr = service.GetLarkWorkitemBranchesV2(ctx, workspaceID, workitemTypeKey, workItemID, serviceName, serviceModule, page, perPage)
}

func GetLarkStageServiceConfigV2(c *gin.Context) {
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

	workspaceID := c.Query("workspace_id")
	if workspaceID == "" {
		ctx.RespErr = fmt.Errorf("workspace_id is required")
		return
	}

	stageName := c.Param("stage")
	if stageName == "" {
		ctx.RespErr = fmt.Errorf("stage is required")
		return
	}

	workitemTypeKey := c.Param("workitemTypeKey")
	if workitemTypeKey == "" {
		ctx.RespErr = fmt.Errorf("workitemTypeKey is required")
		return
	}

	workItemID := c.Param("workItemID")
	if workItemID == "" {
		ctx.RespErr = fmt.Errorf("workItemID is required")
		return
	}

	ctx.Resp, ctx.RespErr = service.GetLarkStageServiceConfigV2(ctx, workspaceID, stageName, workitemTypeKey, workItemID)
}

func UpdateLarkWorkItemStageWorkflowInputV2(c *gin.Context) {
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

	workspaceID := c.Query("workspace_id")
	if workspaceID == "" {
		ctx.RespErr = fmt.Errorf("workspace_id is required")
		return
	}

	stageName := c.Param("stage")
	if stageName == "" {
		ctx.RespErr = fmt.Errorf("stage is required")
		return
	}

	workitemTypeKey := c.Param("workitemTypeKey")
	if workitemTypeKey == "" {
		ctx.RespErr = fmt.Errorf("workitemTypeKey is required")
		return
	}

	workItemID := c.Param("workItemID")
	if workItemID == "" {
		ctx.RespErr = fmt.Errorf("workItemID is required")
		return
	}

	req := new(service.UpdateLarkWorkItemStageWorkflowInputV2Req)
	if err := c.BindJSON(req); err != nil {
		ctx.RespErr = err
		return
	}

	ctx.RespErr = service.UpdateLarkWorkItemStageWorkflowInputV2(ctx, stageName, workspaceID, workitemTypeKey, workItemID, req)
}

func ExecuteLarkWorkitemWorkflowV2(c *gin.Context) {
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

	workspaceID := c.Query("workspace_id")
	if workspaceID == "" {
		ctx.RespErr = fmt.Errorf("workspace_id is required")
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

	ctx.RespErr = service.ExecuteLarkWorkitemWorkflowV2(ctx, workspaceID, workItemTypeKey, workItemID)
}

func GetLarkReleaseWorkItemsV2(c *gin.Context) {
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

	workspaceID := c.Query("workspace_id")
	if workspaceID == "" {
		ctx.RespErr = fmt.Errorf("workspace_id is required")
		return
	}

	stageName := c.Param("stage")
	if stageName == "" {
		ctx.RespErr = fmt.Errorf("stage is required")
		return
	}

	ctx.Resp, ctx.RespErr = service.GetLarkReleaseWorkItemsV2(ctx, workspaceID, stageName)
}

func BindLarkWorkitemToReleaseV2(c *gin.Context) {
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

	workspaceID := c.Query("workspace_id")
	if workspaceID == "" {
		ctx.RespErr = fmt.Errorf("workspace_id is required")
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

	req := new(service.BindLarkWorkitemToReleaseV2Req)
	if err := c.BindJSON(req); err != nil {
		ctx.RespErr = err
		return
	}

	ctx.RespErr = service.BindLarkWorkitemToReleaseV2(ctx, workspaceID, workItemTypeKey, workItemID, req)
}

func GetLarkWorkitemBindV2(c *gin.Context) {
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

	workspaceID := c.Query("workspace_id")
	if workspaceID == "" {
		ctx.RespErr = fmt.Errorf("workspace_id is required")
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

	ctx.Resp, ctx.RespErr = service.GetLarkWorkitemBindV2(ctx, workspaceID, workItemTypeKey, workItemID)
}

func DeleteLarkWorkitemBindV2(c *gin.Context) {
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

	workspaceID := c.Query("workspace_id")
	if workspaceID == "" {
		ctx.RespErr = fmt.Errorf("workspace_id is required")
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

	ctx.RespErr = service.DeleteLarkWorkitemBindV2(ctx, workspaceID, workItemTypeKey, workItemID)
}

func ListLarkReleaseBindItemsV2(c *gin.Context) {
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

	workspaceID := c.Query("workspace_id")
	if workspaceID == "" {
		ctx.RespErr = fmt.Errorf("workspace_id is required")
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

	ctx.Resp, ctx.RespErr = service.ListLarkReleaseBindItemsV2(ctx, workspaceID, workItemID)
}

func ListLarkWorkitemStagesV2(c *gin.Context) {
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

	workspaceID := c.Query("workspace_id")
	if workspaceID == "" {
		ctx.RespErr = fmt.Errorf("workspace_id is required")
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

	ctx.Resp, ctx.RespErr = service.ListLarkWorkitemStagesV2(ctx, workspaceID, workItemTypeKey, workItemID)
}