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
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"

	models2 "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/system/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/system/service"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

// GetOperationLogOptions lists the filter values present in audit logs.
func GetOperationLogOptions(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}
	if !ctx.Resources.IsSystemAdmin && !ctx.Resources.SystemActions.LogOperation.View {
		ctx.UnAuthorized = true
		return
	}
	ctx.Resp, ctx.RespErr = service.ListOperationLogOptions(c.Request.Context(), ctx.Logger)
}

func GetOperationLogs(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin && !ctx.Resources.SystemActions.LogOperation.View {
		ctx.UnAuthorized = true
		return
	}

	var query struct {
		Username    string `form:"username"`
		ProductName string `form:"projectName"`
		Method      string `form:"method"`
		Function    string `form:"function"`
		Status      int    `form:"status" binding:"gte=0"`
		PerPage     int    `form:"per_page,default=50" binding:"gte=0"`
		Page        int    `form:"page,default=1" binding:"gte=0"`
		StartTime   int64  `form:"start_time" binding:"gte=0"`
		EndTime     int64  `form:"end_time" binding:"gte=0"`
		Detail      string `form:"detail"`
	}
	if err := c.ShouldBindQuery(&query); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}
	if query.StartTime > 0 && query.EndTime > 0 && query.StartTime >= query.EndTime {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("start_time must be less than end_time")
		return
	}
	if query.PerPage == 0 {
		query.PerPage = 50
	}
	if query.Page == 0 {
		query.Page = 1
	}
	args := &service.OperationLogArgs{
		Username:    query.Username,
		ProductName: query.ProductName,
		Method:      query.Method,
		Function:    query.Function,
		Status:      query.Status,
		PerPage:     query.PerPage,
		Page:        query.Page,
		StartTime:   query.StartTime,
		EndTime:     query.EndTime,
		Detail:      query.Detail,
	}

	resp, count, err := service.FindOperation(args, ctx.Logger)
	ctx.Resp = resp
	ctx.RespErr = err
	c.Writer.Header().Set("X-Total", strconv.Itoa(count))
}

func AddSystemOperationLog(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := new(models2.OperationLog)
	err = c.BindJSON(args)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid insertOperationLogs args")
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		ctx.UnAuthorized = true
		return
	}

	ctx.Resp, ctx.RespErr = service.InsertOperation(args, ctx.Logger)
}

type updateOperationArgs struct {
	Status int `json:"status"`
}

func UpdateOperationLog(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := new(updateOperationArgs)
	err = c.BindJSON(args)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid insertOperationLogs args")
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		ctx.UnAuthorized = true
		return
	}

	ctx.RespErr = service.UpdateOperation(c.Param("id"), args.Status, ctx.Logger)
}
