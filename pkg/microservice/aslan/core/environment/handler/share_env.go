/*
Copyright 2022 The KodeRover Authors.

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

	"github.com/koderover/zadig/pkg/microservice/aslan/core/environment/service"
	"github.com/koderover/zadig/pkg/setting"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
)

func CheckWorkloadsK8sServices(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = service.CheckWorkloadsK8sServices(c, c.Param("name"), c.Query("projectName"))
}

func EnableBaseEnv(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, c.Query("projectName"), setting.OperationSceneEnv, "开启自测模式", "环境", c.Param("name"), "", ctx.Logger, c.Param("name"))

	ctx.Err = service.EnableBaseEnv(c, c.Param("name"), c.Query("projectName"))
}

func DisableBaseEnv(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, c.Query("projectName"), setting.OperationSceneEnv,
		"关闭自测模式", "环境", c.Param("name"),
		"", ctx.Logger, c.Param("name"))

	ctx.Err = service.DisableBaseEnv(c, c.Param("name"), c.Query("projectName"))
}

func CheckShareEnvReady(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = service.CheckShareEnvReady(c, c.Param("name"), c.Param("op"), c.Query("projectName"))
}
