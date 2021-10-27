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
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/environment/service"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

func GetServiceRenderCharts(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if c.Query("productName") == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("productName can not be null!")
		return
	}

	if c.Query("envName") == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("envName can not be null!")
		return
	}

	ctx.Resp, ctx.Err = service.GetRenderCharts(c.Query("productName"), c.Query("envName"), c.Query("serviceName"), ctx.Logger)
}

func GetProductDefaultValues(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if c.Query("productName") == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("productName can not be null!")
		return
	}

	if c.Query("envName") == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("envName can not be null!")
		return
	}

	ctx.Resp, ctx.Err = service.GetDefaultValues(c.Query("productName"), c.Query("envName"), ctx.Logger)
}

func GetYamlContent(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	var err error
	codehostID := 0

	codehostIDStr := c.Query("codehostID")
	if len(codehostIDStr) > 0 {
		codehostID, err = strconv.Atoi(codehostIDStr)
		if err != nil {
			ctx.Err = e.ErrInvalidParam.AddDesc("cannot convert codehost id to int")
			return
		}
	}

	if codehostID == 0 && len(c.Query("repoLink")) == 0 {
		ctx.Err = e.ErrInvalidParam.AddDesc("neither codehost nor repo link is specified")
		return
	}

	if len(c.Query("valuesPaths")) == 0 {
		ctx.Err = e.ErrInvalidParam.AddDesc("paths can't be empty")
		return
	}

	pathArr := strings.Split(c.Query("valuesPaths"), ",")
	var (
		owner    = c.Query("owner")
		repo     = c.Query("repo")
		branch   = c.Query("branch")
		repoLink = c.Query("repoLink")
	)
	ctx.Resp, ctx.Err = service.GetMergedYamlContent(codehostID, owner, repo, branch, repoLink, pathArr)
}
