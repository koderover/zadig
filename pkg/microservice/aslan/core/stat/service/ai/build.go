package ai

import (
	"encoding/json"
	"fmt"
	"time"

	"go.uber.org/zap"

	service2 "github.com/koderover/zadig/pkg/microservice/aslan/core/stat/service"
)

type DataDetail struct {
	BuildInfo   *BuildData   `json:"build_info"`
	DeployInfo  *DeployData  `json:"deploy_info"`
	TestInfo    *TestData    `json:"test_info"`
	ReleaseInfo *ReleaseData `json:"release_info"`
}

type BuildData struct {
	Description string        `json:"data_description"`
	Details     *BuildDetails `json:"data_details"`
}

type BuildDetails struct {
	BuildTotal            int          `json:"build_total"`
	BuildSuccessTotal     int          `json:"build_success_total"`
	BuildFailureTotal     int          `json:"build_failure_total"`
	BuildTotalDuration    int64        `json:"build_total_duration"`
	BuildTrendData        *BuildDetail `json:"build_weekly_trend_data"`
	BuildDailyMeasureData *BuildDetail `json:"build_daily_measure_data"`
}

type BuildDetail struct {
	Description string `json:"data_description"`
	Details     string `json:"details"`
}

func getBuildData(project string, startTime, endTime int64, log *zap.SugaredLogger) (*BuildData, error) {
	log.Infof("=====> Start to get build data for project %s", project)
	build := &BuildData{
		Description: fmt.Sprintf("%s项目在%s到%s期间构建相关数据，包括构建总次数，构建成功次数，构建失败次数,构建周趋势数据，构建每日数据", project, time.Unix(startTime, 0).Format("2006-01-02"), time.Unix(endTime, 0).Format("2006-01-02")),
		Details:     &BuildDetails{},
	}
	// get build data from mongo
	buildJobList, err := service2.GetProjectBuildStat(startTime, endTime, project)
	if err != nil {
		return build, err
	}

	build.Details.BuildTotal = buildJobList.Total
	build.Details.BuildSuccessTotal = buildJobList.Success
	build.Details.BuildFailureTotal = buildJobList.Failure
	build.Details.BuildTotalDuration = int64(buildJobList.Duration)

	build.Details.BuildTrendData = &BuildDetail{}
	getBuildTrend(startTime, endTime, project, build.Details.BuildTrendData, log)

	// TODO: this data may be too large, need to be optimized
	// only get daily measure data when the time range is less than 1/2 year
	if endTime-startTime <= 86400*183 {
		build.Details.BuildDailyMeasureData = &BuildDetail{}
		getBuildDailyMeasure(startTime, endTime, project, build.Details.BuildDailyMeasureData, log)
	}

	return build, nil
}

func getBuildTrend(startTime, endTime int64, project string, detail *BuildDetail, log *zap.SugaredLogger) {
	// get build trend data
	buildTrendData, err := service2.GetWeeklyBuildStat(startTime, endTime, []string{project})
	if err != nil {
		log.Errorf("Failed to get build trend data, the error is: %+v", err)
	}
	trend, err := json.Marshal(buildTrendData)
	if err != nil {
		log.Errorf("Failed to marshal build trend data, the error is: %+v", err)
	}
	detail.Description = fmt.Sprintf("%s项目在%s到%s期间的构建趋势数据，统计一周内总的构建次数，构建成功次数，构建失败次数，构建平均耗时等数据", project, time.Unix(startTime, 0).Format("2006-01-02"), time.Unix(endTime, 0).Format("2006-01-02"))
	log.Infof("%s: \n%s", detail.Description, string(trend))
	detail.Details = string(trend)
}

func getBuildDailyMeasure(startTime, endTime int64, project string, detail *BuildDetail, log *zap.SugaredLogger) {
	// get build daily measure data
	buildDailyMeasureData, err := service2.GetDailyBuildMeasure(startTime, endTime, []string{project})
	if err != nil {
		log.Errorf("Failed to get build daily measure data, the error is: %+v", err)
	}
	daily, err := json.Marshal(buildDailyMeasureData)
	if err != nil {
		log.Errorf("Failed to marshal build daily measure data, the error is: %+v", err)
	}
	detail.Description = fmt.Sprintf("%s项目在%s到%s期间的构建相关数据，统计每日总次数，构建成功次数，构建失败次数，平均构建耗时", project, time.Unix(startTime, 0).Format("2006-01-02"), time.Unix(endTime, 0).Format("2006-01-02"))
	log.Infof("%s: \n%s", detail.Description, string(daily))
	detail.Details = string(daily)
}
