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
// @Router /api/aslan/system/api_gateways/{id}/apisix/route [get]
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
// @Router /api/aslan/system/api_gateways/{id}/apisix/upstream [get]
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
// @Router /api/aslan/system/api_gateways/{id}/apisix/service [get]
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
// @Router /api/aslan/system/api_gateways/{id}/apisix/proto [get]
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

// @Summary Get APISIX Route
// @Description Get a route by ID from APISIX
// @Tags api_gateways
// @Accept json
// @Produce json
// @Param id path string true "API Gateway ID"
// @Param route_id path string true "Route ID"
// @Success 200
// @Router /api/aslan/system/api_gateways/{id}/apisix/route/{route_id} [get]
func GetApisixRoute(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	id := c.Param("id")
	routeID := c.Param("route_id")

	ctx.Resp, ctx.RespErr = service.GetApisixRoute(id, routeID, ctx.Logger)
}

// @Summary Get APISIX Upstream
// @Description Get an upstream by ID from APISIX
// @Tags api_gateways
// @Accept json
// @Produce json
// @Param id path string true "API Gateway ID"
// @Param upstream_id path string true "Upstream ID"
// @Success 200
// @Router /api/aslan/system/api_gateways/{id}/apisix/upstream/{upstream_id} [get]
func GetApisixUpstream(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	id := c.Param("id")
	upstreamID := c.Param("upstream_id")

	ctx.Resp, ctx.RespErr = service.GetApisixUpstream(id, upstreamID, ctx.Logger)
}

// @Summary Get APISIX Service
// @Description Get a service by ID from APISIX
// @Tags api_gateways
// @Accept json
// @Produce json
// @Param id path string true "API Gateway ID"
// @Param service_id path string true "Service ID"
// @Success 200
// @Router /api/aslan/system/api_gateways/{id}/apisix/service/{service_id} [get]
func GetApisixService(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	id := c.Param("id")
	serviceID := c.Param("service_id")

	ctx.Resp, ctx.RespErr = service.GetApisixService(id, serviceID, ctx.Logger)
}

// @Summary Get APISIX Proto
// @Description Get a proto by ID from APISIX
// @Tags api_gateways
// @Accept json
// @Produce json
// @Param id path string true "API Gateway ID"
// @Param proto_id path string true "Proto ID"
// @Success 200
// @Router /api/aslan/system/api_gateways/{id}/apisix/proto/{proto_id} [get]
func GetApisixProto(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	id := c.Param("id")
	protoID := c.Param("proto_id")

	ctx.Resp, ctx.RespErr = service.GetApisixProto(id, protoID, ctx.Logger)
}
