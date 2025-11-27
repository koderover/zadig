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
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/stat/service"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

type GetTestCountResp struct {
	Count int `json:"count"`
}

func GetTestCount(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	// Get projects from query parameters
	projects := c.QueryArray("projects")

	resp, err := service.GetTestCount(projects, ctx.Logger)
	if err != nil {
		ctx.RespErr = e.ErrGetTestCount.AddErr(err)
		return
	}

	ctx.Resp = &GetTestCountResp{
		Count: resp,
	}
}

type GetDailyTestHealthTrendReq struct {
	StartTime int64    `json:"start_time"`
	EndTime   int64    `json:"end_time"`
	Projects  []string `json:"projects"`
}

func GetDailyTestHealthTrend(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	//params validate
	args := new(GetDailyTestHealthTrendReq)
	if err := c.BindJSON(args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}

	ctx.Resp, ctx.RespErr = service.GetDailyTestHealthTrend(args.StartTime, args.EndTime, args.Projects, ctx.Logger)
}

func GetRecentTestTask(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	// Get projects from query parameters
	projects := c.QueryArray("projects")

	// Get number from query parameter, default to 10
	number := 10
	if numStr := c.Query("number"); numStr != "" {
		if num, err := strconv.Atoi(numStr); err == nil && num > 0 {
			number = num
		}
	}

	ctx.Resp, ctx.RespErr = service.GetRecentTestTask(projects, number, ctx.Logger)
}
