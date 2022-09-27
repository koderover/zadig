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
	"io"

	"github.com/gin-gonic/gin"

	buildservice "github.com/koderover/zadig/pkg/microservice/aslan/core/build/service"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/log"
)

func FindBuildModule(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = buildservice.FindBuild(c.Param("name"), c.Query("projectName"), ctx.Logger)
}

func ListBuildModules(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = buildservice.ListBuild(c.Query("name"), c.Query("targets"), c.Query("projectName"), ctx.Logger)
}

func ListBuildModulesByServiceModule(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	var excludeJenkins bool
	if c.Query("excludeJenkins") == "true" {
		excludeJenkins = true
	}

	ctx.Resp, ctx.Err = buildservice.ListBuildModulesByServiceModule(c.Query("encryptedKey"), c.Query("projectName"), excludeJenkins, ctx.Logger)
}

func CreateBuildModule(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(commonmodels.Build)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("CreateBuildModule c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("CreateBuildModule json.Unmarshal err : %v", err)
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, args.ProductName, "新增", "项目管理-构建", args.Name, string(data), ctx.Logger)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	err = c.BindJSON(args)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid Build args")
		return
	}

	ctx.Err = buildservice.CreateBuild(ctx.UserName, args, ctx.Logger)
}

func UpdateBuildModule(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(commonmodels.Build)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("UpdateBuildModule c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("UpdateBuildModule json.Unmarshal err : %v", err)
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, args.ProductName, "更新", "项目管理-构建", args.Name, string(data), ctx.Logger)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	err = c.BindJSON(args)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid Build args")
		return
	}
	ctx.Err = buildservice.UpdateBuild(ctx.UserName, args, ctx.Logger)
}

func DeleteBuildModule(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	name := c.Query("name")
	productName := c.Query("projectName")
	internalhandler.InsertOperationLog(c, ctx.UserName, productName, "删除", "项目管理-构建", name, "", ctx.Logger)

	if name == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("empty Name")
		return
	}

	ctx.Err = buildservice.DeleteBuild(name, productName, ctx.Logger)
}

func UpdateBuildTargets(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(struct {
		Name    string                              `json:"name"    binding:"required"`
		Targets []*commonmodels.ServiceModuleTarget `json:"targets" binding:"required"`
	})
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("UpdateBuildTargets c.GetRawData() err : %v", err)
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("UpdateBuildTargets json.Unmarshal err : %v", err)
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, c.Query("projectName"), "更新", "项目管理-服务组件", args.Name, string(data), ctx.Logger)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	if err := c.BindJSON(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}

	ctx.Err = buildservice.UpdateBuildTargets(args.Name, c.Query("projectName"), args.Targets, ctx.Logger)
}
