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
	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/stat/service"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

func CreateStatDashboardConfig(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	//params validate
	args := new(service.StatDashboardConfig)
	if err := c.BindJSON(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}

	ctx.Err = service.CreateStatDashboardConfig(args, ctx.Logger)
}

type listStatDashboardConfigResp struct {
	Configs []*service.StatDashboardConfig `json:"configs"`
}

func ListStatDashboardConfigs(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	resp, err := service.ListDashboardConfigs(ctx.Logger)

	ctx.Resp = listStatDashboardConfigResp{resp}
	ctx.Err = err
}

func UpdateStatDashboardConfig(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	//params validate
	args := new(service.StatDashboardConfig)
	if err := c.BindJSON(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}

	ctx.Err = service.UpdateStatDashboardConfig(c.Param("id"), args, ctx.Logger)
}

func DeleteStatDashboardConfig(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Err = service.DeleteStatDashboardConfig(c.Param("id"), ctx.Logger)
}

type getStatDashboardReq struct {
	StartTime int64 `form:"start_time"`
	EndTime   int64 `form:"end_time"`
}

type getStatDashboardResp struct {
	Dashboard []*service.StatDashboardByProject `json:"dashboard"`
}

func GetStatsDashboard(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(getStatDashboardReq)
	if err := c.ShouldBindQuery(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}

	resp, err := service.GetStatsDashboard(args.StartTime, args.EndTime, ctx.Logger)

	ctx.Resp = getStatDashboardResp{resp}
	ctx.Err = err
}

func GetStatsDashboardGeneralData(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(getStatDashboardReq)
	if err := c.ShouldBindQuery(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}

	ctx.Resp, ctx.Err = service.GetStatsDashboardGeneralData(args.StartTime, args.EndTime, ctx.Logger)
}
