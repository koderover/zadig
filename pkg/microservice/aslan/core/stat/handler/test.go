/*
Copyright 2022 The KodeRover Authors.

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
	"time"

	"github.com/gin-gonic/gin"
	e "github.com/koderover/zadig/pkg/tool/errors"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/stat/service"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
)

func GetTestDashboard(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = service.GetTestDashboard(0, 0, ctx.Logger)
}

type DailyTestStat struct {
	Date         string `json:"date"`
	SuccessCount int    `json:"success_count"`
	TimoutCount  int    `json:"timeout_count"`
	FailCount    int    `json:"failed_count"`
}

type OpenAPITestStatResp struct {
	CaseCount      int              `json:"case_count"`
	ExecCount      int              `json:"exec_count"`
	SuccessCount   int              `json:"success_count"`
	AverageRuntime int64            `json:"average_runtime"`
	DailyTestData  []*DailyTestStat `json:"data"`
}

func GetTestStatOpenAPI(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(getStatReq)
	if err := c.ShouldBindQuery(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	testStatistics, err := service.GetTestDashboard(args.StartDate, args.EndDate, ctx.Logger)
	if err != nil {
		ctx.Err = err
		return
	}

	testDailyInfo, err := service.GetTestTrendMeasure(args.StartDate, args.EndDate, []string{}, ctx.Logger)
	if err != nil {
		ctx.Err = err
		return
	}

	dailyTestInfo := make([]*DailyTestStat, 0)

	for _, dailyTestStat := range testDailyInfo.Sum {
		dailyTestInfo = append(dailyTestInfo, &DailyTestStat{
			Date:         time.Unix(dailyTestStat.Day, 0).Format("2006-01-02"),
			SuccessCount: dailyTestStat.Success,
			TimoutCount:  dailyTestStat.Timeout,
			FailCount:    dailyTestStat.Failure,
		})
	}

	resp := &OpenAPITestStatResp{
		CaseCount:      testStatistics.TotalCaseCount,
		ExecCount:      testStatistics.TotalExecCount,
		SuccessCount:   testStatistics.Success,
		AverageRuntime: testStatistics.AverageDuration,
		DailyTestData:  dailyTestInfo,
	}

	ctx.Resp = resp
}
