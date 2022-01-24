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
	"github.com/gin-gonic/gin"
)

type Router struct{}

func (*Router) Inject(router *gin.RouterGroup) {
	dashboard := router.Group("dashboard")
	{
		dashboard.GET("/overview", GetOverviewStat)
		dashboard.GET("/build", GetBuildStat)
		dashboard.GET("/deploy", GetDeployStat)
		dashboard.GET("/test", GetTestDashboard)
	}

	quality := router.Group("quality")
	{
		//buildquality
		quality.GET("/initBuildquality", GetAllPipelineTask)
		quality.GET("/buildAverageMeasure", GetBuildDailyAverageMeasure)
		quality.GET("/buildDailyMeasure", GetBuildDailyMeasure)
		quality.GET("/buildHealthMeasure", GetBuildHealthMeasure)
		quality.GET("/buildLatestTenMeasure", GetLatestTenBuildMeasure)
		quality.GET("/buildTenDurationMeasure", GetTenDurationMeasure)
		quality.GET("/buildTrend", GetBuildTrendMeasure)
		//testquality
		quality.GET("/initTestquality", InitTestStat)
		quality.GET("/testAverageMeasure", GetTestAverageMeasure)
		quality.GET("/testCaseMeasure", GetTestCaseMeasure)
		quality.GET("/testDeliveryDeploy", GetTestDeliveryDeployMeasure)
		quality.GET("/testHealthMeasure", GetTestHealthMeasure)
		quality.GET("/testTrend", GetTestTrendMeasure)
		//deployquality
		quality.GET("/initDeployquality", InitDeployStat)
		quality.GET("/pipelineHealthMeasure", GetPipelineHealthMeasure)
		quality.GET("/deployHealthMeasure", GetDeployHealthMeasure)
		quality.GET("/deployWeeklyMeasure", GetDeployWeeklyMeasure)
		quality.GET("/deployTopFiveHigherMeasure", GetDeployTopFiveHigherMeasure)
		quality.GET("/deployTopFiveFailureMeasure", GetDeployTopFiveFailureMeasure)
	}
}
