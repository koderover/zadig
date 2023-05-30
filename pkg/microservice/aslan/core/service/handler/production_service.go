/*
Copyright 2023 The KodeRover Authors.

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

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/service/service"
	svcservice "github.com/koderover/zadig/pkg/microservice/aslan/core/service/service"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

func ListProductionServices(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = service.ListProductionServices(c.Query("projectName"), ctx.Logger)
}

func GetProductionK8sService(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = service.GetProductionK8sService(c.Param("name"), c.Query("projectName"), ctx.Logger)
}

func GetProductionK8sServiceOption(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = service.GetProductionK8sServiceOption(c.Param("name"), c.Query("projectName"), ctx.Logger)
}

func CreateK8sProductionService(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(commonmodels.Service)
	if err := c.BindJSON(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}

	rawArg, _ := json.Marshal(args)
	internalhandler.InsertOperationLog(c, ctx.UserName, args.ProductName, "新增", "项目管理-生产服务", fmt.Sprintf("服务名称:%s", args.ServiceName), string(rawArg), ctx.Logger)
	args.CreateBy = ctx.UserName
	ctx.Resp, ctx.Err = service.CreateK8sProductionService(c.Query("projectName"), args, ctx.Logger)
}

func UpdateK8sProductionServiceVariables(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(commonservice.ServiceTmplObject)
	if err := c.ShouldBindJSON(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}

	args.ProductName = c.Query("projectName")
	args.ServiceName = c.Param("name")
	args.Username = ctx.UserName
	rawArg, _ := json.Marshal(args)
	internalhandler.InsertOperationLog(c, ctx.UserName, args.ProductName, "更新", "项目管理-生产服务变量", fmt.Sprintf("服务名称:%s", args.ServiceName), string(rawArg), ctx.Logger)

	ctx.Err = svcservice.UpdateProductionServiceVariables(args)
}

func DeleteProductionService(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	internalhandler.InsertOperationLog(c, ctx.UserName, c.Query("projectName"), "删除", "项目管理-生产服务", fmt.Sprintf("服务名称:%s", c.Param("name")), "", ctx.Logger)

	ctx.Err = svcservice.DeleteProductionServiceTemplate(c.Param("name"), c.Query("projectName"), ctx.Logger)
}

func CreateHelmProductionService(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	projectName := c.Query("projectName")
	if projectName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("projectName can't be nil")
		return
	}

	args := new(svcservice.HelmServiceCreationArgs)
	if err := c.BindJSON(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid HelmService json args")
		return
	}

	args.CreatedBy, args.RequestID = ctx.UserName, ctx.RequestID
	args.Production = true

	bs, _ := json.Marshal(args)
	internalhandler.InsertOperationLog(c, ctx.UserName, projectName, "新增", "项目管理-生产服务", fmt.Sprintf("服务名称:%s", args.Name), string(bs), ctx.Logger)
	ctx.Resp, ctx.Err = svcservice.CreateOrUpdateHelmService(projectName, args, false, ctx.Logger)
}

func GetProductionHelmServiceModule(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	revision, err := strconv.ParseInt(c.DefaultQuery("revision", "0"), 10, 64)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid revision number")
		return
	}

	ctx.Resp, ctx.Err = svcservice.GetHelmServiceModule(c.Param("name"), c.Query("projectName"), revision, true, ctx.Logger)
}

func GetProductionHelmFilePath(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	revision := int64(0)
	var err error
	if len(c.Query("revision")) > 0 {
		revision, err = strconv.ParseInt(c.Query("revision"), 10, 64)
	}
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid revision number")
		return
	}
	ctx.Resp, ctx.Err = svcservice.GetProductionHelmServiceFilePath(c.Param("name"), c.Query("projectName"), revision, c.Query("dir"), ctx.Logger)
}

func GetProductionHelmFileContent(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	param := new(svcservice.GetFileContentParam)
	err := c.ShouldBindQuery(param)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}

	ctx.Resp, ctx.Err = svcservice.GetProductionServiceFileContent(c.Param("name"), c.Query("projectName"), param, ctx.Logger)
}

func UpdateProductionHelmReleaseNaming(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	projectName := c.Query("projectName")
	if projectName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("projectName can't be nil")
		return
	}

	args := new(svcservice.ReleaseNamingRule)
	if err := c.BindJSON(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid yaml args")
		return
	}

	bs, _ := json.Marshal(args)
	internalhandler.InsertOperationLog(c, ctx.UserName, projectName, "修改", "项目管理-生产服务", args.ServiceName, string(bs), ctx.Logger)

	ctx.Err = svcservice.UpdateProductionServiceReleaseNamingRule(ctx.UserName, ctx.RequestID, projectName, args, ctx.Logger)
}
