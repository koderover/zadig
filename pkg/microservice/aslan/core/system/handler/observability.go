/*
 * Copyright 2023 The KodeRover Authors.
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

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/system/service"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

func ListObservability(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = service.ListObservability(c.Query("type"), false)
}

func ListObservabilityDetail(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = service.ListObservability(c.Query("type"), true)
}

func CreateObservability(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	var args commonmodels.Observability
	if err := c.ShouldBindJSON(&args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}

	err := commonutil.CheckZadigXLicenseStatus()
	if err != nil {
		ctx.Err = err
		return
	}

	ctx.Err = service.CreateObservability(&args)
}

func UpdateObservability(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	var args commonmodels.Observability
	if err := c.ShouldBindJSON(&args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}

	err := commonutil.CheckZadigXLicenseStatus()
	if err != nil {
		ctx.Err = err
		return
	}

	ctx.Err = service.UpdateObservability(c.Param("id"), &args)
}

func DeleteObservability(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Err = service.DeleteObservability(c.Param("id"))
}

func ValidateObservability(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	var args commonmodels.Observability
	if err := c.ShouldBindJSON(&args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}
	ctx.Err = service.ValidateObservability(&args)
}
