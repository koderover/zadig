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

	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/system/service"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	"github.com/koderover/zadig/v2/pkg/types"
)

func CleanImageCache(c *gin.Context) {
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

	internalhandler.InsertOperationLog(c, ctx.UserName, "", "清理", "系统配置-缓存清理", "镜像缓存", "Image cache", "", types.RequestBodyTypeJSON, ctx.Logger)
	ctx.RespErr = service.CleanImageCache(ctx.Logger)
}

func SetCron(c *gin.Context) {
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

	arg := new(SetCleanConfig)
	if err := c.ShouldBindJSON(arg); err != nil {
		ctx.RespErr = err
		return
	}
	data, _ := json.Marshal(arg)
	internalhandler.InsertOperationLog(c, ctx.UserName, "", "更新", "系统配置-缓存清理", "定时清理配置", "Scheduled cache cleanup settings", string(data), types.RequestBodyTypeJSON, ctx.Logger)
	ctx.RespErr = service.SetCron(arg.Cron, arg.CronEnabled, ctx.Logger)
}

type SetCleanConfig struct {
	Cron        string `json:"cron"`
	CronEnabled bool   `json:"cron_enabled"`
}

func CleanCacheState(c *gin.Context) {
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

	ctx.Resp, ctx.RespErr = service.GetOrCreateCleanCacheState()
}

func CleanSharedStorage(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	internalhandler.InsertOperationLog(c, ctx.UserName, "", "清理", "系统配置-缓存清理", "共享存储", "Shared storage", "", types.RequestBodyTypeJSON, ctx.Logger)
	ctx.RespErr = service.CleanSharedStorage()
}
