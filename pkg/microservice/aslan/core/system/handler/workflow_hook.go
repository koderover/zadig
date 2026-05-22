/*
Copyright 2025 The KodeRover Authors.

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

	"github.com/gin-gonic/gin"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/system/service"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

// @Summary 获取系统工作流 WebHook 配置
// @Description 获取系统工作流 WebHook 配置
// @Tags system
// @Accept json
// @Produce json
// @Success 200 {object} models.WorkflowHookSettings
// @Router /api/aslan/system/workflowHook [get]
func GetWorkflowHookSetting(c *gin.Context) {
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

	ctx.Resp, ctx.RespErr = service.GetWorkflowHookSetting()
}

// @Summary 更新系统工作流 WebHook 配置
// @Description 更新系统工作流 WebHook 配置
// @Tags system
// @Accept json
// @Produce json
// @Param body body models.WorkflowHookSettings true "body"
// @Success 200
// @Router /api/aslan/system/workflowHook [put]
func UpdateWorkflowHookSetting(c *gin.Context) {
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

	req := new(commonmodels.WorkflowHookSettings)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	ctx.RespErr = service.UpdateWorkflowHookSetting(req)
}
