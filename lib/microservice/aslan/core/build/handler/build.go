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
	"io/ioutil"

	"github.com/gin-gonic/gin"

	buildservice "github.com/koderover/zadig/lib/microservice/aslan/core/build/service"
	commonmodels "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
	internalhandler "github.com/koderover/zadig/lib/microservice/aslan/internal/handler"
	e "github.com/koderover/zadig/lib/tool/errors"
	log "github.com/koderover/zadig/lib/tool/xlog"
	"github.com/koderover/zadig/lib/types/permission"
)

func FindBuildModule(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()

	ctx.Resp, ctx.Err = buildservice.FindBuild(c.Param("name"), c.Param("version"), c.Query("productName"), ctx.Logger)
}

func ListBuildModules(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()

	ctx.Resp, ctx.Err = buildservice.ListBuild(c.Query("name"), c.Query("version"), c.Query("targets"), c.Query("productName"), ctx.Logger)
}

// CreateBuildModule ...
func CreateBuildModule(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()

	args := new(commonmodels.Build)
	data, err := c.GetRawData()
	if err != nil {
		log.Error("CreateBuildModule c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Error("CreateBuildModule json.Unmarshal err : %v", err)
	}
	internalhandler.InsertOperationLog(c, ctx.Username, args.ProductName, "新增", "工程管理-构建", args.Name, permission.BuildManageUUID, string(data), ctx.Logger)
	c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(data))

	err = c.BindJSON(args)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid Build args")
		return
	}

	ctx.Err = buildservice.CreateBuild(ctx.Username, args, ctx.Logger)
	return
}

func UpdateBuildModule(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()

	args := new(commonmodels.Build)
	data, err := c.GetRawData()
	if err != nil {
		log.Error("UpdateBuildModule c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Error("UpdateBuildModule json.Unmarshal err : %v", err)
	}
	internalhandler.InsertOperationLog(c, ctx.Username, args.ProductName, "更新", "工程管理-构建", args.Name, permission.BuildManageUUID, string(data), ctx.Logger)
	c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(data))

	err = c.BindJSON(args)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid Build args")
		return
	}
	ctx.Err = buildservice.UpdateBuild(ctx.Username, args, ctx.Logger)
	return
}

func DeleteBuildModule(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()

	name := c.Query("name")
	version := c.Query("version")
	productName := c.Query("productName")
	internalhandler.InsertOperationLog(c, ctx.Username, productName, "删除", "工程管理-构建", name, permission.BuildDeleteUUID, "", ctx.Logger)

	if name == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("empty Name")
		return
	}

	if version == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("empty Version")
		return
	}

	ctx.Err = buildservice.DeleteBuild(name, version, productName, ctx.Logger)
	return
}

func UpdateBuildTargets(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()

	args := new(struct {
		Name    string                              `json:"name"    binding:"required"`
		Targets []*commonmodels.ServiceModuleTarget `json:"targets" binding:"required"`
	})
	data, err := c.GetRawData()
	if err != nil {
		log.Error("UpdateBuildTargets c.GetRawData() err : %v", err)
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Error("UpdateBuildTargets json.Unmarshal err : %v", err)
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}
	internalhandler.InsertOperationLog(c, ctx.Username, c.Query("productName"), "更新", "工程管理-服务组件", args.Name, permission.BuildManageUUID, string(data), ctx.Logger)
	c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(data))

	if err := c.BindJSON(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}

	ctx.Err = buildservice.UpdateBuildTargets(args.Name, c.Query("productName"), args.Targets, ctx.Logger)
	return
}
