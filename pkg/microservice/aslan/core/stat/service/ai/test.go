package ai

import (
	"encoding/json"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	service2 "github.com/koderover/zadig/pkg/microservice/aslan/core/stat/service"
	"go.uber.org/zap"
)

type TestData struct {
	Description string       `json:"description"`
	Details     *TestDetails `json:"test_details"`
}

type TestDetails struct {
	TestTotal              int         `json:"test_total"`
	TestPass               int         `json:"test_pass_total"`
	TestFail               int         `json:"test_fail_total"`
	TestTotalDuration      int64       `json:"test_total_duration"`
	TestTrendData          *TestDetail `json:"test_trend_data"`
	TestHealthMeasureData  *TestDetail `json:"test_health_measure_data"`
	TestCaseMeasureData    *TestDetail `json:"test_case_measure_data"`
	TestAverageMeasureData *TestDetail `json:"test_average_measure_data"`
	TestDeliveryDeployData *TestDetail `json:"test_delivery_deploy_data"`
}

type TestDetail struct {
	Description string `json:"description"`
	Details     string `json:"details"`
}

func getTestData(project string, startTime, endTime int64, log *zap.SugaredLogger) (*TestData, error) {
	test := &TestData{
		Description: "测试数据",
		Details:     &TestDetails{},
	}
	// get test data from mongo
	testJobList, err := commonrepo.NewJobInfoColl().GetTestJobs(startTime, endTime, project)
	if err != nil {
		return test, err
	}
	totalCounter := len(testJobList)
	if totalCounter == 0 {
		return test, err
	}
	passCounter := 0
	for _, job := range testJobList {
		if job.Status == string(config.StatusPassed) {
			passCounter++
		}
	}
	var totalTimesTaken int64 = 0
	for _, job := range testJobList {
		totalTimesTaken += job.Duration
	}

	test.Details.TestTotal = totalCounter
	test.Details.TestPass = passCounter
	test.Details.TestFail = totalCounter - passCounter
	test.Details.TestTotalDuration = totalTimesTaken / int64(totalCounter)

	test.Details.TestTrendData = &TestDetail{}
	getTestTrendMeasure(project, startTime, endTime, test.Details.TestTrendData, log)

	test.Details.TestHealthMeasureData = &TestDetail{}
	getTestHealthMeasure(project, startTime, endTime, test.Details.TestHealthMeasureData, log)

	test.Details.TestCaseMeasureData = &TestDetail{}
	getTestCaseMeasure(project, startTime, endTime, test.Details.TestCaseMeasureData, log)

	// TODO: this data may be too large, need to be optimized
	//test.TestAverageMeasureData = &TestDetail{}
	//getTestAverageMeasure(project, startTime, endTime, test.TestAverageMeasureData, log)

	test.Details.TestDeliveryDeployData = &TestDetail{}
	getTestDeliveryDeploy(project, startTime, endTime, test.Details.TestDeliveryDeployData, log)

	return test, nil
}

func getTestTrendMeasure(project string, startTime, endTime int64, testTrendData *TestDetail, log *zap.SugaredLogger) {
	// get test trend data
	testTrendMeasure, err := service2.GetTestTrendMeasure(startTime, endTime, []string{project}, log)
	if err != nil {
		log.Errorf("failed to get test trend measure, the error is: %+v", err)
	}

	trend, err := json.Marshal(testTrendMeasure)
	if err != nil {
		log.Errorf("failed to marshal test trend measure, the error is: %+v", err)
	}

	testTrendData.Description = "测试趋势（周）"
	testTrendData.Details = string(trend)
}

func getTestHealthMeasure(project string, startTime, endTime int64, testHealthMeasureData *TestDetail, log *zap.SugaredLogger) {
	// get test health measure
	testHealthMeasure, err := service2.GetTestHealthMeasure(startTime, endTime, []string{project}, log)
	if err != nil {
		log.Errorf("failed to get test health measure, the error is: %+v", err)
	}

	health, err := json.Marshal(testHealthMeasure)
	if err != nil {
		log.Errorf("failed to marshal test health measure, the error is: %+v", err)
	}

	testHealthMeasureData.Description = "测试健康度"
	testHealthMeasureData.Details = string(health)
}

func getTestCaseMeasure(project string, startTime, endTime int64, testCaseMeasureData *TestDetail, log *zap.SugaredLogger) {
	// get test case measure
	testCaseMeasure, err := service2.GetTestCaseMeasure(startTime, endTime, []string{project}, log)
	if err != nil {
		log.Errorf("failed to get test case measure, the error is: %+v", err)
	}

	caseMeasure, err := json.Marshal(testCaseMeasure)
	if err != nil {
		log.Errorf("failed to marshal test case measure, the error is: %+v", err)
	}

	testCaseMeasureData.Description = "周测试收益（执行次数 x 测试用例数）"
	testCaseMeasureData.Details = string(caseMeasure)
}

func getTestAverageMeasure(project string, startTime, endTime int64, testAverageMeasureData *TestDetail, log *zap.SugaredLogger) {
	// get test average measure
	testAverageMeasure, err := service2.GetTestAverageMeasure(startTime, endTime, []string{project}, log)
	if err != nil {
		log.Errorf("failed to get test average measure, the error is: %+v", err)
	}

	average, err := json.Marshal(testAverageMeasure)
	if err != nil {
		log.Errorf("failed to marshal test average measure, the error is: %+v", err)
	}

	testAverageMeasureData.Description = "平均测试时长"
	testAverageMeasureData.Details = string(average)
}

func getTestDeliveryDeploy(project string, startTime, endTime int64, testDeliveryDeployData *TestDetail, log *zap.SugaredLogger) {
	// get test delivery deploy
	testDeliveryDeploy, err := service2.GetTestDeliveryDeployMeasure(startTime, endTime, []string{project}, log)
	if err != nil {
		log.Errorf("failed to get test delivery deploy, the error is: %+v", err)
	}

	delivery, err := json.Marshal(testDeliveryDeploy)
	if err != nil {
		log.Errorf("failed to marshal test delivery deploy, the error is: %+v", err)
	}

	testDeliveryDeployData.Description = "周交付部署次数"
	testDeliveryDeployData.Details = string(delivery)
}
