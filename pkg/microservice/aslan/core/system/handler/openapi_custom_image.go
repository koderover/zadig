/*
Copyright 2026 The KodeRover Authors.

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
	"strings"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/system/service"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/types"
)

func OpenAPIListCustomImages(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}
	if !ctx.Resources.IsSystemAdmin {
		ctx.UnAuthorized = true
		return
	}

	ctx.Resp, ctx.RespErr = service.ListCustomImagesOpenAPI(ctx.Logger)
}

func OpenAPIGetCustomImage(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}
	if !ctx.Resources.IsSystemAdmin {
		ctx.UnAuthorized = true
		return
	}
	if !primitive.IsValidObjectID(c.Param("id")) {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid custom image id")
		return
	}

	ctx.Resp, ctx.RespErr = service.GetCustomImageOpenAPI(c.Param("id"), ctx.Logger)
}

func OpenAPICreateCustomImage(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}
	if !ctx.Resources.IsSystemAdmin {
		ctx.UnAuthorized = true
		return
	}

	data, err := internalhandler.GetRawData(c)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid custom image request body")
		return
	}
	req := new(service.OpenAPICustomImageReq)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid custom image request body")
		return
	}
	if err := req.Validate(); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}
	internalhandler.InsertOperationLog(c, ctx.UserName+"(openAPI)", "", "新增", "基础镜像", req.Label, req.Label, string(data), types.RequestBodyTypeJSON, ctx.Logger)
	ctx.RespErr = service.CreateBasicImage(&commonmodels.BasicImage{
		Label:     strings.TrimSpace(req.Label),
		Value:     strings.TrimSpace(req.Value),
		ImageFrom: commonmodels.ImageFromCustom,
		UpdateBy:  ctx.UserName,
	}, ctx.Logger)
}

func OpenAPIUpdateCustomImage(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}
	if !ctx.Resources.IsSystemAdmin {
		ctx.UnAuthorized = true
		return
	}
	if !primitive.IsValidObjectID(c.Param("id")) {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid custom image id")
		return
	}

	data, err := internalhandler.GetRawData(c)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid custom image request body")
		return
	}
	req := new(service.OpenAPICustomImageReq)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid custom image request body")
		return
	}
	internalhandler.InsertOperationLog(c, ctx.UserName+"(openAPI)", "", "更新", "基础镜像", c.Param("id"), c.Param("id"), string(data), types.RequestBodyTypeJSON, ctx.Logger)
	ctx.RespErr = service.UpdateCustomImageOpenAPI(c.Param("id"), req, ctx.UserName, ctx.Logger)
}

func OpenAPIDeleteCustomImage(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}
	if !ctx.Resources.IsSystemAdmin {
		ctx.UnAuthorized = true
		return
	}
	if !primitive.IsValidObjectID(c.Param("id")) {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid custom image id")
		return
	}

	internalhandler.InsertOperationLog(c, ctx.UserName+"(openAPI)", "", "删除", "基础镜像", c.Param("id"), c.Param("id"), "", types.RequestBodyTypeJSON, ctx.Logger)
	ctx.RespErr = service.DeleteCustomImageOpenAPI(c.Param("id"), ctx.Logger)
}
