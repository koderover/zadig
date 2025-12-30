/*
 * Copyright 2025 The KodeRover Authors.
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

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/system/service"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
)

// @Summary List Api Gateways
// @Description List Api Gateways
// @Tags 	system
// @Accept 	json
// @Produce json
// @Success 200 	{array} 	models.ApiGateway
// @Router /api/aslan/system/api_gateways [get]
func ListApiGateway(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	// authorization checks
	// TODO: Authorization leak
	// comment: this API should only be used when the user requires IM app's full information, including AK/SK
	// however this is currently used in multiple situation, thus having serious security leaks.
	// if !ctx.Resources.IsSystemAdmin {
	// 	ctx.UnAuthorized = true
	// 	return
	// }

	apiGateways, err := service.ListApiGateway(ctx.Logger)
	if err != nil {
		ctx.RespErr = err
		return
	}

	// Hide token for security
	for _, gw := range apiGateways {
		gw.Token = ""
	}

	ctx.Resp = apiGateways
}

// @Summary Create Api Gateway
// @Description Create Api Gateway
// @Tags 	system
// @Accept 	json
// @Produce json
// @Param 	body 	body 		models.ApiGateway 	true 	"body"
// @Success 200
// @Router /api/aslan/system/api_gateways [post]
func CreateApiGateway(c *gin.Context) {
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

	req := new(models.ApiGateway)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.RespErr = err
		return
	}

	ctx.RespErr = service.CreateApiGateway(req, ctx.Logger)
}

// @Summary Update Api Gateway
// @Description Update Api Gateway
// @Tags 	system
// @Accept 	json
// @Produce json
// @Param 	id 		path 		string 				true 	"id"
// @Param 	body 	body 		models.ApiGateway 	true 	"body"
// @Success 200
// @Router /api/aslan/system/api_gateways/{id} [patch]
func UpdateApiGateway(c *gin.Context) {
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

	req := new(models.ApiGateway)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.RespErr = err
		return
	}

	ctx.RespErr = service.UpdateApiGateway(c.Param("id"), req, ctx.Logger)
}

// @Summary Delete Api Gateway
// @Description Delete Api Gateway
// @Tags 	system
// @Accept 	json
// @Produce json
// @Param 	id 		path 		string 	true 	"id"
// @Success 200
// @Router /api/aslan/system/api_gateways/{id} [delete]
func DeleteApiGateway(c *gin.Context) {
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

	ctx.RespErr = service.DeleteApiGateway(c.Param("id"), ctx.Logger)
}

// @Summary Validate Api Gateway
// @Description Validate Api Gateway connection
// @Tags 	system
// @Accept 	json
// @Produce json
// @Param 	body 	body 		models.ApiGateway 	true 	"body"
// @Success 200
// @Router /api/aslan/system/api_gateways/validate [post]
func ValidateApiGateway(c *gin.Context) {
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

	req := new(models.ApiGateway)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.RespErr = err
		return
	}

	ctx.RespErr = service.ValidateApiGateway(req, ctx.Logger)
}

