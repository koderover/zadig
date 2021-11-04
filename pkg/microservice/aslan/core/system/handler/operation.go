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
	"strconv"

	"github.com/gin-gonic/gin"

	models2 "github.com/koderover/zadig/pkg/microservice/aslan/core/system/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/system/service"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

func GetOperationLogs(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	status, err := strconv.Atoi(c.Query("status"))
	if err != nil {
		ctx.Err = e.ErrFindOperationLog.AddErr(err)
		return
	}

	perPage, err := strconv.Atoi(c.Query("per_page"))
	if err != nil {
		ctx.Err = e.ErrFindOperationLog.AddErr(err)
		return
	}

	page, err := strconv.Atoi(c.Query("page"))
	if err != nil {
		ctx.Err = e.ErrFindOperationLog.AddErr(err)
		return
	}

	args := &service.OperationLogArgs{
		Username:    c.Query("username"),
		ProductName: c.Query("product_name"),
		Function:    c.Query("function"),
		Status:      status,
		PerPage:     perPage,
		Page:        page,
	}

	if args.PerPage == 0 {
		args.PerPage = 50
	}

	if args.Page == 0 {
		args.Page = 1
	}

	resp, count, err := service.FindOperation(args, ctx.Logger)
	ctx.Resp = resp
	ctx.Err = err
	c.Writer.Header().Set("X-Total", strconv.Itoa(count))
}

func AddSystemOperationLog(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(models2.OperationLog)
	err := c.BindJSON(args)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid insertOperationLogs args")
		return
	}
	ctx.Resp, ctx.Err = service.InsertOperation(args, ctx.Logger)
}

type updateOperationArgs struct {
	Status int `json:"status"`
}

func UpdateOperationLog(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(updateOperationArgs)
	err := c.BindJSON(args)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid insertOperationLogs args")
		return
	}
	ctx.Err = service.UpdateOperation(c.Param("id"), args.Status, ctx.Logger)
}
