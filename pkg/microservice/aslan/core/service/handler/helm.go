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
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	ctx.Resp, ctx.Err = svcservice.ListHelmServices(c.Param("productName"), ctx.Logger)
}

func GetHelmServiceModule(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	revision, err := strconv.ParseInt(c.DefaultQuery("revision", "0"), 10, 64)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid revision number")
		return
	}
	ctx.Resp, ctx.Err = svcservice.GetHelmServiceModule(c.Param("serviceName"), c.Param("productName"), revision, ctx.Logger)
}

func GetFilePath(c *gin.Context) {
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
	ctx.Resp, ctx.Err = svcservice.GetFilePath(c.Param("serviceName"), c.Param("productName"), revision, c.Query("dir"), ctx.Logger)
}

func GetFileContent(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	param := new(svcservice.GetFileContentParam)
	err := c.ShouldBindQuery(param)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}

	ctx.Resp, ctx.Err = svcservice.GetFileContent(c.Param("serviceName"), c.Param("productName"), param, ctx.Logger)
}

func UpdateFileContent(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	param := new(svcservice.HelmChartEditInfo)
	err := c.ShouldBind(param)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}

	ctx.Err = svcservice.EditFileContent(c.Param("serviceName"), c.Query("projectName"), ctx.UserName, ctx.RequestID, param, ctx.Logger)
}

func CreateOrUpdateHelmService(c *gin.Context) {
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

	bs, _ := json.Marshal(args)
	internalhandler.InsertOperationLog(c, ctx.UserName, projectName, "新增", "项目管理-服务", fmt.Sprintf("服务名称:%s", args.Name), string(bs), ctx.Logger)

	ctx.Resp, ctx.Err = svcservice.CreateOrUpdateHelmService(projectName, args, false, ctx.Logger)
}

func UpdateHelmService(c *gin.Context) {
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

	bs, _ := json.Marshal(args)
	internalhandler.InsertOperationLog(c, ctx.UserName, projectName, "更新", "项目管理-服务", fmt.Sprintf("服务名称:%s", args.Name), string(bs), ctx.Logger)

	ctx.Resp, ctx.Err = svcservice.CreateOrUpdateHelmService(projectName, args, true, ctx.Logger)
}

func CreateOrUpdateBulkHelmServices(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	projectName := c.Query("projectName")
	if projectName == "" {
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

	ctx.Resp, ctx.Err = svcservice.CreateOrUpdateBulkHelmService(projectName, args, false, ctx.Logger)
}
