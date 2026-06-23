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

	"github.com/gin-gonic/gin"
	"github.com/koderover/zadig/v2/pkg/types"

	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	svcservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/service/service"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
)

// ListManualServiceModules lists the manual module declarations for a service.
// Query: ?projectName=...&production=true|false (default false)
// Path:  /services/:name/manual-modules — :name is the service name.
func ListManualServiceModules(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectName := c.Query("projectName")
	production := c.Query("production") == "true"
	serviceName := c.Param("name")

	if !canViewService(ctx, projectName, production) {
		ctx.UnAuthorized = true
		return
	}
	if production {
		if err := commonutil.CheckZadigProfessionalLicense(); err != nil {
			ctx.RespErr = err
			return
		}
	}

	ctx.Resp, ctx.RespErr = svcservice.ListManualServiceModules(projectName, serviceName, production, ctx.Logger)
}

// CreateManualServiceModule declares a new manual module for a service.
func CreateManualServiceModule(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectName := c.Query("projectName")
	production := c.Query("production") == "true"

	if !canEditService(ctx, projectName, production) {
		ctx.UnAuthorized = true
		return
	}
	if production {
		if err := commonutil.CheckZadigProfessionalLicense(); err != nil {
			ctx.RespErr = err
			return
		}
	}

	args := new(svcservice.ManualModuleCreateReq)
	if err := c.ShouldBindJSON(args); err != nil {
		ctx.RespErr = err
		return
	}

	function := "项目管理-服务"
	if production {
		function = "项目管理-生产服务"
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, projectName, "新增", function+"/手动模块", args.ServiceName+"/"+args.Name, args.Name, "", types.RequestBodyTypeJSON, ctx.Logger)

	ctx.Resp, ctx.RespErr = svcservice.CreateManualServiceModule(args, projectName, production, ctx.Logger)
}

// UpdateManualServiceModule edits an existing manual module by id.
// Path: /services/manual-modules/:id
func UpdateManualServiceModule(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectName := c.Query("projectName")
	production := c.Query("production") == "true"

	if !canEditService(ctx, projectName, production) {
		ctx.UnAuthorized = true
		return
	}
	if production {
		if err := commonutil.CheckZadigProfessionalLicense(); err != nil {
			ctx.RespErr = err
			return
		}
	}

	args := new(svcservice.ManualModuleUpdateReq)
	if err := c.ShouldBindJSON(args); err != nil {
		ctx.RespErr = err
		return
	}

	id := c.Param("id")
	function := "项目管理-服务"
	if production {
		function = "项目管理-生产服务"
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, projectName, "修改", function+"/手动模块", id, "", "", types.RequestBodyTypeJSON, ctx.Logger)

	ctx.RespErr = svcservice.UpdateManualServiceModule(id, production, args, ctx.Logger)
}

// DeleteManualServiceModule removes a manual module by id.
// Path: /services/manual-modules/:id
func DeleteManualServiceModule(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectName := c.Query("projectName")
	production := c.Query("production") == "true"

	if !canEditService(ctx, projectName, production) {
		ctx.UnAuthorized = true
		return
	}
	if production {
		if err := commonutil.CheckZadigProfessionalLicense(); err != nil {
			ctx.RespErr = err
			return
		}
	}

	id := c.Param("id")
	function := "项目管理-服务"
	if production {
		function = "项目管理-生产服务"
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, projectName, "删除", function+"/手动模块", id, "", "", types.RequestBodyTypeJSON, ctx.Logger)

	ctx.RespErr = svcservice.DeleteManualServiceModule(id, production, ctx.Logger)
}

// DeleteAutoServiceModule removes one auto-discovered module by id.
// Query: ?projectName=...&production=true|false
func DeleteAutoServiceModule(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectName := c.Query("projectName")
	production := c.Query("production") == "true"

	if !canEditService(ctx, projectName, production) {
		ctx.UnAuthorized = true
		return
	}
	if production {
		if err := commonutil.CheckZadigProfessionalLicense(); err != nil {
			ctx.RespErr = err
			return
		}
	}

	id := c.Param("id")
	function := "项目管理-服务"
	if production {
		function = "项目管理-生产服务"
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, projectName, "删除", function+"/自动识别模块", id, "", "", types.RequestBodyTypeJSON, ctx.Logger)

	ctx.RespErr = svcservice.DeleteAutoServiceModule(id, production, ctx.Logger)
}

// canViewService mirrors the authorization scaffolding used by the existing
// service-template handlers — system admin / project admin / service view
// permission, branching on production.
func canViewService(ctx *internalhandler.Context, projectName string, production bool) bool {
	if ctx.Resources.IsSystemAdmin {
		return true
	}
	info, ok := ctx.Resources.ProjectAuthInfo[projectName]
	if !ok {
		return false
	}
	if production {
		return info.IsProjectAdmin || info.ProductionService.View
	}
	return info.IsProjectAdmin || info.Service.View
}

func canEditService(ctx *internalhandler.Context, projectName string, production bool) bool {
	if ctx.Resources.IsSystemAdmin {
		return true
	}
	info, ok := ctx.Resources.ProjectAuthInfo[projectName]
	if !ok {
		return false
	}
	if production {
		return info.IsProjectAdmin || info.ProductionService.Edit
	}
	return info.IsProjectAdmin || info.Service.Edit
}
