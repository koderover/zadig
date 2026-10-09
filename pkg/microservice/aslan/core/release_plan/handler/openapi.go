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

// @summary List Release Plan Custom Fields
// @description List the current release plan custom field definitions
// @tags 	OpenAPI
// @accept 	json
// @produce json
// @success 200 {array} models.ReleasePlanCustomFieldDefinition
// @router /openapi/release_plan/v1/custom_fields [get]
func OpenAPIListReleasePlanCustomFields(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Logger.Errorf("failed to generate authorization info for user: %s, error: %s", ctx.UserID, err)
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	err = commonutil.CheckZadigEnterpriseLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	ctx.Resp, ctx.RespErr = service.ListReleasePlanCustomFields()
}

// @summary Update Release Plan Custom Fields
// @description Replace custom field values on a release plan in planning status
// @tags OpenAPI
// @accept json
// @produce json
// @Param body body service.CustomFieldsUpdater true "body"
// @success 200
// @router /openapi/release_plan/v1/{id}/custom_fields [put]
func OpenAPIUpdateReleasePlanCustomFields(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}
	if !ctx.Resources.IsSystemAdmin && !ctx.Resources.SystemActions.ReleasePlan.EditMetadata {
		ctx.UnAuthorized = true
		return
	}

	args := new(service.CustomFieldsUpdater)
	if err := c.ShouldBindJSON(args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	if err := commonutil.CheckZadigEnterpriseLicense(); err != nil {
		ctx.RespErr = err
		return
	}

	ctx.RespErr = service.UpdateReleasePlan(ctx, c.Param("id"), &service.UpdateReleasePlanArgs{
		Verb: service.ActionUpdateCustomFields,
		Spec: args,
	})
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
// @tags 	OpenAPI
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

	// This OpenAPI updates metadata, approval, and jobs all at once, so require all edit permissions
	if !ctx.Resources.IsSystemAdmin {
		rp := ctx.Resources.SystemActions.ReleasePlan
		if !rp.EditMetadata || !rp.EditApproval || !rp.EditSubtasks {
			ctx.UnAuthorized = true
			return
		}
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

	ctx.RespErr = service.OpenAPIUpdateReleasePlanWithJobs(ctx, c.Param("id"), opt)
}
