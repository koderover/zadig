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
	"encoding/json"
	"fmt"

	"github.com/gin-gonic/gin"

	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	svcservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/service/service"
	"github.com/koderover/zadig/v2/pkg/setting"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/types"
)

type helmServiceAction string

const (
	helmServiceActionView   helmServiceAction = "view"
	helmServiceActionEdit   helmServiceAction = "edit"
	helmServiceActionDelete helmServiceAction = "delete"
)

func GetHelmServiceOpenAPI(c *gin.Context) {
	handleGetHelmServiceOpenAPI(c, false)
}

func GetProductionHelmServiceOpenAPI(c *gin.Context) {
	handleGetHelmServiceOpenAPI(c, true)
}

func handleGetHelmServiceOpenAPI(c *gin.Context, production bool) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey, serviceName, valid := validateHelmServiceOpenAPIParams(c, ctx)
	if !valid || !authorizeHelmServiceOpenAPI(ctx, projectKey, production, helmServiceActionView) {
		return
	}
	if production {
		if err := commonutil.CheckZadigProfessionalLicense(); err != nil {
			ctx.RespErr = err
			return
		}
	}
	ctx.Resp, ctx.RespErr = svcservice.GetHelmServiceOpenAPI(projectKey, serviceName, production, ctx.Logger)
}

func UpdateHelmServiceOpenAPI(c *gin.Context) {
	handleUpdateHelmServiceOpenAPI(c, false)
}

func UpdateProductionHelmServiceOpenAPI(c *gin.Context) {
	handleUpdateHelmServiceOpenAPI(c, true)
}

func handleUpdateHelmServiceOpenAPI(c *gin.Context, production bool) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey, serviceName, valid := validateHelmServiceOpenAPIParams(c, ctx)
	if !valid {
		return
	}
	req := new(svcservice.OpenAPIUpdateHelmServiceReq)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid update Helm service request body")
		return
	}
	if err := req.Validate(); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}
	if !authorizeHelmServiceOpenAPI(ctx, projectKey, production, helmServiceActionEdit) {
		return
	}
	if production {
		if err := commonutil.CheckZadigProfessionalLicense(); err != nil {
			ctx.RespErr = err
			return
		}
	}

	maskedValues, err := commonutil.MaskSensitiveValuesYAML(req.ValuesYAML)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(fmt.Sprintf("invalid values_yaml: %s", err))
		return
	}
	logBody, _ := json.Marshal(&svcservice.OpenAPIUpdateHelmServiceReq{ExpectedRevision: req.ExpectedRevision, ValuesYAML: maskedValues})
	function := "项目管理-测试服务"
	if production {
		function = "项目管理-生产服务"
	}
	internalhandler.InsertOperationLog(c, ctx.UserName+"(OpenAPI)", projectKey, "更新", function, serviceName, serviceName, string(logBody), types.RequestBodyTypeJSON, ctx.Logger)

	ctx.Resp, ctx.RespErr = svcservice.UpdateHelmServiceOpenAPI(projectKey, serviceName, ctx.UserName, ctx.RequestID, production, req, ctx.Logger)
}

func DeleteHelmServiceOpenAPI(c *gin.Context) {
	handleDeleteHelmServiceOpenAPI(c, false)
}

func DeleteProductionHelmServiceOpenAPI(c *gin.Context) {
	handleDeleteHelmServiceOpenAPI(c, true)
}

func handleDeleteHelmServiceOpenAPI(c *gin.Context, production bool) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey, serviceName, valid := validateHelmServiceOpenAPIParams(c, ctx)
	if !valid || !authorizeHelmServiceOpenAPI(ctx, projectKey, production, helmServiceActionDelete) {
		return
	}
	if production {
		if err := commonutil.CheckZadigProfessionalLicense(); err != nil {
			ctx.RespErr = err
			return
		}
	}
	function := "项目管理-测试服务"
	if production {
		function = "项目管理-生产服务"
	}
	internalhandler.InsertOperationLog(c, ctx.UserName+"(OpenAPI)", projectKey, "删除", function, serviceName, serviceName, "", types.RequestBodyTypeJSON, ctx.Logger)

	ctx.RespErr = svcservice.DeleteServiceTemplate(serviceName, setting.HelmDeployType, projectKey, production, ctx.Logger)
	if ctx.RespErr == nil {
		ctx.Resp = gin.H{"message": "success"}
	}
}

func validateHelmServiceOpenAPIParams(c *gin.Context, ctx *internalhandler.Context) (string, string, bool) {
	projectKey := c.Query("projectKey")
	if projectKey == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("projectKey cannot be empty")
		return "", "", false
	}
	serviceName := c.Param("name")
	if serviceName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("serviceName cannot be empty")
		return "", "", false
	}
	return projectKey, serviceName, true
}

func authorizeHelmServiceOpenAPI(ctx *internalhandler.Context, projectKey string, production bool, action helmServiceAction) bool {
	if ctx.Resources.IsSystemAdmin {
		return true
	}
	projectAuth, ok := ctx.Resources.ProjectAuthInfo[projectKey]
	if !ok {
		ctx.UnAuthorized = true
		return false
	}
	if projectAuth.IsProjectAdmin {
		return true
	}

	allowed := false
	if production {
		switch action {
		case helmServiceActionView:
			allowed = projectAuth.ProductionService.View
		case helmServiceActionEdit:
			allowed = projectAuth.ProductionService.Edit
		case helmServiceActionDelete:
			allowed = projectAuth.ProductionService.Delete
		}
	} else {
		switch action {
		case helmServiceActionView:
			allowed = projectAuth.Service.View
		case helmServiceActionEdit:
			allowed = projectAuth.Service.Edit
		case helmServiceActionDelete:
			allowed = projectAuth.Service.Delete
		}
	}
	ctx.UnAuthorized = !allowed
	return allowed
}
