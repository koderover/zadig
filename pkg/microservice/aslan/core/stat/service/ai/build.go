package ai

import (
	"encoding/json"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	service2 "github.com/koderover/zadig/pkg/microservice/aslan/core/stat/service"
	"go.uber.org/zap"
)

type DataDetail struct {
	BuildInfo   *BuildData   `json:"build_info"`
	DeployInfo  *DeployData  `json:"deploy_info"`
	TestInfo    *TestData    `json:"test_info"`
	ReleaseInfo *ReleaseData `json:"release_info"`
}

type BuildData struct {
	Description string        `json:"description"`
	Details     *BuildDetails `json:"build_details"`
}

type BuildDetails struct {
	StatStartTime             int64        `json:"stat_start_time"`
	StatEndTime               int64        `json:"stat_end_time"`
	BuildTotal                int          `json:"build_total"`
	BuildSuccessTotal         int          `json:"build_success_total"`
	BuildFailureTotal         int          `json:"build_failure_total"`
	BuildTotalDuration        int64        `json:"build_total_duration"`
	BuildTrendData            *BuildDetail `json:"build_trend_data"`
	BuildHealthMeasureData    *BuildDetail `json:"build_health_measure_data"`
	BuildLatestTenMeasureData *BuildDetail `json:"build_latest_ten_measure_data"`
	BuildDailyMeasureData     *BuildDetail `json:"build_daily_measure_data"`
	BuildAverageMeasureData   *BuildDetail `json:"build_average_measure_data"`
}

type BuildDetail struct {
	Description string `json:"description"`
	Details     string `json:"details"`
}

func getBuildData(project string, startTime, endTime int64, log *zap.SugaredLogger) (*BuildData, error) {
	build := &BuildData{
		Description: "构建数据",
		Details:     &BuildDetails{},
	}
	// get build data from mongo
	buildJobList, err := commonrepo.NewJobInfoColl().GetBuildJobs(startTime, endTime, project)
	if err != nil {
		return build, err
	}
	totalCounter := len(buildJobList)
	if totalCounter == 0 {
		return build, err
	}
	passCounter := 0
	for _, job := range buildJobList {
		if job.Status == string(config.StatusPassed) {
			passCounter++
		}
	}
	var totalTimesTaken int64 = 0
	for _, job := range buildJobList {
		totalTimesTaken += job.Duration
	}

	build.Details.StatStartTime = startTime
	build.Details.StatEndTime = endTime
	build.Details.BuildTotal = totalCounter
	build.Details.BuildSuccessTotal = passCounter
	build.Details.BuildFailureTotal = totalCounter - passCounter
	build.Details.BuildTotalDuration = totalTimesTaken / int64(totalCounter)

	build.Details.BuildTrendData = &BuildDetail{}
	getBuildTrend(startTime, endTime, project, build.Details.BuildTrendData, log)

	build.Details.BuildHealthMeasureData = &BuildDetail{}
	getBuildHealthMeasure(startTime, endTime, project, build.Details.BuildHealthMeasureData, log)

	build.Details.BuildLatestTenMeasureData = &BuildDetail{}
	getBuildLatestTenMeasure(startTime, endTime, project, build.Details.BuildLatestTenMeasureData, log)

	// TODO: this data may be too large, need to be optimized
	//build.Details.BuildDailyMeasureData = &BuildDetail{}
	//getBuildDailyMeasure(startTime, endTime, project, build.Details.BuildDailyMeasureData, log)

	build.Details.BuildAverageMeasureData = &BuildDetail{}
	getBuildAverageMeasure(startTime, endTime, project, build.Details.BuildAverageMeasureData, log)

	return build, nil
}

func getBuildTrend(startTime, endTime int64, project string, detail *BuildDetail, log *zap.SugaredLogger) {
	// get build trend data
	buildTrendData, err := service2.GetBuildTrendMeasure(startTime, endTime, []string{project}, log)
	if err != nil {
		log.Errorf("Failed to get build trend data, the error is: %+v", err)
	}
	trend, err := json.Marshal(buildTrendData.Sum)
	if err != nil {
		log.Errorf("Failed to marshal build trend data, the error is: %+v", err)
	}
	detail.Description = "构建趋势"
	detail.Details = string(trend)
}

func getBuildHealthMeasure(startTime, endTime int64, project string, detail *BuildDetail, log *zap.SugaredLogger) {
	// get build health measure data
	buildHealthMeasureData, err := service2.GetBuildHealthMeasure(startTime, endTime, []string{project}, log)
	if err != nil {
		log.Errorf("Failed to get build health measure data, the error is: %+v", err)
	}
	health, err := json.Marshal(buildHealthMeasureData)
	if err != nil {
		log.Errorf("Failed to marshal build health measure data, the error is: %+v", err)
	}
	detail.Description = "构建健康度"
	detail.Details = string(health)
}

func getBuildLatestTenMeasure(startTime, endTime int64, project string, detail *BuildDetail, log *zap.SugaredLogger) {
	// get build latest ten measure data
	buildLatestTenMeasureData, err := service2.GetLatestTenBuildMeasure([]string{project}, log)
	if err != nil {
		log.Errorf("Failed to get build latest ten measure data, the error is: %+v", err)
	}
	latestTen, err := json.Marshal(buildLatestTenMeasureData)
	if err != nil {
		log.Errorf("Failed to marshal build latest ten measure data, the error is: %+v", err)
	}
	detail.Description = "最近十次构建"
	detail.Details = string(latestTen)
}

func getBuildDailyMeasure(startTime, endTime int64, project string, detail *BuildDetail, log *zap.SugaredLogger) {
	// get build daily measure data
	buildDailyMeasureData, err := service2.GetBuildDailyMeasure(startTime, endTime, []string{project}, log)
	if err != nil {
		log.Errorf("Failed to get build daily measure data, the error is: %+v", err)
	}
	daily, err := json.Marshal(buildDailyMeasureData)
	if err != nil {
		log.Errorf("Failed to marshal build daily measure data, the error is: %+v", err)
	}
	detail.Description = "构建日报"
	detail.Details = string(daily)
}

func getBuildAverageMeasure(startTime, endTime int64, project string, detail *BuildDetail, log *zap.SugaredLogger) {
	// get build average measure data
	buildAverageMeasureData, err := service2.GetBuildDailyAverageMeasure(startTime, endTime, []string{project}, log)
	if err != nil {
		log.Errorf("Failed to get build average measure data, the error is: %+v", err)
	}
	average, err := json.Marshal(buildAverageMeasureData)
	if err != nil {
		log.Errorf("Failed to marshal build average measure data, the error is: %+v", err)
	}
	detail.Description = "构建平均耗时"
	detail.Details = string(average)
}
