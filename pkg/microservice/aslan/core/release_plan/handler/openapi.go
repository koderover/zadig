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

	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/release_plan/service"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

type OpenAPIListReleasePlanOption struct {
	PageNum  int64 `form:"pageNum" binding:"required"`
	PageSize int64 `form:"pageSize" binding:"required"`
}

func OpenAPIListReleasePlans(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Logger.Errorf("failed to generate authorization info for user: %s, error: %s", ctx.UserID, err)
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	//if !ctx.Resources.IsSystemAdmin && !ctx.Resources.SystemActions.ReleasePlan.View {
	//	ctx.UnAuthorized = true
	//	return
	//}

	opt := new(OpenAPIListReleasePlanOption)
	if err := c.ShouldBindQuery(&opt); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	err = commonutil.CheckZadigEnterpriseLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	ctx.Resp, ctx.RespErr = service.OpenAPIListReleasePlans(opt.PageNum, opt.PageSize)
}

func OpenAPIGetReleasePlan(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Logger.Errorf("failed to generate authorization info for user: %s, error: %s", ctx.UserID, err)
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	//if !ctx.Resources.IsSystemAdmin && !ctx.Resources.SystemActions.ReleasePlan.View {
	//	ctx.UnAuthorized = true
	//	return
	//}

	err = commonutil.CheckZadigEnterpriseLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	ctx.Resp, ctx.RespErr = service.OpenAPIGetReleasePlan(c.Param("id"))
}

func OpenAPICreateReleasePlan(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Logger.Errorf("failed to generate authorization info for user: %s, error: %s", ctx.UserID, err)
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	if !ctx.Resources.IsSystemAdmin && !ctx.Resources.SystemActions.ReleasePlan.Create {
		ctx.UnAuthorized = true
		return
	}

	opt := new(service.OpenAPICreateReleasePlanArgs)
	if err := c.ShouldBindJSON(&opt); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	err = commonutil.CheckZadigEnterpriseLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	ctx.Resp, ctx.RespErr = service.OpenAPICreateReleasePlan(ctx, opt)
}

// @summary Update Release Plan
// @description Update Release Plan
// @tags 	Openapi
// @accept 	json
// @produce json
// @Param 	body 			body 		service.OpenAPIUpdateReleasePlanWithJobsArgs 				true 	"body"
// @success 200
// @router /openapi/release_plan/v1/{id} [patch]
func OpenAPIUpdateReleasePlanWithJobs(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Logger.Errorf("failed to generate authorization info for user: %s, error: %s", ctx.UserID, err)
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	if !ctx.Resources.IsSystemAdmin && !ctx.Resources.SystemActions.ReleasePlan.Edit {
		ctx.UnAuthorized = true
		return
	}

	opt := new(service.OpenAPIUpdateReleasePlanWithJobsArgs)
	if err := c.ShouldBindJSON(&opt); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	err = commonutil.CheckZadigEnterpriseLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	ctx.RespErr = service.OpenAPICreateReleasePlanWithJobs(ctx, c.Param("id"), opt)
}
