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
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/system/service"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
)

// @Summary List APISIX Routes
// @Description List all routes from APISIX
// @Tags api_gateways
// @Accept json
// @Produce json
// @Param id path string true "API Gateway ID"
// @Param page query int false "Page number"
// @Param page_size query int false "Page size"
// @Success 200
// @Router /api/aslan/system/api_gateways/{id}/apisix/routes [get]
func ListApisixRoutes(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	id := c.Param("id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "0"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "0"))

	ctx.Resp, ctx.RespErr = service.ListApisixRoutes(id, page, pageSize, ctx.Logger)
}

// @Summary List APISIX Upstreams
// @Description List all upstreams from APISIX
// @Tags api_gateways
// @Accept json
// @Produce json
// @Param id path string true "API Gateway ID"
// @Param page query int false "Page number"
// @Param page_size query int false "Page size"
// @Success 200
// @Router /api/aslan/system/api_gateways/{id}/apisix/upstreams [get]
func ListApisixUpstreams(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	id := c.Param("id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "0"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "0"))

	ctx.Resp, ctx.RespErr = service.ListApisixUpstreams(id, page, pageSize, ctx.Logger)
}

// @Summary List APISIX Services
// @Description List all services from APISIX
// @Tags api_gateways
// @Accept json
// @Produce json
// @Param id path string true "API Gateway ID"
// @Param page query int false "Page number"
// @Param page_size query int false "Page size"
// @Success 200
// @Router /api/aslan/system/api_gateways/{id}/apisix/services [get]
func ListApisixServices(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	id := c.Param("id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "0"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "0"))

	ctx.Resp, ctx.RespErr = service.ListApisixServices(id, page, pageSize, ctx.Logger)
}

// @Summary List APISIX Protos
// @Description List all protos from APISIX
// @Tags api_gateways
// @Accept json
// @Produce json
// @Param id path string true "API Gateway ID"
// @Param page query int false "Page number"
// @Param page_size query int false "Page size"
// @Success 200
// @Router /api/aslan/system/api_gateways/{id}/apisix/protos [get]
func ListApisixProtos(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	id := c.Param("id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "0"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "0"))

	ctx.Resp, ctx.RespErr = service.ListApisixProtos(id, page, pageSize, ctx.Logger)
}
