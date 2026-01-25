/*
Copyright 2023 The KodeRover Authors.

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
	cloudservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/cloudservice"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/setting"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

// @Summary 列出云服务
// @Description 列出云服务
// @Tags 	system
// @Accept 	json
// @Produce json
// @Param 	encryptedKey		query		string				true	"encrypted key"
// @Success 200 	{array} 	models.CloudService
// @Router /api/aslan/system/cloud_service [get]
func ListCloudService(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	encryptedKey := c.Query("encryptedKey")
	if len(encryptedKey) == 0 {
		ctx.RespErr = e.ErrInvalidParam
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		ctx.UnAuthorized = true
		return
	}

	ctx.Resp, ctx.RespErr = cloudservice.ListCloudService(ctx, encryptedKey)
}

// @Summary 列出云服务详情
// @Description 列出云服务详情
// @Tags 	system
// @Accept 	json
// @Produce json
// @Success 200 	{array} 	models.CloudService
// @Router /api/aslan/system/cloud_service/detail [get]
func ListCloudServiceInfo(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	ctx.Resp, ctx.RespErr = cloudservice.ListCloudServiceInfo(ctx)
}

// @Summary 创建云服务
// @Description 创建云服务
// @Tags 	system
// @Accept 	json
// @Produce json
// @Param 	body 	body 		models.CloudService			true 	"body"
// @Success 200
// @Router /api/aslan/system/cloud_service [post]
func CreateCloudService(c *gin.Context) {
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

	args := new(commonmodels.CloudService)
	if err := c.BindJSON(args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid sae json args")
		return
	}

	if args.Type == setting.CloudServiceTypeSAE {
		err = commonutil.CheckZadigLicenseFeatureSae()
		if err != nil {
			ctx.RespErr = err
			return
		}
	}

	args.UpdateBy = ctx.UserName
	ctx.RespErr = cloudservice.CreateCloudService(ctx, args)
}

// @Summary 获取云服务
// @Description 获取云服务
// @Tags 	system
// @Accept 	json
// @Produce json
// @Success 200 	{object} 	models.CloudService
// @Router /api/aslan/system/cloud_service/{id} [get]
func GetCloudService(c *gin.Context) {
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

	id := c.Param("id")
	if len(id) == 0 {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid id")
		return
	}

	ctx.Resp, ctx.RespErr = cloudservice.FindCloudService(ctx, id)
}

// @Summary 更新云服务
// @Description 更新云服务
// @Tags 	system
// @Accept 	json
// @Produce json
// @Param 	id		path		string					true	"云服务 ID"
// @Param 	body 	body 		models.CloudService			true 	"body"
// @Success 200
// @Router /api/aslan/system/cloud_service/{id} [put]
func UpdateCloudService(c *gin.Context) {
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

	id := c.Param("id")
	if len(id) == 0 {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid id")
		return
	}

	args := new(commonmodels.CloudService)
	if err := c.BindJSON(args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid sae json args")
		return
	}
	args.UpdateBy = ctx.UserName

	if args.Type == setting.CloudServiceTypeSAE {
		err = commonutil.CheckZadigLicenseFeatureSae()
		if err != nil {
			ctx.RespErr = err
			return
		}
	}

	ctx.RespErr = cloudservice.UpdateCloudService(ctx, id, args)
}

// @Summary 删除云服务
// @Description 删除云服务
// @Tags 	system
// @Accept 	json
// @Produce json
// @Param 	id		path		string				true	"云服务 id"
// @Success 200
// @Router /api/aslan/system/cloud_service/{id} [delete]
func DeleteCloudService(c *gin.Context) {
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

	id := c.Param("id")
	if len(id) == 0 {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid id")
		return
	}

	ctx.RespErr = cloudservice.DeleteCloudService(ctx, id)
}

// @Summary 验证云服务连接
// @Description
// @Tags 	system
// @Accept 	json
// @Produce json
// @Param 	body 	body 		models.CloudService			true 	"body"
// @Success 200
// @Router /api/aslan/system/cloud_service/validate [post]
func ValidateCloudService(c *gin.Context) {
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

	args := new(commonmodels.CloudService)
	if err := c.BindJSON(args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid sae json args")
		return
	}

	ctx.RespErr = cloudservice.ValidateCloudService(ctx, args)
}
