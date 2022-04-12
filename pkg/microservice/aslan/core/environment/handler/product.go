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
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/environment/service"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/koderover/zadig/pkg/types"
)

func CleanProductCronJob(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	service.CleanProductCronJob(ctx.RequestID, ctx.Logger)
}

func GetInitProduct(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	productTemplateName := c.Param("name")

	envType := types.EnvType(c.Query("envType"))
	isBaseEnvStr := c.Query("isBaseEnv")
	baseEnvName := c.Query("baseEnv")

	// Note: Compatible with previous behavior.
	isBaseEnv, err := strconv.ParseBool(isBaseEnvStr)
	if err != nil {
		log.Warnf("Failed to parse %s to bool: %s.", isBaseEnvStr, err)
	}

	if envType == "" {
		envType = types.GeneralEnv
	}

	ctx.Resp, ctx.Err = service.GetInitProduct(productTemplateName, envType, isBaseEnv, baseEnvName, ctx.Logger)
}
