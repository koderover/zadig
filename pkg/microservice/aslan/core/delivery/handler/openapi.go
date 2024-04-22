/*
Copyright 2024 The KodeRover Authors.

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
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/delivery/service"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

type OpenAPIListDeliveryVersionRequest struct {
	ProjectName string `form:"projectName" binding:"required"`
	PageNum     int    `form:"pageNum" binding:"required"`
	PageSize    int    `form:"pageSize" binding:"required"`
}

// @Summary OpenAPI List Delivery Version
// @Description OpenAPI List Delivery Version
// @Tags 	OpenAPI
// @Accept 	json
// @Produce json
// @Param 	projectName		query		string								true	"project name"
// @Param 	pageNum 		query		string								true	"page num"
// @Param 	pageSize 		query		string								true	"page size"
// @Success 200
// @Router /openapi/delivery/releases [get]
func OpenAPIListDeliveryVersion(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Logger.Errorf("failed to generate authorization info for user: %s, error: %s", ctx.UserID, err)
		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	req := new(OpenAPIListDeliveryVersionRequest)
	if err := c.ShouldBindQuery(&req); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	projectKey := req.ProjectName
	if !ctx.Resources.IsSystemAdmin {
		if projectKey == "" {
			if !ctx.Resources.SystemActions.DeliveryCenter.ViewVersion {
				ctx.UnAuthorized = true
				return
			}
		} else {
			if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
				ctx.UnAuthorized = true
				return
			}

			if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
				!ctx.Resources.ProjectAuthInfo[projectKey].Version.View {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	if req.PageNum <= 0 {
		req.PageNum = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}

	err = commonutil.CheckZadigProfessionalLicense()
	if err != nil {
		ctx.Err = err
		return
	}

	ctx.Resp, ctx.Err = service.OpenAPIListDeliveryVersion(req.ProjectName, req.PageNum, req.PageSize)
}

// @Summary OpenAPI Get Delivery Version
// @Description OpenAPI Get Delivery Version
// @Tags 	OpenAPI
// @Accept 	json
// @Produce json
// @Param 	projectName		query		string								true	"project name"
// @Success 200
// @Router /openapi/delivery/releases/{id} [get]
func OpenAPIGetDeliveryVersion(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Logger.Errorf("failed to generate authorization info for user: %s, error: %s", ctx.UserID, err)
		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	//params validate
	ID := c.Param("id")
	if ID == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("id can't be empty!")
		return
	}
	projectKey := c.Query("projectName")
	if projectKey == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("projectName can't be empty!")
		return
	}

	permit := false
	if ctx.Resources.IsSystemAdmin {
		permit = true
	} else {
		if ctx.Resources.SystemActions.DeliveryCenter.ViewVersion {
			permit = true
		}

		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; ok {
			if ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin ||
				ctx.Resources.ProjectAuthInfo[projectKey].Version.View {
				permit = true
			}
		}
	}

	if !permit {
		ctx.UnAuthorized = true
		return
	}

	err = commonutil.CheckZadigProfessionalLicense()
	if err != nil {
		ctx.Err = err
		return
	}

	ctx.Resp, ctx.Err = service.OpenAPIGetDeliveryVersion(c.Param("id"))
}

// @Summary OpenAPI Delete Delivery Version
// @Description OpenAPI Delete Delivery Version
// @Tags 	OpenAPI
// @Accept 	json
// @Produce json
// @Param 	projectName		query		string								true	"project name"
// @Success 200
// @Router /openapi/delivery/releases/{id} [delete]
func OpenAPIDeleteDeliveryVersion(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Logger.Errorf("failed to generate authorization info for user: %s, error: %s", ctx.UserID, err)
		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	//params validate
	ID := c.Param("id")
	if ID == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("id can't be empty!")
		return
	}
	projectKey := c.Query("projectName")
	if projectKey == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("projectName can't be empty!")
		return
	}

	permit := false
	if ctx.Resources.IsSystemAdmin {
		permit = true
	} else {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; ok {
			if ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin ||
				ctx.Resources.ProjectAuthInfo[projectKey].Version.Delete {
				permit = true
			}
		}
	}

	if !permit {
		ctx.UnAuthorized = true
		return
	}

	err = commonutil.CheckZadigProfessionalLicense()
	if err != nil {
		ctx.Err = err
		return
	}

	ctx.Err = service.OpenAPIDeleteDeliveryVersion(c.Param("id"))
}

// @Summary OpenAPI Create K8S Delivery Version
// @Description OpenAPI Create K8S Delivery Version
// @Tags 	OpenAPI
// @Accept 	json
// @Produce json
// @Param 	body 			body 		service.OpenAPICreateK8SDeliveryVersionRequest  true 	"body"
// @Success 200
// @Router /openapi/delivery/releases/k8s [post]
func OpenAPICreateK8SDeliveryVersion(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Logger.Errorf("failed to generate authorization info for user: %s, error: %s", ctx.UserID, err)
		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	req := new(service.OpenAPICreateK8SDeliveryVersionRequest)
	if err := c.ShouldBindJSON(&req); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	req.CreateBy = ctx.UserName

	projectKey := req.ProjectName
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectKey].Version.Create {
			ctx.UnAuthorized = true
			return
		}
	}

	err = commonutil.CheckZadigProfessionalLicense()
	if err != nil {
		ctx.Err = err
		return
	}

	ctx.Err = service.OpenAPICreateK8SDeliveryVersion(req)
}

// @Summary OpenAPI Create Helm Delivery Version
// @Description OpenAPI Create Helm Delivery Version
// @Tags 	OpenAPI
// @Accept 	json
// @Produce json
// @Param 	body 			body 		service.OpenAPICreateHelmDeliveryVersionRequest true 	"body"
// @Success 200
// @Router /openapi/delivery/releases/helm [post]
func OpenAPICreateHelmDeliveryVersion(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Logger.Errorf("failed to generate authorization info for user: %s, error: %s", ctx.UserID, err)
		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	req := new(service.OpenAPICreateHelmDeliveryVersionRequest)
	if err := c.ShouldBindJSON(&req); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	req.CreateBy = ctx.UserName

	projectKey := req.ProjectName
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectKey].Version.Create {
			ctx.UnAuthorized = true
			return
		}
	}

	err = commonutil.CheckZadigProfessionalLicense()
	if err != nil {
		ctx.Err = err
		return
	}

	ctx.Err = service.OpenAPICreateHelmDeliveryVersion(req)
}
