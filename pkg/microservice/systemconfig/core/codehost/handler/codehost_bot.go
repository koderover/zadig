/*
Copyright 2026 The KodeRover Authors.

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
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/v2/pkg/microservice/systemconfig/core/codehost/service"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

// @Summary 获取代码源机器人身份配置
// @Description
// @Tags 	codehost
// @Accept 	json
// @Produce json
// @Param 	id		path		int								true	"代码源 ID"
// @Success 200 	{object} 	service.CodeHostBotResp
// @Router /api/v1/codehosts/{id}/bot [get]
func GetCodeHostBot(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	if !ctx.Resources.IsSystemAdmin {
		ctx.UnAuthorized = true
		return
	}

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid codehost id")
		return
	}

	ctx.Resp, ctx.RespErr = service.GetCodeHostBot(id, ctx.Logger)
}

// @Summary 配置代码源机器人身份
// @Description
// @Tags 	codehost
// @Accept 	json
// @Produce json
// @Param 	id		path		int								true	"代码源 ID"
// @Param 	body 	body 		service.CodeHostBotArgs 		true 	"机器人身份配置"
// @Success 200 	{object} 	service.CodeHostBotResp
// @Router /api/v1/codehosts/{id}/bot [patch]
func UpdateCodeHostBot(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	if !ctx.Resources.IsSystemAdmin {
		ctx.UnAuthorized = true
		return
	}

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid codehost id")
		return
	}

	args := new(service.CodeHostBotArgs)
	if err := c.ShouldBindJSON(args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid codehost bot args")
		return
	}

	ctx.Resp, ctx.RespErr = service.UpdateCodeHostBot(id, args, ctx.UserName, ctx.Logger)
}

// @Summary 删除代码源机器人身份
// @Description
// @Tags 	codehost
// @Accept 	json
// @Produce json
// @Param 	id		path		int								true	"代码源 ID"
// @Success 200
// @Router /api/v1/codehosts/{id}/bot [delete]
func DeleteCodeHostBot(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	if !ctx.Resources.IsSystemAdmin {
		ctx.UnAuthorized = true
		return
	}

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid codehost id")
		return
	}

	ctx.RespErr = service.DeleteCodeHostBot(id, ctx.Logger)
}
