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
	"io/ioutil"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"

	"github.com/qiniu/x/log.v7"

	"github.com/koderover/zadig/lib/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
	"github.com/koderover/zadig/lib/microservice/aslan/core/system/service"
	internalhandler "github.com/koderover/zadig/lib/microservice/aslan/internal/handler"
	e "github.com/koderover/zadig/lib/tool/errors"
	"github.com/koderover/zadig/lib/types/permission"
)

func GetProxyConfig(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()
	ctx.Resp = ProxyConfig{
		HTTPSAddr:  config.ProxyHTTPSAddr(),
		HTTPAddr:   config.ProxyHTTPAddr(),
		Socks5Addr: config.ProxySocks5Addr(),
	}
}

func ListProxies(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()

	ctx.Resp, ctx.Err = service.ListProxies(ctx.Logger)
}

func GetProxy(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()

	ctx.Resp, ctx.Err = service.GetProxy(c.Param("id"), ctx.Logger)
}

func CreateProxy(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()

	args := new(commonmodels.Proxy)
	data, err := c.GetRawData()
	if err != nil {
		log.Error("CreateProxy c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Error("CreateProxy json.Unmarshal err : %v", err)
	}
	internalhandler.InsertOperationLog(c, ctx.Username, "", "新增", "代理", fmt.Sprintf("server:%s:%d", args.Address, args.Port), permission.SuperUserUUID, string(data), ctx.Logger)

	c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(data))

	if err := c.ShouldBindWith(&args, binding.JSON); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid proxy args")
		return
	}
	args.UpdateBy = ctx.Username

	ctx.Err = service.CreateProxy(args, ctx.Logger)
}

func UpdateProxy(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()

	args := new(commonmodels.Proxy)
	data, err := c.GetRawData()
	if err != nil {
		log.Error("UpdateProxy c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Error("UpdateProxy json.Unmarshal err : %v", err)
	}
	internalhandler.InsertOperationLog(c, ctx.Username, "", "更新", "代理", fmt.Sprintf("id:%s", args.ID), permission.SuperUserUUID, string(data), ctx.Logger)

	c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(data))

	if err := c.ShouldBindWith(&args, binding.JSON); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid proxy args")
		return
	}
	args.UpdateBy = ctx.Username

	ctx.Err = service.UpdateProxy(c.Param("id"), args, ctx.Logger)
}

func DeleteProxy(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()

	internalhandler.InsertOperationLog(c, ctx.Username, "", "删除", "代理", fmt.Sprintf("id:%s", c.Param("id")), permission.SuperUserUUID, "", ctx.Logger)

	ctx.Err = service.DeleteProxy(c.Param("id"), ctx.Logger)
}

func TestConnection(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()

	args := new(commonmodels.Proxy)
	data, err := c.GetRawData()
	if err != nil {
		log.Error("TestConnection c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Error("TestConnection json.Unmarshal err : %v", err)
	}

	c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(data))

	if err := c.ShouldBindWith(&args, binding.JSON); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid proxy args")
		return
	}

	ctx.Err = service.TestConnection(args, ctx.Logger)
}
