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
	"github.com/gin-gonic/gin"

	buildservice "github.com/koderover/zadig/pkg/microservice/aslan/core/build/service"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

func OpenAPICreateBuildModule(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	source := c.Query("source")

	if source == "template" {
		args := new(buildservice.OpenAPIBuildCreationFromTemplateReq)
		err := c.BindJSON(args)
		if err != nil {
			ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
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
	err := c.BindJSON(args)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
	}

	isValid, err := args.Validate()
	if !isValid {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	ctx.Err = buildservice.OpenAPICreateBuildModule(ctx.UserName, args, ctx.Logger)
}

func OpenAPIDeleteBuildModule(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	name := c.Query("name")
	productName := c.Query("projectName")
	internalhandler.InsertOperationLog(c, ctx.UserName, productName, "(OpenAPI)"+"删除", "项目管理-构建", name, "", ctx.Logger)

	if name == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("empty env name.")
		return
	}

	ctx.Err = buildservice.DeleteBuild(name, productName, ctx.Logger)
}

func OpenAPIListBuildModules(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	projectName := c.Query("projectName")
	if projectName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("empty project name.")
		return
	}

	args := new(buildservice.OpenAPIPageParamsFromReq)
	err := c.BindQuery(args)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	ctx.Resp, ctx.Err = buildservice.OpenAPIListBuildModules(projectName, args.PageNum, args.PageSize, ctx.Logger)
}

func OpenAPIGetBuildModule(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	name := c.Param("name")
	if name == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("empty buildName.")
		return
	}
	projectName := c.Query("projectName")
	if projectName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("empty projectName.")
		return
	}

	ctx.Resp, ctx.Err = buildservice.OpenAPIGetBuildModule(name, projectName, ctx.Logger)
}
