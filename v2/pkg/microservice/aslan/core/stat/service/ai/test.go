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

package ai

import (
	"encoding/json"
	"fmt"
	"time"

	"go.uber.org/zap"

	service2 "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/stat/service"
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
	TestWeeklyTrendData    *TestDetail `json:"test_weekly_trend_data"`
	TestDailyMeasureData   *TestDetail `json:"test_daily_measure_data"`
	TestHealthMeasureData  *TestDetail `json:"test_health_measure_data"`
	TestCaseMeasureData    *TestDetail `json:"test_case_measure_data"`
	TestAverageMeasureData *TestDetail `json:"test_average_measure_data"`
	TestDeliveryDeployData *TestDetail `json:"test_delivery_deploy_data"`
}

type TestDetail struct {
	Description string `json:"data_description"`
	Details     string `json:"data_details"`
}

func getTestData(project string, startTime, endTime int64, log *zap.SugaredLogger) (*TestData, error) {
	test := &TestData{
		Description: fmt.Sprintf("%s项目在%s到%s期间测试相关数据，包括测试总次数，测试通过次数，测试失败次数, 测试周趋势数据，测试每日数据", project, time.Unix(startTime, 0).Format("2006-01-02"), time.Unix(endTime, 0).Format("2006-01-02")),
		Details:     &TestDetails{},
	}
	// get test data from mongo
	testJobList, err := service2.GetProjectTestStat(startTime, endTime, project)
	if err != nil {
		log.Errorf("GetProjectTestStat error: %v", err)
		return nil, err
	}
	test.Details.TestTotal = testJobList.Total
	test.Details.TestPass = testJobList.Success
	test.Details.TestFail = testJobList.Failure
	test.Details.TestTotalDuration = int64(testJobList.Duration)

	test.Details.TestWeeklyTrendData = &TestDetail{}
	getTestTrendMeasure(project, startTime, endTime, test.Details.TestWeeklyTrendData, log)

	test.Details.TestDailyMeasureData = &TestDetail{}
	getTestDailyMeasure(project, startTime, endTime, test.Details.TestDailyMeasureData, log)

	return test, nil
}

func getTestTrendMeasure(project string, startTime, endTime int64, testTrendData *TestDetail, log *zap.SugaredLogger) {
	// get test trend data
	testTrendMeasure, err := service2.GetWeeklyTestStatus(startTime, endTime, []string{project})
	if err != nil {
		log.Errorf("failed to get test trend measure, the error is: %+v", err)
	}

	trend, err := json.Marshal(testTrendMeasure)
	if err != nil {
		log.Errorf("failed to marshal test trend measure, the error is: %+v", err)
	}

	testTrendData.Description = fmt.Sprintf("%s项目在%s到%s期间测试周趋势数据, 主要是统计每周内总的测试次数，测试成功次数，测试失败次数，测试平均耗时", project, time.Unix(startTime, 0).Format("2006-01-02"), time.Unix(endTime, 0).Format("2006-01-02"))
	testTrendData.Details = string(trend)
}

func getTestDailyMeasure(project string, startTime, endTime int64, testDailyMeasureData *TestDetail, log *zap.SugaredLogger) {
	dailyTestData, err := service2.GetDailyTestStatus(startTime, endTime, []string{project})
	if err != nil {
		log.Errorf("failed to get daily test status, the error is: %+v", err)
	}

	daily, err := json.Marshal(dailyTestData)
	if err != nil {
		log.Errorf("failed to marshal daily test status, the error is: %+v", err)
	}

	testDailyMeasureData.Description = fmt.Sprintf("%s项目在%s到%s期间测试每日数据, 主要是统计每日内总的测试次数，测试成功次数，测试失败次数，测试平均耗时", project, time.Unix(startTime, 0).Format("2006-01-02"), time.Unix(endTime, 0).Format("2006-01-02"))
	testDailyMeasureData.Details = string(daily)
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
