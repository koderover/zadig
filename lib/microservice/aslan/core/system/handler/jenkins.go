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
	"github.com/gin-gonic/gin"

	commonmodels "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
	"github.com/koderover/zadig/lib/microservice/aslan/core/system/service"
	internalhandler "github.com/koderover/zadig/lib/microservice/aslan/internal/handler"
	e "github.com/koderover/zadig/lib/tool/errors"
	"github.com/koderover/zadig/lib/util"
)

func CreateJenkinsIntegration(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()

	args := new(commonmodels.JenkinsIntegration)
	if err := c.BindJSON(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid jenkinsIntegration json args")
		return
	}
	args.UpdateBy = ctx.Username
	if _, err := util.GetUrl(args.URL); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid url")
		return
	}
	ctx.Err = service.CreateJenkinsIntegration(args, ctx.Logger)
}

func ListJenkinsIntegration(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()

	ctx.Resp, ctx.Err = service.ListJenkinsIntegration(ctx.Logger)
}

func UpdateJenkinsIntegration(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()

	args := new(commonmodels.JenkinsIntegration)
	if err := c.BindJSON(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid jenkinsIntegration json args")
		return
	}
	args.UpdateBy = ctx.Username
	ctx.Err = service.UpdateJenkinsIntegration(c.Param("id"), args, ctx.Logger)
}

func DeleteJenkinsIntegration(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()

	ctx.Err = service.DeleteJenkinsIntegration(c.Param("id"), ctx.Logger)
}

func TestJenkinsConnection(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()

	args := new(service.JenkinsArgs)
	if err := c.BindJSON(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid jenkinsArgs json args")
		return
	}

	ctx.Err = service.TestJenkinsConnection(args, ctx.Logger)
}

func ListJobNames(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()

	ctx.Resp, ctx.Err = service.ListJobNames(ctx.Logger)
}

func ListJobBuildArgs(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()

	ctx.Resp, ctx.Err = service.ListJobBuildArgs(c.Param("jobName"), ctx.Logger)
}
