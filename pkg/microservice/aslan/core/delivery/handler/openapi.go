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
	ProjectKey string `form:"projectKey" binding:"required"`
	PageNum    int    `form:"pageNum" binding:"required"`
	PageSize   int    `form:"pageSize" binding:"required"`
}

// @Summary 列出版本
// @Description 列出版本
// @Tags 	OpenAPI
// @Accept 	json
// @Produce json
// @Param 	projectKey		query		string							true	"project key"
// @Param 	pageNum 		query		int								true	"page num"
// @Param 	pageSize 		query		int								true	"page size"
// @Success 200             {object}    service.OpenAPIListDeliveryVersionV2Resp
// @Router /openapi/delivery/releases [get]
func OpenAPIListDeliveryVersion(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Logger.Errorf("failed to generate authorization info for user: %s, error: %s", ctx.UserID, err)
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	req := new(OpenAPIListDeliveryVersionRequest)
	if err := c.ShouldBindQuery(&req); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	projectKey := req.ProjectKey
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
		ctx.RespErr = err
		return
	}

	ctx.Resp, ctx.RespErr = service.OpenAPIListDeliveryVersion(req.ProjectKey, req.PageNum, req.PageSize)
}

// @Summary 获取版本详情
// @Description 获取版本详情
// @Tags 	OpenAPI
// @Accept 	json
// @Produce json
// @Param 	projectKey		query		string								true	"项目标识"
// @Param 	versionName		query		string								true	"版本名称"
// @Success 200             {object}    service.OpenAPIGetDeliveryVersionV2Resp
// @Router /openapi/delivery/releases/detail [get]
func OpenAPIGetDeliveryVersion(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Logger.Errorf("failed to generate authorization info for user: %s, error: %s", ctx.UserID, err)
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	//params validate
	versionName := c.Query("versionName")
	if versionName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("versionName can't be empty!")
		return
	}
	projectKey := c.Query("projectKey")
	if projectKey == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("projectKey can't be empty!")
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
		ctx.RespErr = err
		return
	}

	ctx.Resp, ctx.RespErr = service.OpenAPIGetDeliveryVersion(projectKey, versionName)
}

// @Summary 删除版本
// @Description 删除版本
// @Tags 	OpenAPI
// @Accept 	json
// @Produce json
// @Param 	projectKey		query		string								true	"project key"
// @Success 200
// @Router /openapi/delivery/releases/{id} [delete]
func OpenAPIDeleteDeliveryVersion(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Logger.Errorf("failed to generate authorization info for user: %s, error: %s", ctx.UserID, err)
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	//params validate
	ID := c.Param("id")
	if ID == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("id can't be empty!")
		return
	}
	projectKey := c.Query("projectKey")
	if projectKey == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("projectKey can't be empty!")
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
		ctx.RespErr = err
		return
	}

	ctx.RespErr = service.OpenAPIDeleteDeliveryVersion(c.Param("id"))
}

// @Summary 创建K8S版本
// @Description 创建K8S版本
// @Tags 	OpenAPI
// @Accept 	json
// @Produce json
// @Param 	body 			body   service.OpenAPICreateK8SDeliveryVersionV2Request  true 	"body"
// @Success 200             id     string
// @Router /openapi/delivery/releases/k8s [post]
func OpenAPICreateK8SDeliveryVersion(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Logger.Errorf("failed to generate authorization info for user: %s, error: %s", ctx.UserID, err)
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	req := new(service.OpenAPICreateK8SDeliveryVersionV2Request)
	if err := c.ShouldBindJSON(&req); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	req.CreateBy = ctx.UserName + "(OpenAPI)"

	projectKey := req.ProjectKey
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
		ctx.RespErr = err
		return
	}

	ctx.RespErr = service.OpenAPICreateK8SDeliveryVersion(req)
}

// @Summary 创建Helm版本
// @Description 创建Helm版本
// @Tags 	OpenAPI
// @Accept 	json
// @Produce json
// @Param 	body 			body 		service.OpenAPICreateHelmDeliveryVersionV2Request true 	"body"
// @Success 200
// @Router /openapi/delivery/releases/helm [post]
func OpenAPICreateHelmDeliveryVersion(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Logger.Errorf("failed to generate authorization info for user: %s, error: %s", ctx.UserID, err)
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	req := new(service.OpenAPICreateHelmDeliveryVersionV2Request)
	if err := c.ShouldBindJSON(&req); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	req.CreateBy = ctx.UserName + "(OpenAPI)"

	projectKey := req.ProjectKey
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
		ctx.RespErr = err
		return
	}

	ctx.RespErr = service.OpenAPICreateHelmDeliveryVersion(req)
}

// @Summary 重试创建版本
// @Description 重试创建版本
// @Tags 	OpenAPI
// @Accept 	json
// @Produce json
// @Param 	id			query		string							true	"id"
// @Success 200
// @Router /openapi/delivery/releases/retry [post]
func OpenAPIRetryCreateDeliveryVersion(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Logger.Errorf("failed to generate authorization info for user: %s, error: %s", ctx.UserID, err)
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	id := c.Query("id")
	if id == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("id can't be empty!")
		return
	}

	projectKey := c.Query("projectKey")
	if projectKey == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("projectName can't be empty!")
		return
	}

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
		ctx.RespErr = err
		return
	}

	ctx.RespErr = service.OpenAPIRetryCreateDeliveryVersion(id)
}
