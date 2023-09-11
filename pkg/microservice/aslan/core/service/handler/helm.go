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

package handler

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"

	svcservice "github.com/koderover/zadig/pkg/microservice/aslan/core/service/service"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

func ListHelmServices(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Param("productName")

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectKey].Service.View {
			ctx.UnAuthorized = true
			return
		}
	}

	ctx.Resp, ctx.Err = svcservice.ListHelmServices(projectKey, false, ctx.Logger)
}

func ListHelmProductionServices(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Query("projectName")

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectKey].ProductionService.View {
			ctx.UnAuthorized = true
			return
		}
	}

	ctx.Resp, ctx.Err = svcservice.ListHelmServices(projectKey, true, ctx.Logger)
}

func GetHelmServiceModule(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Param("productName")

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectKey].Service.View {
			ctx.UnAuthorized = true
			return
		}
	}

	revision, err := strconv.ParseInt(c.DefaultQuery("revision", "0"), 10, 64)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid revision number")
		return
	}
	ctx.Resp, ctx.Err = svcservice.GetHelmServiceModule(c.Param("serviceName"), projectKey, revision, false, ctx.Logger)
}

func GetFilePath(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Param("productName")

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectKey].Service.View {
			ctx.UnAuthorized = true
			return
		}
	}

	revision := int64(0)
	if len(c.Query("revision")) > 0 {
		revision, err = strconv.ParseInt(c.Query("revision"), 10, 64)
	}
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid revision number")
		return
	}
	ctx.Resp, ctx.Err = svcservice.GetFilePath(c.Param("serviceName"), projectKey, revision, c.Query("dir"), false, ctx.Logger)
}

func GetFileContent(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Param("productName")

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectKey].Service.View {
			ctx.UnAuthorized = true
			return
		}
	}

	param := new(svcservice.GetFileContentParam)
	err = c.ShouldBindQuery(param)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}

	ctx.Resp, ctx.Err = svcservice.GetFileContent(c.Param("serviceName"), projectKey, param, ctx.Logger)
}

func UpdateFileContent(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Query("projectName")

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectKey].Service.Edit {
			ctx.UnAuthorized = true
			return
		}
	}

	param := new(svcservice.HelmChartEditInfo)
	err = c.ShouldBind(param)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}

	param.Production = false
	ctx.Err = svcservice.EditFileContent(c.Param("serviceName"), projectKey, ctx.UserName, ctx.RequestID, param, ctx.Logger)
}

func UpdateProductionSvcFileContent(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Query("projectName")

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectKey].ProductionService.Edit {
			ctx.UnAuthorized = true
			return
		}
	}

	param := new(svcservice.HelmChartEditInfo)
	err = c.ShouldBind(param)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}

	param.Production = true
	ctx.Err = svcservice.EditFileContent(c.Param("name"), projectKey, ctx.UserName, ctx.RequestID, param, ctx.Logger)
}

func CreateOrUpdateHelmService(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Query("projectName")
	if projectKey == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("projectName can't be nil")
		return
	}

	args := new(svcservice.HelmServiceCreationArgs)
	if err := c.BindJSON(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid HelmService json args")
		return
	}
	args.CreatedBy, args.RequestID = ctx.UserName, ctx.RequestID

	bs, _ := json.Marshal(args)
	internalhandler.InsertOperationLog(c, ctx.UserName, projectKey, "新增", "项目管理-服务", fmt.Sprintf("服务名称:%s", args.Name), string(bs), ctx.Logger)

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectKey].Service.Create {
			ctx.UnAuthorized = true
			return
		}
	}

	ctx.Resp, ctx.Err = svcservice.CreateOrUpdateHelmService(projectKey, args, false, ctx.Logger)
}

func UpdateHelmService(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Query("projectName")
	if projectKey == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("projectName can't be nil")
		return
	}

	args := new(svcservice.HelmServiceCreationArgs)
	if err := c.BindJSON(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid HelmService json args")
		return
	}
	args.CreatedBy, args.RequestID = ctx.UserName, ctx.RequestID

	bs, _ := json.Marshal(args)
	internalhandler.InsertOperationLog(c, ctx.UserName, projectKey, "更新", "项目管理-服务", fmt.Sprintf("服务名称:%s", args.Name), string(bs), ctx.Logger)

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectKey].Service.Edit {
			ctx.UnAuthorized = true
			return
		}
	}

	ctx.Resp, ctx.Err = svcservice.CreateOrUpdateHelmService(projectKey, args, true, ctx.Logger)
}

func UpdateHelmProductionService(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Query("projectName")
	if projectKey == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("projectName can't be nil")
		return
	}

	args := new(svcservice.HelmServiceCreationArgs)
	if err := c.BindJSON(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid HelmService json args")
		return
	}
	args.CreatedBy, args.RequestID = ctx.UserName, ctx.RequestID

	bs, _ := json.Marshal(args)
	internalhandler.InsertOperationLog(c, ctx.UserName, projectKey, "更新", "项目管理-生产服务", fmt.Sprintf("服务名称:%s", args.Name), string(bs), ctx.Logger)

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectKey].ProductionService.Edit {
			ctx.UnAuthorized = true
			return
		}
	}

	args.Production = true
	ctx.Resp, ctx.Err = svcservice.CreateOrUpdateHelmService(projectKey, args, true, ctx.Logger)
}

func CreateOrUpdateBulkHelmServices(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Query("projectName")
	if projectKey == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("projectName can't be nil")
		return
	}

	args := new(svcservice.BulkHelmServiceCreationArgs)
	if err := c.BindJSON(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid HelmService json args")
		return
	}
	args.CreatedBy, args.RequestID = ctx.UserName, ctx.RequestID

	bs, _ := json.Marshal(args)
	internalhandler.InsertOperationLog(c, ctx.UserName, c.Query("projectName"), "新增", "项目管理-服务", "", string(bs), ctx.Logger)

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectKey].Service.Create {
			ctx.UnAuthorized = true
			return
		}
	}

	ctx.Resp, ctx.Err = svcservice.CreateOrUpdateBulkHelmService(projectKey, args, false, ctx.Logger)
}
