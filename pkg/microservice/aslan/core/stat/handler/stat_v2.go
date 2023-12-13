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
	"encoding/json"
	"time"

	"github.com/gin-gonic/gin"

	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/stat/service"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/stat/service/ai"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
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

	err := commonutil.CheckZadigXLicenseStatus()
	if err != nil {
		ctx.Err = err
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

	err := commonutil.CheckZadigXLicenseStatus()
	if err != nil {
		ctx.Err = err
		return
	}

	ctx.Err = service.UpdateStatDashboardConfig(c.Param("id"), args, ctx.Logger)
}

func DeleteStatDashboardConfig(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	err := commonutil.CheckZadigXLicenseStatus()
	if err != nil {
		ctx.Err = err
		return
	}

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

	err := commonutil.CheckZadigXLicenseStatus()
	if err != nil {
		ctx.Err = err
		return
	}

	resp, err := service.GetStatsDashboard(args.StartTime, args.EndTime, nil, ctx.Logger)

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

	err := commonutil.CheckZadigXLicenseStatus()
	if err != nil {
		ctx.Err = err
		return
	}

	ctx.Resp, ctx.Err = service.GetStatsDashboardGeneralData(args.StartTime, args.EndTime, ctx.Logger)
}

func GetAIStatsAnalysis(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	// get params prompt, projectList, duration from query
	reqData, err := c.GetRawData()
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}
	args := new(ai.AiAnalysisReq)
	if err := json.Unmarshal(reqData, args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}

	ctx.Resp, ctx.Err = ai.AnalyzeProjectStats(args, ctx.Logger)
}

func GetAIStatsAnalysisPrompts(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = ai.GetAiPrompts(ctx.Logger)
}

func GetProjectsOverview(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(getStatDashboardReq)
	if err := c.ShouldBindQuery(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}

	if args.StartTime == 0 && args.EndTime == 0 {
		now := time.Now()
		args.StartTime = now.AddDate(0, -1, 0).Unix()
		args.EndTime = now.Unix()
	}

	ctx.Resp, ctx.Err = service.GetProjectsOverview(args.StartTime, args.EndTime, ctx.Logger)
}

type aiStatReq struct {
	StartTime int64    `form:"start_time,default=0"`
	EndTime   int64    `form:"end_time,default=0"`
	Projects  []string `json:"projects"`
}

func GetCurrently30DayBuildTrend(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(aiStatReq)
	if err := c.ShouldBindQuery(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}

	if args.StartTime == 0 && args.EndTime == 0 {
		now := time.Now()
		args.StartTime = now.AddDate(0, 0, -30).Unix()
		args.EndTime = now.Unix()
	}

	ctx.Resp, ctx.Err = service.GetCurrently30DayBuildTrend(args.StartTime, args.EndTime, args.Projects, ctx.Logger)
}

func GetEfficiencyRadar(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(aiStatReq)
	if err := c.ShouldBindQuery(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}
	if args.StartTime == 0 && args.EndTime == 0 {
		now := time.Now()
		args.StartTime = now.AddDate(0, 0, -30).Unix()
		args.EndTime = now.Unix()
	}

	ctx.Resp, ctx.Err = service.GetEfficiencyRadar(args.StartTime, args.EndTime, args.Projects, ctx.Logger)
}

type AIMonthAttentionResp struct {
	AIAnswer     *ai.AIAttentionResp       `json:"ai_answer"`
	SystemAnswer []*service.MonthAttention `json:"system_answer"`
}

func GetMonthAttention(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(aiStatReq)
	if err := c.ShouldBindQuery(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}
	if args.StartTime == 0 && args.EndTime == 0 {
		now := time.Now()
		args.StartTime = now.AddDate(0, -1, 0).Unix()
		args.EndTime = now.Unix()
	}

	data, err := service.GetMonthAttention(args.StartTime, args.EndTime, args.Projects, ctx.Logger)
	if err != nil {
		ctx.Err = err
		return
	}

	// AI analyze data to get the attention of the month
	resp, err := ai.AnalyzeMonthAttention(args.StartTime, args.EndTime, data, ctx.Logger)
	if err != nil {
		if err != ai.ReturnAnswerWrongFormat {
			ctx.Err = err
			return
		}
	}
	ctx.Resp = AIMonthAttentionResp{
		AIAnswer:     resp,
		SystemAnswer: data,
	}
}

func GetRequirementDevDepPeriod(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(aiStatReq)
	if err := c.ShouldBindQuery(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}
	if args.StartTime == 0 && args.EndTime == 0 {
		now := time.Now()
		args.StartTime = now.AddDate(0, -1, 0).Unix()
		args.EndTime = now.Unix()
	}

	ctx.Resp, ctx.Err = service.GetRequirementDevDelPeriod(args.StartTime, args.EndTime, args.Projects, ctx.Logger)
}
