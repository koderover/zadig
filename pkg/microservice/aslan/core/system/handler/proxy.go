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

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/system/service"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

func GetProxyConfig(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		ctx.UnAuthorized = true
		return
	}

	ctx.Resp = ProxyConfig{
		HTTPSAddr:  config.ProxyHTTPSAddr(),
		HTTPAddr:   config.ProxyHTTPAddr(),
		Socks5Addr: config.ProxySocks5Addr(),
	}
}

func ListProxies(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	// TODO: Authroization leaks
	// authorization checks
	//if !ctx.Resources.IsSystemAdmin {
	//	ctx.UnAuthorized = true
	//	return
	//}

	ctx.Resp, ctx.RespErr = service.ListProxies(ctx.Logger)
}

func GetProxy(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		ctx.UnAuthorized = true
		return
	}

	ctx.Resp, ctx.RespErr = service.GetProxy(c.Param("id"), ctx.Logger)
}

func CreateProxy(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := new(commonmodels.Proxy)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("CreateProxy c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("CreateProxy json.Unmarshal err : %v", err)
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, "", "新增", "代理", fmt.Sprintf("server:%s:%d", args.Address, args.Port), string(data), types.RequestBodyTypeJSON, ctx.Logger)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		ctx.UnAuthorized = true
		return
	}

	if err := c.ShouldBindWith(&args, binding.JSON); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid proxy args")
		return
	}
	args.UpdateBy = ctx.UserName

	ctx.RespErr = service.CreateProxy(args, ctx.Logger)
}

func UpdateProxy(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := new(commonmodels.Proxy)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("UpdateProxy c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("UpdateProxy json.Unmarshal err : %v", err)
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, "", "更新", "代理", fmt.Sprintf("id:%s", args.ID), string(data), types.RequestBodyTypeJSON, ctx.Logger)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		ctx.UnAuthorized = true
		return
	}

	if err := c.ShouldBindWith(&args, binding.JSON); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid proxy args")
		return
	}
	args.UpdateBy = ctx.UserName

	ctx.RespErr = service.UpdateProxy(c.Param("id"), args, ctx.Logger)
}

func DeleteProxy(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	internalhandler.InsertOperationLog(c, ctx.UserName, "", "删除", "代理", fmt.Sprintf("id:%s", c.Param("id")), "", types.RequestBodyTypeJSON, ctx.Logger)

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		ctx.UnAuthorized = true
		return
	}

	ctx.RespErr = service.DeleteProxy(c.Param("id"), ctx.Logger)
}

func TestConnection(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := new(commonmodels.Proxy)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("TestConnection c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("TestConnection json.Unmarshal err : %v", err)
	}

	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	if err := c.ShouldBindWith(&args, binding.JSON); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid proxy args")
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		ctx.UnAuthorized = true
		return
	}

	ctx.RespErr = service.TestConnection(args, ctx.Logger)
}
