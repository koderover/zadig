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

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/system/service"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/log"
)

func GetProxyConfig(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	ctx.Resp = ProxyConfig{
		HTTPSAddr:  config.ProxyHTTPSAddr(),
		HTTPAddr:   config.ProxyHTTPAddr(),
		Socks5Addr: config.ProxySocks5Addr(),
	}
}

func ListProxies(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = service.ListProxies(ctx.Logger)
}

func GetProxy(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = service.GetProxy(c.Param("id"), ctx.Logger)
}

func CreateProxy(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(commonmodels.Proxy)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("CreateProxy c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("CreateProxy json.Unmarshal err : %v", err)
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, "", "新增", "代理", fmt.Sprintf("server:%s:%d", args.Address, args.Port), string(data), ctx.Logger)

	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	if err := c.ShouldBindWith(&args, binding.JSON); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid proxy args")
		return
	}
	args.UpdateBy = ctx.UserName

	ctx.Err = service.CreateProxy(args, ctx.Logger)
}

func UpdateProxy(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(commonmodels.Proxy)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("UpdateProxy c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("UpdateProxy json.Unmarshal err : %v", err)
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, "", "更新", "代理", fmt.Sprintf("id:%s", args.ID), string(data), ctx.Logger)

	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	if err := c.ShouldBindWith(&args, binding.JSON); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid proxy args")
		return
	}
	args.UpdateBy = ctx.UserName

	ctx.Err = service.UpdateProxy(c.Param("id"), args, ctx.Logger)
}

func DeleteProxy(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	internalhandler.InsertOperationLog(c, ctx.UserName, "", "删除", "代理", fmt.Sprintf("id:%s", c.Param("id")), "", ctx.Logger)

	ctx.Err = service.DeleteProxy(c.Param("id"), ctx.Logger)
}

func TestConnection(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

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
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid proxy args")
		return
	}

	ctx.Err = service.TestConnection(args, ctx.Logger)
}
