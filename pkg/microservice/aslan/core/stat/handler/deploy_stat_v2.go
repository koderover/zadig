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
	"github.com/gin-gonic/gin"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/stat/service"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
)

func CreateWeeklyDeployStat(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.RespErr = service.CreateWeeklyDeployStat(ctx.Logger)
}

func CreateMonthlyDeployStat(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.RespErr = service.CreateMonthlyDeployStat(ctx.Logger)
}

type getStatGeneralReq struct {
	StartTime      int64                 `json:"startDate"  form:"startDate"`
	EndTime        int64                 `json:"endDate"    form:"endDate"`
	Projects       []string              `json:"projects"   form:"projects"`
	ProductionType config.ProductionType `json:"type"       form:"type"`
}

func GetDeployHeathStats(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(getStatGeneralReq)
	if err := c.ShouldBindQuery(args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}

	ctx.Resp, ctx.RespErr = service.GetDeployHeathStats(args.StartTime, args.EndTime, args.Projects, args.ProductionType, ctx.Logger)
}

func GetDeployWeeklyTrend(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(getStatGeneralReq)
	if err := c.ShouldBindQuery(args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}

	ctx.Resp, ctx.RespErr = service.GetDeployWeeklyTrend(args.StartTime, args.EndTime, args.Projects, args.ProductionType, ctx.Logger)
}

func GetDeployMonthlyTrend(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(getStatGeneralReq)
	if err := c.ShouldBindQuery(args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}

	ctx.Resp, ctx.RespErr = service.GetDeployMonthlyTrend(args.StartTime, args.EndTime, args.Projects, args.ProductionType, ctx.Logger)
}

type getStateReqWithTop struct {
	getStatGeneralReq `form:",inline"`

	Top int `json:"top" form:"top"`
}

func GetTopDeployedService(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(getStateReqWithTop)
	if err := c.ShouldBindQuery(args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}

	ctx.Resp, ctx.RespErr = service.GetTopDeployedService(args.StartTime, args.EndTime, args.Projects, args.Top, args.ProductionType, ctx.Logger)
}

func GetTopDeployFailuresByService(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(getStateReqWithTop)
	if err := c.ShouldBindQuery(args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}

	ctx.Resp, ctx.RespErr = service.GetTopDeployFailuresByService(args.StartTime, args.EndTime, args.Projects, args.Top, args.ProductionType, ctx.Logger)
}
