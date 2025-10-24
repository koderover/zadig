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
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/gin-gonic/gin"
	"github.com/koderover/zadig/v2/pkg/types"

	buildservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/build/service"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

// @Summary 获取构建详情
// @Description
// @Tags 	build
// @Accept 	json
// @Produce json
// @Param 	projectKey		query		string							true	"项目标识"
// @Param 	name			path		string							true	"构建标识"
// @Success 200 			{object} 	commonmodels.Build
// @Router /api/aslan/build/build/{name} [get]
func FindBuildModule(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
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
			!ctx.Resources.ProjectAuthInfo[projectKey].Build.View {
			ctx.UnAuthorized = true
			return
		}
	}

	ctx.Resp, ctx.RespErr = buildservice.FindBuild(c.Param("name"), projectKey, ctx.Logger)
}

func ListBuildModules(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Query("projectName")

	// TODO: Authorization leak
	// this API is sometimes used in edit env scenario, thus giving the edit/create workflow permission
	// authorization check
	permitted := false

	if ctx.Resources.IsSystemAdmin {
		permitted = true
	} else if projectAuthInfo, ok := ctx.Resources.ProjectAuthInfo[projectKey]; ok {
		// first check if the user is projectAdmin
		if projectAuthInfo.IsProjectAdmin {
			permitted = true
		} else if projectAuthInfo.Env.EditConfig ||
			projectAuthInfo.Build.View {
			// then check if user has edit workflow permission
			permitted = true
		} else {
			// finally check if the permission is given by collaboration mode
			collaborationAuthorizedEdit, err := internalhandler.CheckPermissionGivenByCollaborationMode(ctx.UserID, projectKey, types.ResourceTypeEnvironment, types.EnvActionEditConfig)
			if err == nil && collaborationAuthorizedEdit {
				permitted = true
			}
		}
	}

	if !permitted {
		ctx.UnAuthorized = true
		return
	}

	ctx.Resp, ctx.RespErr = buildservice.ListBuild(c.Query("name"), c.Query("targets"), projectKey, ctx.Logger)
}

func ListBuildModulesByServiceModule(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Query("projectName")

	// TODO: Authorization leak
	// this API is sometimes used in edit/create workflow scenario, thus giving the edit/create workflow permission
	// authorization check
	permitted := false

	if ctx.Resources.IsSystemAdmin {
		permitted = true
	} else if ctx.Resources.SystemActions.Template.Create ||
		ctx.Resources.SystemActions.Template.Edit {
		permitted = true
	} else if projectAuthInfo, ok := ctx.Resources.ProjectAuthInfo[projectKey]; ok {
		// first check if the user is projectAdmin
		if projectAuthInfo.IsProjectAdmin {
			permitted = true
		} else if projectAuthInfo.Workflow.Edit ||
			projectAuthInfo.Workflow.Create ||
			projectAuthInfo.Build.View {
			// then check if user has edit workflow permission
			permitted = true
		} else {
			// finally check if the permission is given by collaboration mode
			collaborationAuthorizedEdit, err := internalhandler.CheckPermissionGivenByCollaborationMode(ctx.UserID, projectKey, types.ResourceTypeWorkflow, types.WorkflowActionEdit)
			if err == nil && collaborationAuthorizedEdit {
				permitted = true
			}
		}
	}

	if !permitted {
		ctx.UnAuthorized = true
		return
	}

	var excludeJenkins, updateServiceRevision bool
	if c.Query("excludeJenkins") == "true" {
		excludeJenkins = true
	}
	updateServiceRevision = c.Query("updateServiceRevision") == "true"
	envName := c.Query("envName")

	ctx.Resp, ctx.RespErr = buildservice.ListBuildModulesByServiceModule(c.Query("encryptedKey"), projectKey, envName, excludeJenkins, updateServiceRevision, ctx.Logger)
}

func CreateBuildModule(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := new(commonmodels.Build)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("CreateBuildModule c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("CreateBuildModule json.Unmarshal err : %v", err)
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, args.ProductName, "新增", "项目管理-构建", args.Name, args.Name, string(data), types.RequestBodyTypeJSON, ctx.Logger)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	err = c.BindJSON(args)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid Build args")
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[args.ProductName]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[args.ProductName].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[args.ProductName].Build.Create {
			ctx.UnAuthorized = true
			return
		}
	}

	ctx.RespErr = buildservice.CreateBuild(ctx.UserName, args, ctx.Logger)
}

// @Summary 更新构建
// @Description 如果仅需要更新服务和代码信息，则只需要更新target_repos字段
// @Tags 	build
// @Accept 	json
// @Produce json
// @Param 	body 			body 		commonmodels.Build		  true 	"body"
// @Success 200
// @Router /api/aslan/build/build [put]
func UpdateBuildModule(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := new(commonmodels.Build)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("UpdateBuildModule c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("UpdateBuildModule json.Unmarshal err : %v", err)
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, args.ProductName, "更新", "项目管理-构建", args.Name, args.Name, string(data), types.RequestBodyTypeJSON, ctx.Logger)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	err = c.BindJSON(args)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid Build args")
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[args.ProductName]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[args.ProductName].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[args.ProductName].Build.Edit {
			ctx.UnAuthorized = true
			return
		}
	}

	ctx.RespErr = buildservice.UpdateBuild(ctx.UserName, args, ctx.Logger)
}

func DeleteBuildModule(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	name := c.Query("name")
	projectKey := c.Query("projectName")
	internalhandler.InsertOperationLog(c, ctx.UserName, projectKey, "删除", "项目管理-构建", name, name, "", types.RequestBodyTypeJSON, ctx.Logger)

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectKey].Build.Delete {
			ctx.UnAuthorized = true
			return
		}
	}

	if name == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("empty Name")
		return
	}

	ctx.RespErr = buildservice.DeleteBuild(name, projectKey, ctx.Logger)
}

func UpdateBuildTargets(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Query("projectName")

	args := new(struct {
		Name    string                              `json:"name"    binding:"required"`
		Targets []*commonmodels.ServiceModuleTarget `json:"targets" binding:"required"`
	})
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("UpdateBuildTargets c.GetRawData() err : %v", err)
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("UpdateBuildTargets json.Unmarshal err : %v", err)
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, projectKey, "更新", "项目管理-服务组件", args.Name, args.Name, string(data), types.RequestBodyTypeJSON, ctx.Logger)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	if err := c.BindJSON(args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectKey].Build.Edit {
			ctx.UnAuthorized = true
			return
		}
	}

	ctx.RespErr = buildservice.UpdateBuildTargets(args.Name, c.Query("projectName"), args.Targets, ctx.Logger)
}
