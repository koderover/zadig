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
	"strings"

	"github.com/gin-gonic/gin"

	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/environment/service"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

func GetServiceRenderCharts(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if c.Query("projectName") == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("productName can not be null!")
		return
	}

	if c.Query("envName") == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("envName can not be null!")
		return
	}

	ctx.Resp, _, ctx.Err = commonservice.GetSvcRenderArgs(c.Query("projectName"), c.Query("envName"), c.Query("serviceName"), ctx.Logger)
}

func GetServiceVariables(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if c.Query("projectName") == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("productName can not be null!")
		return
	}

	ctx.Resp, _, ctx.Err = commonservice.GetK8sSvcRenderArgs(c.Query("projectName"), c.Query("envName"), c.Query("serviceName"), ctx.Logger)
}

func GetProductDefaultValues(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if c.Query("projectName") == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("productName can not be null!")
		return
	}

	if c.Query("envName") == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("envName can not be null!")
		return
	}

	ctx.Resp, ctx.Err = service.GetDefaultValues(c.Query("projectName"), c.Query("envName"), ctx.Logger)
}

func GetYamlContent(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	arg := &service.YamlContentRequestArg{}

	if err := c.ShouldBindQuery(arg); err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}
	if arg.CodehostID == 0 && len(arg.RepoLink) == 0 {
		ctx.Err = e.ErrInvalidParam.AddDesc("neither codehost nor repo link is specified")
		return
	}

	if len(arg.ValuesPaths) == 0 {
		ctx.Err = e.ErrInvalidParam.AddDesc("paths can't be empty")
		return
	}

	pathArr := strings.Split(arg.ValuesPaths, ",")
	ctx.Resp, ctx.Err = service.GetMergedYamlContent(arg, pathArr)
}
