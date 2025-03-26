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
	"fmt"
	"io"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/koderover/zadig/v2/pkg/types"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/system/service"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

func CreateInstall(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := new(commonmodels.Install)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("CreateInstall c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("CreateInstall json.Unmarshal err : %v", err)
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, "", "新增", "系统设置-应用设置", fmt.Sprintf("应用名称:%s,应用版本:%s", args.Name, args.Version), string(data), types.RequestBodyTypeJSON, ctx.Logger)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		ctx.UnAuthorized = true
		return
	}

	if err := c.ShouldBindWith(&args, binding.JSON); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid Install args")
		return
	}
	args.UpdateBy = ctx.UserName

	ctx.RespErr = service.CreateInstall(args, ctx.Logger)
}

func UpdateInstall(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := new(commonmodels.Install)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("UpdateInstall c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("UpdateInstall json.Unmarshal err : %v", err)
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, "", "更新", "系统设置-应用设置", fmt.Sprintf("应用名称:%s,应用版本:%s", args.Name, args.Version), string(data), types.RequestBodyTypeJSON, ctx.Logger)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		ctx.UnAuthorized = true
		return
	}

	if err := c.ShouldBindWith(&args, binding.JSON); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid Install args")
		return
	}
	args.UpdateBy = ctx.UserName
	name := args.Name
	version := args.Version
	if name == "" || version == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("名称或者版本号不能为空!")
		return
	}

	ctx.RespErr = service.UpdateInstall(name, version, args, ctx.Logger)
}

func GetInstall(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.RespErr = service.GetInstall(c.Param("name"), c.Param("version"), ctx.Logger)
}

func ListInstalls(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if c.Query("available") == "" || c.Query("available") != "true" {
		ctx.Resp, ctx.RespErr = service.ListInstalls(ctx.Logger)
	} else {
		ctx.Resp, ctx.RespErr = service.ListAvaiableInstalls(ctx.Logger)
	}
}

func DeleteInstall(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := new(commonmodels.Install)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("DeleteInstall c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("DeleteInstall json.Unmarshal err : %v", err)
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, "", "删除", "系统设置-应用设置", fmt.Sprintf("应用名称:%s,应用版本:%s", args.Name, args.Version), string(data), types.RequestBodyTypeJSON, ctx.Logger)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		ctx.UnAuthorized = true
		return
	}

	if err := c.ShouldBindWith(&args, binding.JSON); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid Install args")
		return
	}
	name := args.Name
	version := args.Version
	if name == "" || version == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("名称或者版本号不能为空!")
		return
	}

	ctx.RespErr = service.DeleteInstall(name, version, ctx.Logger)
}
