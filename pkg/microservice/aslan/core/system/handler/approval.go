/*
 * Copyright 2022 The KodeRover Authors.
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
	"github.com/gin-gonic/gin"

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/system/service"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

func ListIMApp(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = service.ListIMApp(c.Query("type"), ctx.Logger)
}

func CreateIMApp(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	var args commonmodels.IMApp
	if err := c.ShouldBindJSON(&args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}
	oid, err := service.CreateIMApp(&args, ctx.Logger)
	if err != nil {
		ctx.Err = err
		return
	}
	ctx.Resp = map[string]string{"id": oid}
}

func UpdateIMApp(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	var args commonmodels.IMApp
	if err := c.ShouldBindJSON(&args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}
	ctx.Err = service.UpdateIMApp(c.Param("id"), &args, ctx.Logger)
}

func DeleteIMApp(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Err = service.DeleteIMApp(c.Param("id"), ctx.Logger)
}

func ValidateIMApp(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	var args commonmodels.IMApp
	if err := c.ShouldBindJSON(&args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}
	ctx.Err = service.ValidateIMApp(&args, ctx.Logger)
}
