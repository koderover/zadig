/*
 * Copyright 2023 The KodeRover Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package handler

import (
	"fmt"

	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/release_plan/service"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

type ListReleasePlanOption struct {
	PageNum  int64 `form:"pageNum" binding:"required"`
	PageSize int64 `form:"pageSize" binding:"required"`
}

func ListReleasePlans(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	if !ctx.Resources.IsSystemAdmin && !ctx.Resources.SystemActions.ReleasePlan.View {
		ctx.UnAuthorized = true
		return
	}

	opt := new(ListReleasePlanOption)
	if err := c.ShouldBindQuery(&opt); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	ctx.Resp, ctx.Err = service.ListReleasePlans(opt.PageNum, opt.PageSize)
}

func GetReleasePlan(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	if !ctx.Resources.IsSystemAdmin && !ctx.Resources.SystemActions.ReleasePlan.View {
		ctx.UnAuthorized = true
		return
	}

	ctx.Resp, ctx.Err = service.GetReleasePlan(c.Param("id"))
}

func GetReleasePlanLogs(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Logger.Errorf("failed to generate authorization info for user: %s, error: %s", ctx.UserID, err)
		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	if !ctx.Resources.IsSystemAdmin && !ctx.Resources.SystemActions.ReleasePlan.View {
		ctx.UnAuthorized = true
		return
	}

	ctx.Resp, ctx.Err = service.GetReleasePlanLogs(c.Param("id"))
}

func CreateReleasePlan(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	if !ctx.Resources.IsSystemAdmin && !ctx.Resources.SystemActions.ReleasePlan.Create {
		ctx.UnAuthorized = true
		return
	}

	req := new(models.ReleasePlan)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	ctx.Err = service.CreateReleasePlan(ctx, req)
}

func UpdateReleasePlan(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	if !ctx.Resources.IsSystemAdmin && !ctx.Resources.SystemActions.ReleasePlan.Edit {
		ctx.UnAuthorized = true
		return
	}

	req := new(service.UpdateReleasePlanArgs)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	ctx.Err = service.UpdateReleasePlan(ctx, c.Param("id"), req)
}

func DeleteReleasePlan(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	if !ctx.Resources.IsSystemAdmin && !ctx.Resources.SystemActions.ReleasePlan.Delete {
		ctx.UnAuthorized = true
		return
	}

	ctx.Err = service.DeleteReleasePlan(c, ctx.UserName, c.Param("id"))
}

func ExecuteReleaseJob(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	if err != nil {

		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	req := new(service.ExecuteReleaseJobArgs)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	// only release plan manager can execute release job
	// so no need to check authorization there
	ctx.Err = service.ExecuteReleaseJob(ctx, c.Param("id"), req)
}

func UpdateReleaseJobStatus(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	if err != nil {

		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	// only release plan manager can execute release job
	// so no need to check authorization there
	ctx.Err = service.UpdateReleasePlanStatus(ctx, c.Param("id"), c.Param("status"))
}

func ApproveReleasePlan(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	if err != nil {

		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	req := new(service.ApproveRequest)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	ctx.Err = service.ApproveReleasePlan(ctx, c.Param("id"), req)
}
