/*
Copyright 2021 The KodeRover Authors.

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
	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/pkg/microservice/systemconfig/core/email/repository/models"
	"github.com/koderover/zadig/pkg/microservice/systemconfig/core/email/service"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
)

func GetEmailHost(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	ctx.Resp, ctx.Err = service.GetEmailHost(ctx.Logger)
}

func InternalGetEmailHost(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	ctx.Resp, ctx.Err = service.InternalGetEmailHost(ctx.Logger)
}

func CreateEmailHost(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	req := new(models.EmailHost)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.Err = err
		return
	}
	ctx.Resp, ctx.Err = service.CreateEmailHost(req, ctx.Logger)
}

func UpdateEmailHost(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	req := new(models.EmailHost)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.Err = err
		return
	}
	ctx.Resp, ctx.Err = service.UpdateEmailHost(req, ctx.Logger)
}

func DeleteEmailHost(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	ctx.Err = service.DeleteEmailHost(ctx.Logger)
}

func GetEmailService(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	ctx.Resp, ctx.Err = service.GetEmailService(ctx.Logger)
}

func CreateEmailService(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	req := new(models.EmailService)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.Err = err
		return
	}
	ctx.Resp, ctx.Err = service.CreateEmailService(req, ctx.Logger)
}

func UpdateEmailService(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	req := new(models.EmailService)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.Err = err
		return
	}
	ctx.Resp, ctx.Err = service.UpdateEmailService(req, ctx.Logger)
}

func DeleteEmailService(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	ctx.Err = service.DeleteEmailService(ctx.Logger)
}
