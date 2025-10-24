/*
Copyright 2022 The KodeRover Authors.

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
	buildservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/build/service"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/types"
)

func OpenAPICreateBuildModule(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	source := c.Query("source")

	if source == "template" {
		args := new(buildservice.OpenAPIBuildCreationFromTemplateReq)
		err := c.BindJSON(args)
		if err != nil {
			ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
			return
		}

		// authorization checks
		if !ctx.Resources.IsSystemAdmin {
			if _, ok := ctx.Resources.ProjectAuthInfo[args.ProjectName]; !ok {
				ctx.UnAuthorized = true
				return
			}
			if !ctx.Resources.ProjectAuthInfo[args.ProjectName].IsProjectAdmin &&
				!ctx.Resources.ProjectAuthInfo[args.ProjectName].Build.Create {
				ctx.UnAuthorized = true
				return
			}
		}

		isValid, err := args.Validate()
		if !isValid {
			ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
			return
		}

		err = commonutil.CheckZadigProfessionalLicense()
		if err != nil {
			ctx.RespErr = err
			return
		}

		ctx.RespErr = buildservice.OpenAPICreateBuildModuleFromTemplate(ctx.UserName, args, ctx.Logger)
		return
	}

	args := new(buildservice.OpenAPIBuildCreationReq)
	err = c.BindJSON(args)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[args.ProjectName]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[args.ProjectName].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[args.ProjectName].Build.Create {
			ctx.UnAuthorized = true
			return
		}
	}

	isValid, err := args.Validate()
	if !isValid {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	ctx.RespErr = buildservice.OpenAPICreateBuildModule(ctx.UserName, args, ctx.Logger)
}

func OpenAPIDeleteBuildModule(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	buildName := c.Query("name")
	projectKey := c.Query("projectKey")
	internalhandler.InsertOperationLog(c, ctx.UserName, projectKey, "(OpenAPI)"+"删除", "项目管理-构建", buildName, buildName, "", types.RequestBodyTypeJSON, ctx.Logger)

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

	if buildName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("empty build name.")
		return
	}

	ctx.RespErr = buildservice.DeleteBuild(buildName, projectKey, ctx.Logger)
}

func OpenAPIListBuildModules(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Query("projectKey")
	if projectKey == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("empty project key.")
		return
	}

	args := new(buildservice.OpenAPIPageParamsFromReq)
	err = c.BindQuery(args)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

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

	ctx.Resp, ctx.RespErr = buildservice.OpenAPIListBuildModules(projectKey, args.PageNum, args.PageSize, ctx.Logger)
}

func OpenAPIGetBuildModule(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	name := c.Param("name")
	if name == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("empty buildName.")
		return
	}
	projectKey := c.Query("projectKey")
	if projectKey == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("empty projectKey.")
		return
	}

	serviceName := c.Query("serviceName")
	serviceModule := c.Query("serviceModule")

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

	ctx.Resp, ctx.RespErr = buildservice.OpenAPIGetBuildModule(name, serviceName, serviceModule, projectKey, ctx.Logger)
}
