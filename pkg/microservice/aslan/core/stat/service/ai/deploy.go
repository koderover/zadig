package ai

import (
	"encoding/json"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	service2 "github.com/koderover/zadig/pkg/microservice/aslan/core/stat/service"
	"go.uber.org/zap"
)

type DeployData struct {
	Description string         `json:"description"`
	Details     *DeployDetails `json:"deploy_details"`
}

type DeployDetails struct {
	StatStartTime                   int64         `json:"stat_start_time"`
	StatEndTime                     int64         `json:"stat_end_time"`
	DeployTotal                     int           `json:"deploy_total"`
	DeploySuccessTotal              int           `json:"deploy_success_total"`
	DeployFailureTotal              int           `json:"deploy_failure_total"`
	DeployTotalDuration             int64         `json:"deploy_total_duration"`
	DeployHealthMeasureData         *DeployDetail `json:"deploy_health_measure_data"`
	DeployWeeklyMeasureData         *DeployDetail `json:"deploy_weekly_measure_data"`
	DeployTopFiveHigherMeasureData  *DeployDetail `json:"deploy_top_five_higher_measure_data"`
	DeployTopFiveFailureMeasureData *DeployDetail `json:"deploy_top_five_failure_measure_data"`
}

type DeployDetail struct {
	Description string `json:"description"`
	Details     string `json:"details"`
}

func getDeployData(project string, startTime, endTime int64, log *zap.SugaredLogger) (*DeployData, error) {
	deploy := &DeployData{
		Description: "部署数据",
		Details:     &DeployDetails{},
	}
	// get deploy data from mongo
	deployJobList, err := commonrepo.NewJobInfoColl().GetDeployJobs(startTime, endTime, project)
	if err != nil {
		return deploy, err
	}
	totalCounter := len(deployJobList)
	if totalCounter == 0 {
		return deploy, err
	}
	passCounter := 0
	for _, job := range deployJobList {
		if job.Status == string(config.StatusPassed) {
			passCounter++
		}
	}
	var totalTimesTaken int64 = 0
	for _, job := range deployJobList {
		totalTimesTaken += job.Duration
	}

	deploy.Details.DeployTotal = totalCounter
	deploy.Details.DeploySuccessTotal = passCounter
	deploy.Details.DeployFailureTotal = totalCounter - passCounter
	deploy.Details.DeployTotalDuration = totalTimesTaken / int64(totalCounter)

	deploy.Details.DeployHealthMeasureData = &DeployDetail{}
	getDeployHealthMeasure(project, startTime, endTime, deploy.Details.DeployHealthMeasureData, log)

	deploy.Details.DeployWeeklyMeasureData = &DeployDetail{}
	getDeployWeeklyMeasure(project, startTime, endTime, deploy.Details.DeployWeeklyMeasureData, log)

	deploy.Details.DeployTopFiveHigherMeasureData = &DeployDetail{}
	getDeployTopFiveHigherMeasure(project, startTime, endTime, deploy.Details.DeployTopFiveHigherMeasureData, log)

	deploy.Details.DeployTopFiveFailureMeasureData = &DeployDetail{}
	getDeployTopFiveFailureMeasure(project, startTime, endTime, deploy.Details.DeployTopFiveFailureMeasureData, log)

	return deploy, nil
}

func getDeployHealthMeasure(project string, startTime, endTime int64, detail *DeployDetail, log *zap.SugaredLogger) {
	// get deploy health measure data
	DeployHealthMeasure, err := service2.GetDeployHealthMeasure(startTime, endTime, []string{project}, log)
	if err != nil {
		log.Errorf("Failed to get deploy health measure data, the error is: %+v", err)
	}

	health, err := json.Marshal(DeployHealthMeasure)
	if err != nil {
		log.Errorf("Failed to marshal deploy health measure data, the error is: %+v", err)
	}
	detail.Description = "服务部署健康度"
	detail.Details = string(health)
}

func getDeployWeeklyMeasure(project string, startTime, endTime int64, detail *DeployDetail, log *zap.SugaredLogger) {
	// get deploy weekly measure data
	DeployWeeklyMeasure, err := service2.GetDeployWeeklyMeasure(startTime, endTime, []string{project}, log)
	if err != nil {
		log.Errorf("Failed to get deploy weekly measure data, the error is: %+v", err)
	}

	weekly, err := json.Marshal(DeployWeeklyMeasure)
	if err != nil {
		log.Errorf("Failed to marshal deploy weekly measure data, the error is: %+v", err)
	}
	detail.Description = "服务周部署频次"
	detail.Details = string(weekly)
}

func getDeployTopFiveHigherMeasure(project string, startTime, endTime int64, detail *DeployDetail, log *zap.SugaredLogger) {
	// get deploy top five higher measure data
	DeployTopFiveHigherMeasure, err := service2.GetDeployTopFiveHigherMeasure(startTime, endTime, []string{project}, log)
	if err != nil {
		log.Errorf("Failed to get deploy top five higher measure data, the error is: %+v", err)
	}

	higher, err := json.Marshal(DeployTopFiveHigherMeasure)
	if err != nil {
		log.Errorf("Failed to marshal deploy top five higher measure data, the error is: %+v", err)
	}
	detail.Description = "Top5服务部署统计"
	detail.Details = string(higher)
}

func getDeployTopFiveFailureMeasure(project string, startTime, endTime int64, detail *DeployDetail, log *zap.SugaredLogger) {
	// get deploy top five failure measure data
	DeployTopFiveFailureMeasure, err := service2.GetDeployTopFiveFailureMeasure(startTime, endTime, []string{project}, log)
	if err != nil {
		log.Errorf("Failed to get deploy top five failure measure data, the error is: %+v", err)
	}

	failure, err := json.Marshal(DeployTopFiveFailureMeasure)
	if err != nil {
		log.Errorf("Failed to marshal deploy top five failure measure data, the error is: %+v", err)
	}
	detail.Description = "Top5服务部署失败统计"
	detail.Details = string(failure)
}
