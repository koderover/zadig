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

	buildservice "github.com/koderover/zadig/pkg/microservice/aslan/core/build/service"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

func OpenAPICreateBuildModule(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Logger.Errorf("failed to generate authorization info for user: %s, error: %s", ctx.UserID, err)
		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	source := c.Query("source")

	if source == "template" {
		args := new(buildservice.OpenAPIBuildCreationFromTemplateReq)
		err := c.BindJSON(args)
		if err != nil {
			ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
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
			ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
			return
		}

		ctx.Err = buildservice.OpenAPICreateBuildModuleFromTemplate(ctx.UserName, args, ctx.Logger)
		return
	}

	args := new(buildservice.OpenAPIBuildCreationReq)
	err = c.BindJSON(args)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
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
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	ctx.Err = buildservice.OpenAPICreateBuildModule(ctx.UserName, args, ctx.Logger)
}

func OpenAPIDeleteBuildModule(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Logger.Errorf("failed to generate authorization info for user: %s, error: %s", ctx.UserID, err)
		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	name := c.Query("name")
	projectKey := c.Query("projectName")
	internalhandler.InsertOperationLog(c, ctx.UserName, projectKey, "(OpenAPI)"+"删除", "项目管理-构建", name, "", ctx.Logger)

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
		ctx.Err = e.ErrInvalidParam.AddDesc("empty env name.")
		return
	}

	ctx.Err = buildservice.DeleteBuild(name, projectKey, ctx.Logger)
}
