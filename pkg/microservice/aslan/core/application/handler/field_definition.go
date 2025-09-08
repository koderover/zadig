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
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/application/service"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

func CreateFieldDefinition(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}
	args := new(commonmodels.ApplicationFieldDefinition)
	data, _ := c.GetRawData()
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))
	if err := c.ShouldBindWith(&args, binding.JSON); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid field definition args")
		return
	}
	ctx.Resp, ctx.RespErr = service.CreateFieldDefinition(args, ctx.Logger)
}

func ListFieldDefinitions(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	ctx.Resp, ctx.RespErr = service.ListFieldDefinitions(ctx.Logger)
}

func UpdateFieldDefinition(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}
	args := new(commonmodels.ApplicationFieldDefinition)
	data, _ := c.GetRawData()
	if err := json.Unmarshal(data, args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid field definition args")
		return
	}
	ctx.RespErr = service.UpdateFieldDefinition(c.Param("id"), args, ctx.Logger)
}

func DeleteFieldDefinition(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}
	ctx.RespErr = service.DeleteFieldDefinition(c.Param("id"), ctx.Logger)
}
