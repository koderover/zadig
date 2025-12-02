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
	e "github.com/koderover/zadig/v2/pkg/tool/errors"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/stat/service"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
)

func CreateWeeklyRollbackStat(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.RespErr = service.CreateWeeklyRollbackStat(ctx.Logger)
}

// @Summary 获取回滚统计总数
// @Description
// @Tags 	stat
// @Accept 	json
// @Produce json
// @Param 	startDate		query		int								true	"开始时间，格式为时间戳"
// @Param 	endDate			query		int								true	"结束时间，格式为时间戳"
// @Param 	projects		query		[]string						true	"项目列表"
// @Param 	type			query		string							true	"环境类型，可选值为 production、testing、both"
// @Success 200 			{object} 	service.RollbackTotalStat
// @Router /api/aslan/stat/v2/quality/rollback/total [get]
func GetRollbackTotalStat(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(getStatGeneralReq)
	if err := c.ShouldBindQuery(args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}

	ctx.Resp, ctx.RespErr = service.GetRollbackTotalStat(args.StartTime, args.EndTime, args.Projects, args.ProductionType, ctx.Logger)
}

// @Summary 获取回滚次数TopN的服务
// @Description
// @Tags 	stat
// @Accept 	json
// @Produce json
// @Param 	startDate		query		int								true	"开始时间，格式为时间戳"
// @Param 	endDate			query		int								true	"结束时间，格式为时间戳"
// @Param 	top				query		int								true	"TopN"
// @Param 	projects		query		[]string						true	"项目列表"
// @Param 	type			query		string							true	"环境类型，可选值为 production、testing、both"
// @Success 200 			{array} 	mongodb.RollbackServiceCount
// @Router /api/aslan/stat/v2/quality/rollback/service/top [get]
func GetTopRollbackedService(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(getStateReqWithTop)
	if err := c.ShouldBindQuery(args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}

	ctx.Resp, ctx.RespErr = service.GetTopRollbackedProject(args.StartTime, args.EndTime, args.Top, args.ProductionType, args.Projects, ctx.Logger)
}

// @Summary 获取回滚周趋势
// @Description
// @Tags 	stat
// @Accept 	json
// @Produce json
// @Param 	startDate		query		int								true	"开始时间，格式为时间戳"
// @Param 	endDate			query		int								true	"结束时间，格式为时间戳"
// @Param 	projects		query		[]string						true	"项目列表"
// @Param 	type			query		string							true	"环境类型，可选值为 production、testing、both"
// @Success 200 			{array} 	models.WeeklyRollbackStat
// @Router /api/aslan/stat/v2/quality/rollback/trend/weekly [get]
func GetRollbackWeeklyTrend(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(getStatGeneralReq)
	if err := c.ShouldBindQuery(args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}

	ctx.Resp, ctx.RespErr = service.GetRollbackWeeklyTrend(args.StartTime, args.EndTime, args.Projects, args.ProductionType, ctx.Logger)
}

// @Summary 获取回滚数据详情
// @Description
// @Tags 	stat
// @Accept 	json
// @Produce json
// @Param 	startDate		query		int								true	"开始时间，格式为时间戳"
// @Param 	endDate			query		int								true	"结束时间，格式为时间戳"
// @Param 	projects		query		[]string						true	"项目列表"
// @Param 	type			query		string							true	"环境类型，可选值为 production、testing、both"
// @Success 200 			{object} 	service.GetRollbackStatResponse
// @Router /api/aslan/stat/v2/quality/rollback/stat [get]
func GetRollbackStat(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(getStatGeneralReq)
	if err := c.ShouldBindQuery(args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}

	ctx.Resp, ctx.RespErr = service.GetRollbackStat(args.StartTime, args.EndTime, args.ProductionType, args.Projects, ctx.Logger)
}
