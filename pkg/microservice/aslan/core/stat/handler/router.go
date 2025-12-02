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
		dashboard.GET("/release", GetReleaseDashboard)
	}

	// Deprecated: this whole Group is deprecated and will be replaced by v2 api
	quality := router.Group("quality")
	{
		//buildStat
		quality.POST("/initBuildStat", GetAllPipelineTask)
		quality.POST("/buildAverageMeasure", GetBuildDailyAverageMeasure)
		quality.POST("/buildDailyMeasure", GetBuildDailyMeasure)
		quality.POST("/buildHealthMeasure", GetBuildHealthMeasure)
		quality.POST("/buildLatestTenMeasure", GetLatestTenBuildMeasure)
		quality.POST("/buildTenDurationMeasure", GetTenDurationMeasure)
		quality.POST("/buildTrend", GetBuildTrendMeasure)
		//testStat
		quality.POST("/initTestStat", InitTestStat)
		quality.POST("/testAverageMeasure", GetTestAverageMeasure)
		quality.POST("/testCaseMeasure", GetTestCaseMeasure)
		quality.POST("/testDeliveryDeploy", GetTestDeliveryDeployMeasure)
		quality.POST("/testHealthMeasure", GetTestHealthMeasure)
		quality.POST("/testTrend", GetTestTrendMeasure)
		//deployStat
		quality.POST("/initDeployStat", InitDeployStat)
		quality.POST("/pipelineHealthMeasure", GetPipelineHealthMeasure)
		quality.POST("/deployHealthMeasure", GetDeployHealthMeasure)
		quality.POST("/deployWeeklyMeasure", GetDeployWeeklyMeasure)
		quality.POST("/deployTopFiveHigherMeasure", GetDeployTopFiveHigherMeasure)
		quality.POST("/deployTopFiveFailureMeasure", GetDeployTopFiveFailureMeasure)
	}

	// v2 api below
	v2 := router.Group("v2")

	dashboardConfigV2 := v2.Group("config")
	{
		dashboardConfigV2.GET("", ListStatDashboardConfigs)
		dashboardConfigV2.POST("", CreateStatDashboardConfig)
		dashboardConfigV2.PUT("/:id", UpdateStatDashboardConfig)
		dashboardConfigV2.DELETE("/:id", DeleteStatDashboardConfig)
	}

	dashboardV2 := v2.Group("dashboard")
	{
		dashboardV2.GET("", GetStatsDashboard)
		dashboardV2.GET("/general", GetStatsDashboardGeneralData)
	}

	aiV2 := v2.Group("ai")
	{
		aiV2.POST("/analysis", GetAIStatsAnalysis)
		aiV2.GET("/analysis/prompt", GetAIStatsAnalysisPrompts)
		aiV2.GET("/overview", GetProjectsOverview)
		aiV2.GET("/build/trend", GetCurrently30DayBuildTrend)
		aiV2.GET("/radar", GetEfficiencyRadar)
		aiV2.GET("/attention", GetMonthAttention)
		aiV2.GET("/requirement/period", GetRequirementDevDepPeriod)
	}

	releaseV2 := v2.Group("release")
	{
		releaseV2.POST("/monthly", CreateMonthlyReleaseStat)
	}

	qualityV2 := v2.Group("quality")

	deployV2 := qualityV2.Group("deploy")
	{
		deployV2.POST("/weekly", CreateWeeklyDeployStat)
		deployV2.POST("/monthly", CreateMonthlyDeployStat)
		deployV2.GET("/health", GetDeployHeathStats)
		deployV2.GET("/trend/weekly", GetDeployWeeklyTrend)
		deployV2.GET("/trend/monthly", GetDeployMonthlyTrend)
		deployV2.GET("/service/top", GetTopDeployedService)
		deployV2.GET("/service/failure", GetTopDeployFailuresByService)
	}

	rollbackV2 := qualityV2.Group("rollback")
	{
		rollbackV2.POST("/weekly", CreateWeeklyRollbackStat)
		rollbackV2.GET("/total", GetRollbackTotalStat)
		rollbackV2.GET("/service/top", GetTopRollbackedService)
		rollbackV2.GET("/trend/weekly", GetRollbackWeeklyTrend)
		rollbackV2.GET("/stat", GetRollbackStat)
	}

	testV2 := qualityV2.Group("test")
	{
		testV2.GET("/count", GetTestCount)
		testV2.POST("/dailyHealthTrend", GetDailyTestHealthTrend)
		testV2.GET("/recentTask", GetRecentTestTask)
	}

}

type OpenAPIRouter struct{}

func (*OpenAPIRouter) Inject(router *gin.RouterGroup) {
	dashboard := router.Group("")
	{
		dashboard.GET("/overview", GetOverviewStat)
		dashboard.GET("/build", GetBuildStatForOpenAPI)
		dashboard.GET("/deploy", GetDeployStatsOpenAPI)
		dashboard.GET("/test", GetTestStatOpenAPI)
	}

	// enterprise statistics OpenAPI
	v2 := router.Group("/v2")
	{
		v2.GET("/release", GetReleaseStatOpenAPI)
		v2.GET("/rollback/detail", GetRollbackStatDetailOpenAPI)
	}
}
