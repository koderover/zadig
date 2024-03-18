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
	"fmt"

	"github.com/gin-gonic/gin"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	templateservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/templatestore/service"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

// @summary Get Release Plan Template by ID
// @description Get Release Plan Template by ID
// @tags 	template
// @accept 	json
// @produce json
// @param 	id 				path 		string 						 	true 	"template ID"
// @success 200 			{object} 	commonmodels.ReleasePlanTemplate
// @router /api/aslan/template/release_plan/{id} [get]
func GetReleasePlanTemplateByID(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)

	if err != nil {
		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		internalhandler.JSONResponse(c, ctx)
		return
	}

	// authorization check
	if !ctx.Resources.IsSystemAdmin {
		if !ctx.Resources.SystemActions.Template.View && !ctx.Resources.SystemActions.ReleasePlan.Create {
			ctx.UnAuthorized = true
			return
		}
	}

	ctx.Resp, ctx.Err = templateservice.GetReleasePlanTemplateByID(c.Param("id"), ctx.Logger)
	return
}

// @Summary List Release Plan Template
// @Description List Release Plan Template
// @Tags 	template
// @Accept 	json
// @Produce json
// @Success 200 			{array} 	commonmodels.ReleasePlanTemplate
// @Router /api/aslan/template/release_plan [get]
func ListReleasePlanTemplate(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	// authorization check
	if !ctx.Resources.IsSystemAdmin {
		if !ctx.Resources.SystemActions.Template.View && !ctx.Resources.SystemActions.ReleasePlan.Create {
			ctx.UnAuthorized = true
			return
		}
	}

	ctx.Resp, ctx.Err = templateservice.ListReleasePlanTemplate(ctx.Logger)
}

// @summary Create Release Plan Template
// @description Create Release Plan Template
// @tags 	template
// @accept 	json
// @produce json
// @Param 	body 			body 		commonmodels.ReleasePlanTemplate 				true 	"body"
// @success 200
// @router /api/aslan/template/release_plan [post]
func CreateReleasePlanTemplate(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	// authorization check
	if !ctx.Resources.IsSystemAdmin {
		if !ctx.Resources.SystemActions.Template.Create {
			ctx.UnAuthorized = true
			return
		}
	}

	if err = commonutil.CheckZadigProfessionalLicense(); err != nil {
		ctx.Err = err
		return
	}

	args := new(commonmodels.ReleasePlanTemplate)

	if err := c.ShouldBindJSON(&args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	ctx.Err = templateservice.CreateReleasePlanTemplate(ctx.UserName, args, ctx.Logger)
}

// @summary Update Release Plan Template
// @description Update Release Plan Template
// @tags 	template
// @accept 	json
// @produce json
// @Param 	body 			body 		commonmodels.ReleasePlanTemplate 				true 	"body"
// @success 200
// @router /api/aslan/template/release_plan [put]
func UpdateReleasePlanTemplate(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	// authorization check
	if !ctx.Resources.IsSystemAdmin {
		if !ctx.Resources.SystemActions.Template.Edit {
			ctx.UnAuthorized = true
			return
		}
	}

	if err = commonutil.CheckZadigProfessionalLicense(); err != nil {
		ctx.Err = err
		return
	}

	args := new(commonmodels.ReleasePlanTemplate)

	if err := c.ShouldBindYAML(&args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	ctx.Err = templateservice.UpdateReleasePlanTemplate(ctx.UserName, args, ctx.Logger)
}

// @summary Delete Release Plan Template by ID
// @description Delete Release Plan Template by ID
// @tags 	template
// @accept 	json
// @produce json
// @param 	id 				path 		string 						 	true 	"template ID"
// @success 200
// @router /api/aslan/template/release_plan/{id} [delete]
func DeleteReleasePlanTemplateByID(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	// authorization check
	if !ctx.Resources.IsSystemAdmin {
		if !ctx.Resources.SystemActions.Template.Delete {
			ctx.UnAuthorized = true
			return
		}
	}

	ctx.Err = templateservice.DeleteReleasePlanTemplateByID(c.Param("id"), ctx.Logger)
}
