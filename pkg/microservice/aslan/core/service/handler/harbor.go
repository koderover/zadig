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

	"github.com/gin-gonic/gin"

	svcservice "github.com/koderover/zadig/pkg/microservice/aslan/core/service/service"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
)

func ListHarborProjects(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	ctx.Resp, ctx.Err = svcservice.ListHarborProjects(page, pageSize, ctx.Logger)
}

func ListHarborChartRepos(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	ctx.Resp, ctx.Err = svcservice.ListHarborChartRepos(c.Param("project"), ctx.Logger)
}

func ListHarborChartVersions(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	ctx.Resp, ctx.Err = svcservice.ListHarborChartVersions(c.Param("project"), c.Param("chart"), ctx.Logger)
}

func FindHarborChartDetail(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	ctx.Resp, ctx.Err = svcservice.FindHarborChartDetail(c.Param("project"), c.Param("chart"), c.Param("version"), ctx.Logger)
}
