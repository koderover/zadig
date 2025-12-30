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
	"github.com/koderover/zadig/v2/pkg/tool/apisix"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
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

// @Summary Create APISIX Route
// @Description Create a new route in APISIX
// @Tags api_gateways
// @Accept json
// @Produce json
// @Param id path string true "API Gateway ID"
// @Param body body apisix.Route true "Route configuration"
// @Success 200
// @Router /api/aslan/system/api_gateways/{id}/apisix/routes [post]
func CreateApisixRoute(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	id := c.Param("id")
	req := new(apisix.Route)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}

	ctx.Resp, ctx.RespErr = service.CreateApisixRoute(id, req, ctx.Logger)
}

// @Summary Delete APISIX Route
// @Description Delete a route from APISIX
// @Tags api_gateways
// @Accept json
// @Produce json
// @Param id path string true "API Gateway ID"
// @Param route_id path string true "Route ID"
// @Success 200
// @Router /api/aslan/system/api_gateways/{id}/apisix/routes/{route_id} [delete]
func DeleteApisixRoute(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	id := c.Param("id")
	routeID := c.Param("route_id")

	ctx.RespErr = service.DeleteApisixRoute(id, routeID, ctx.Logger)
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

// @Summary Create APISIX Upstream
// @Description Create a new upstream in APISIX
// @Tags api_gateways
// @Accept json
// @Produce json
// @Param id path string true "API Gateway ID"
// @Param body body apisix.Upstream true "Upstream configuration"
// @Success 200
// @Router /api/aslan/system/api_gateways/{id}/apisix/upstreams [post]
func CreateApisixUpstream(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	id := c.Param("id")
	req := new(apisix.Upstream)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}

	ctx.Resp, ctx.RespErr = service.CreateApisixUpstream(id, req, ctx.Logger)
}

// @Summary Delete APISIX Upstream
// @Description Delete an upstream from APISIX
// @Tags api_gateways
// @Accept json
// @Produce json
// @Param id path string true "API Gateway ID"
// @Param upstream_id path string true "Upstream ID"
// @Success 200
// @Router /api/aslan/system/api_gateways/{id}/apisix/upstreams/{upstream_id} [delete]
func DeleteApisixUpstream(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	id := c.Param("id")
	upstreamID := c.Param("upstream_id")

	ctx.RespErr = service.DeleteApisixUpstream(id, upstreamID, ctx.Logger)
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

// @Summary Create APISIX Service
// @Description Create a new service in APISIX
// @Tags api_gateways
// @Accept json
// @Produce json
// @Param id path string true "API Gateway ID"
// @Param body body apisix.Service true "Service configuration"
// @Success 200
// @Router /api/aslan/system/api_gateways/{id}/apisix/services [post]
func CreateApisixService(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	id := c.Param("id")
	req := new(apisix.Service)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}

	ctx.Resp, ctx.RespErr = service.CreateApisixService(id, req, ctx.Logger)
}

// @Summary Delete APISIX Service
// @Description Delete a service from APISIX
// @Tags api_gateways
// @Accept json
// @Produce json
// @Param id path string true "API Gateway ID"
// @Param service_id path string true "Service ID"
// @Success 200
// @Router /api/aslan/system/api_gateways/{id}/apisix/services/{service_id} [delete]
func DeleteApisixService(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	id := c.Param("id")
	serviceID := c.Param("service_id")

	ctx.RespErr = service.DeleteApisixService(id, serviceID, ctx.Logger)
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

// @Summary Create APISIX Proto
// @Description Create a new proto in APISIX
// @Tags api_gateways
// @Accept json
// @Produce json
// @Param id path string true "API Gateway ID"
// @Param body body apisix.Proto true "Proto configuration"
// @Success 200
// @Router /api/aslan/system/api_gateways/{id}/apisix/protos [post]
func CreateApisixProto(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	id := c.Param("id")
	req := new(apisix.Proto)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}

	ctx.Resp, ctx.RespErr = service.CreateApisixProto(id, req, ctx.Logger)
}

// @Summary Delete APISIX Proto
// @Description Delete a proto from APISIX
// @Tags api_gateways
// @Accept json
// @Produce json
// @Param id path string true "API Gateway ID"
// @Param proto_id path string true "Proto ID"
// @Success 200
// @Router /api/aslan/system/api_gateways/{id}/apisix/protos/{proto_id} [delete]
func DeleteApisixProto(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	id := c.Param("id")
	protoID := c.Param("proto_id")

	ctx.RespErr = service.DeleteApisixProto(id, protoID, ctx.Logger)
}
