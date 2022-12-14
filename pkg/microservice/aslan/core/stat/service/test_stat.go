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

package service

import (
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/jinzhu/now"
	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	taskmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/task"
	commonmongodb "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	templaterepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb/template"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/base"
	s3service "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/s3"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/stat/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/stat/repository/mongodb"
	"github.com/koderover/zadig/pkg/setting"
	s3tool "github.com/koderover/zadig/pkg/tool/s3"
	"github.com/koderover/zadig/pkg/util"
)

func InitTestStat(log *zap.SugaredLogger) error {
	option := &commonmongodb.ListAllTaskOption{Type: config.WorkflowType}
	count, err := mongodb.NewTestStatColl().FindCount()
	if err != nil {
		log.Errorf("testStat FindCount err:%v", err)
		return fmt.Errorf("testStat FindCount err:%v", err)
	}
	if count > 0 {
		option.CreateTime = time.Now().AddDate(0, 0, -1).Unix()
	}
	//获取所有的项目名称
	allProducts, err := templaterepo.NewProductColl().List()
	if err != nil {
		log.Errorf("testStat ProductTmpl List err:%v", err)
		return fmt.Errorf("testStat ProductTmpl List err:%v", err)
	}
	for _, product := range allProducts {
		option.ProductNames = []string{product.ProductName}
		allTasks, err := commonmongodb.NewTaskColl().ListAllTasks(option)
		if err != nil {
			log.Errorf("pipeline list err:%v", err)
			return fmt.Errorf("pipeline list err:%v", err)
		}

		taskDateMap := make(map[string][]*taskmodels.Task)
		if len(allTasks) > 0 {
			//将task的时间戳转成日期，以日期为单位分组
			for _, task := range allTasks {
				time := time.Unix(task.CreateTime, 0)
				date := time.Format(config.Date)
				taskDateMap[date] = append(taskDateMap[date], task)
			}
		} else {
			time := time.Now().AddDate(0, 0, -1)
			date := time.Format(config.Date)
			taskDateMap[date] = []*taskmodels.Task{
				{Stages: []*commonmodels.Stage{}},
			}
		}

		taskDateKeys := make([]string, 0, len(taskDateMap))
		for taskDateMapKey := range taskDateMap {
			taskDateKeys = append(taskDateKeys, taskDateMapKey)
		}
		sort.Strings(taskDateKeys)

		defaultS3Storage, _ := s3service.FindDefaultS3()

		for _, taskDate := range taskDateKeys {
			var (
				totalSuccess     = 0
				totalFailure     = 0
				totalTimeout     = 0
				totalDuration    int64
				totalDeployCount = 0
				totalTestCount   = 0
				totalTestCase    = 0
			)
			//循环task任务获取需要的数据
			for _, taskPreview := range taskDateMap[taskDate] {
				stages := taskPreview.Stages
				for _, subStage := range stages {
					taskType := subStage.TaskType
					switch taskType {
					case config.TaskTestingV2:
						// 获取构建时长
						for _, subTask := range subStage.SubTasks {
							testInfo, err := base.ToTestingTask(subTask)
							if err != nil {
								log.Errorf("TestStat ToTestingTask err:%v", err)
								continue
							}
							if testInfo.TaskStatus == config.StatusPassed {
								totalSuccess++
							} else if testInfo.TaskStatus == config.StatusFailed {
								totalFailure++
							} else if testInfo.TaskStatus == config.StatusTimeout {
								totalTimeout++
							} else {
								continue
							}

							totalDuration += testInfo.EndTime - testInfo.StartTime
							totalTestCount++

							if testInfo.JobCtx.TestType != setting.PerformanceTest {
								func() {
									testJobName := strings.Replace(strings.ToLower(fmt.Sprintf("%s-%s-%d-%s-%s",
										config.WorkflowType, taskPreview.PipelineName, taskPreview.TaskID, config.TaskTestingV2, testInfo.TestModuleName)), "_", "-", -1)

									filename, err := util.GenerateTmpFile()
									defer func() {
										_ = os.Remove(filename)
									}()

									if err != nil {
										log.Errorf("failed to genarate tmp file, err: %s", err)
										return
									}

									objectKey := defaultS3Storage.GetObjectPath(fmt.Sprintf("%s/%d/%s/%s", taskPreview.PipelineName, taskPreview.TaskID, "test", testJobName))
									forcedPathStyle := true
									if defaultS3Storage.Provider == setting.ProviderSourceAli {
										forcedPathStyle = false
									}
									client, err := s3tool.NewClient(defaultS3Storage.Endpoint, defaultS3Storage.Ak, defaultS3Storage.Sk, defaultS3Storage.Region, defaultS3Storage.Insecure, forcedPathStyle)
									if err != nil {
										log.Errorf("Failed to create s3 client for download, error: %s", err)
										return
									}
									if err = client.Download(defaultS3Storage.Bucket, objectKey, filename); err != nil {
										log.Errorf("s3Service download err:%s", err)
										return
									}

									b, err := ioutil.ReadFile(filename)
									if err != nil {
										log.Error(fmt.Sprintf("InitTestStat get local test result file error: %v", err))
										return
									}
									testReport := new(commonmodels.TestReport)
									if err := xml.Unmarshal(b, &testReport.FunctionTestSuite); err != nil {
										log.Error(fmt.Sprintf("unmarshal it report xml error: %v", err))
										return
									}
									totalTestCase += testReport.FunctionTestSuite.Tests
								}()
							}
						}
					case config.TaskDeploy:
						totalDeployCount++
					}
				}
			}

			testStat := new(models.TestStat)
			testStat.ProductName = product.ProductName
			testStat.TotalSuccess = totalSuccess
			testStat.TotalFailure = totalFailure
			testStat.TotalTimeout = totalTimeout
			testStat.TotalDuration = totalDuration
			testStat.TotalTestCount = totalTestCount
			testStat.TotalDeployCount = totalDeployCount
			testStat.TotalTestCase = totalTestCase
			testStat.Date = taskDate
			tt, _ := time.ParseInLocation(config.Date, taskDate, time.Local)
			testStat.CreateTime = tt.Unix()
			testStat.UpdateTime = time.Now().Unix()

			err := mongodb.NewTestStatColl().Create(testStat)
			if err != nil { //插入失败就更新
				err = mongodb.NewTestStatColl().Update(testStat)
				if err != nil {
					log.Errorf("TestStat Update err:%v", err)
					continue
				}
			}
		}
	}
	return nil
}

type testStatDailyArg struct {
	Date            string `json:"date"`
	AverageDuration int    `json:"averageDuration"`
}

func GetTestAverageMeasure(startDate, endDate int64, productNames []string, log *zap.SugaredLogger) ([]*testStatDailyArg, error) {
	testStats, err := mongodb.NewTestStatColl().ListTestStat(&mongodb.TestStatOption{StartDate: startDate, EndDate: endDate, IsAsc: true, ProductNames: productNames})
	if err != nil {
		log.Errorf("ListTestStat err:%v", err)
		return nil, fmt.Errorf("ListTestStat err:%v", err)
	}
	testStatMap := make(map[string][]*models.TestStat)
	for _, testStat := range testStats {
		if _, isExist := testStatMap[testStat.Date]; isExist {
			testStatMap[testStat.Date] = append(testStatMap[testStat.Date], testStat)
		} else {
			tempTestStats := make([]*models.TestStat, 0)
			tempTestStats = append(tempTestStats, testStat)
			testStatMap[testStat.Date] = tempTestStats
		}
	}
	testStatDateKeys := make([]string, 0, len(testStatMap))
	for testStatDateMapKey := range testStatMap {
		testStatDateKeys = append(testStatDateKeys, testStatDateMapKey)
	}
	sort.Strings(testStatDateKeys)
	testStatDailyArgs := make([]*testStatDailyArg, 0)
	for _, testStatDateKey := range testStatDateKeys {
		totalDuration := 0
		totalTestCount := 0
		for _, testStat := range testStatMap[testStatDateKey] {
			totalDuration += int(testStat.TotalDuration)
			totalTestCount += testStat.TotalTestCount
		}
		testStatDailyArg := new(testStatDailyArg)
		testStatDailyArg.Date = testStatDateKey
		if totalTestCount > 0 {
			testStatDailyArg.AverageDuration = int(math.Floor(float64(totalDuration)/float64(totalTestCount) + 0.5))
		}

		testStatDailyArgs = append(testStatDailyArgs, testStatDailyArg)
	}
	return testStatDailyArgs, nil
}

type testCaseStat struct {
	Day      int64 `json:"day"`
	TestCase int   `json:"testCase"`
}

func GetTestCaseMeasure(startDate, endDate int64, productNames []string, log *zap.SugaredLogger) ([]*testCaseStat, error) {
	testStats, err := mongodb.NewTestStatColl().ListTestStat(&mongodb.TestStatOption{StartDate: startDate, EndDate: endDate, IsAsc: true, ProductNames: productNames})
	if err != nil {
		log.Errorf("ListTestStat err:%v", err)
		return nil, fmt.Errorf("ListTestStat err:%v", err)
	}
	testStatMap := make(map[string][]*models.TestStat)
	for _, testStat := range testStats {
		if _, isExist := testStatMap[testStat.Date]; isExist {
			testStatMap[testStat.Date] = append(testStatMap[testStat.Date], testStat)
		} else {
			tempTestStats := make([]*models.TestStat, 0)
			tempTestStats = append(tempTestStats, testStat)
			testStatMap[testStat.Date] = tempTestStats
		}
	}
	testStatDateKeys := make([]string, 0, len(testStatMap))
	for testStatDateMapKey := range testStatMap {
		testStatDateKeys = append(testStatDateKeys, testStatDateMapKey)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(testStatDateKeys)))
	testCaseStats := make([]*testCaseStat, 0)
	totalTestCase := 0
	if len(testStatMap) <= config.Day {
		for _, testStat := range testStats {
			totalTestCase += testStat.TotalTestCount * testStat.TotalTestCase
		}
		testCaseStat := &testCaseStat{
			Day:      time.Now().Unix(),
			TestCase: totalTestCase,
		}
		testCaseStats = append(testCaseStats, testCaseStat)
	} else {
		for index, testStatDate := range testStatDateKeys {
			for _, testStat := range testStatMap[testStatDate] {
				totalTestCase += testStat.TotalTestCount * testStat.TotalTestCase
			}

			if ((index + 1) % config.Day) == 0 {
				day, err := time.ParseInLocation("2006-01-02", testStatDate, time.Local)
				if err != nil {
					log.Errorf("Failed to parse %s as time, err:", testStatDate, err)
					return nil, err
				}
				testCaseStat := &testCaseStat{
					Day:      day.Unix(),
					TestCase: totalTestCase,
				}
				testCaseStats = append(testCaseStats, testCaseStat)
				totalTestCase = 0
			}
		}
	}

	return testCaseStats, nil
}

type testDeployStat struct {
	Day            int64 `json:"day"`
	DeliveryDeploy int   `json:"deliveryDeploy"`
}

func GetTestDeliveryDeployMeasure(startDate, endDate int64, productNames []string, log *zap.SugaredLogger) ([]*testDeployStat, error) {
	testStats, err := mongodb.NewTestStatColl().ListTestStat(&mongodb.TestStatOption{StartDate: startDate, EndDate: endDate, IsAsc: true, ProductNames: productNames})
	if err != nil {
		log.Errorf("ListTestStat err:%v", err)
		return nil, fmt.Errorf("ListTestStat err:%v", err)
	}
	testStatMap := make(map[string][]*models.TestStat)
	for _, testStat := range testStats {
		if _, isExist := testStatMap[testStat.Date]; isExist {
			testStatMap[testStat.Date] = append(testStatMap[testStat.Date], testStat)
		} else {
			tempTestStats := make([]*models.TestStat, 0)
			tempTestStats = append(tempTestStats, testStat)
			testStatMap[testStat.Date] = tempTestStats
		}
	}
	testStatDateKeys := make([]string, 0, len(testStatMap))
	for testStatDateMapKey := range testStatMap {
		testStatDateKeys = append(testStatDateKeys, testStatDateMapKey)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(testStatDateKeys)))
	testDeployStats := make([]*testDeployStat, 0)
	totalDeploy := 0
	if len(testStatMap) <= config.Day {
		for _, testStat := range testStats {
			totalDeploy += testStat.TotalDeployCount
		}
		testDeployStat := &testDeployStat{
			Day:            time.Now().Unix(),
			DeliveryDeploy: totalDeploy,
		}
		testDeployStats = append(testDeployStats, testDeployStat)
	} else {
		for index, testStatDate := range testStatDateKeys {
			for _, testStat := range testStatMap[testStatDate] {
				totalDeploy += testStat.TotalDeployCount
			}

			if ((index + 1) % config.Day) == 0 {
				day, err := time.ParseInLocation("2006-01-02", testStatDate, time.Local)
				if err != nil {
					log.Errorf("Failed to parse %s as time, err:", testStatDate, err)
					return nil, err
				}
				testDeployStat := &testDeployStat{
					Day:            day.Unix(),
					DeliveryDeploy: totalDeploy,
				}
				testDeployStats = append(testDeployStats, testDeployStat)
				totalDeploy = 0
			}
		}
	}

	return testDeployStats, nil
}

type testStatTotal struct {
	TotalSuccess int `json:"totalSuccess"`
	TotalFailure int `json:"totalFailure"`
}

func GetTestHealthMeasure(startDate, endDate int64, productNames []string, log *zap.SugaredLogger) (*testStatTotal, error) {
	testStats, err := mongodb.NewTestStatColl().ListTestStat(&mongodb.TestStatOption{StartDate: startDate, EndDate: endDate, IsAsc: true, ProductNames: productNames})
	if err != nil {
		log.Errorf("ListTestStat err:%v", err)
		return nil, fmt.Errorf("ListTestStat err:%v", err)
	}
	var (
		totalSuccess = 0
		totalFailure = 0
	)
	for _, testStat := range testStats {
		totalSuccess += testStat.TotalSuccess
		totalFailure += testStat.TotalFailure
	}
	testStatTotal := &testStatTotal{
		TotalSuccess: totalSuccess,
		TotalFailure: totalFailure,
	}
	return testStatTotal, nil
}

type testTrend struct {
	*CurrentDay
	Sum []*sumData `json:"sum"`
}

func GetTestTrendMeasure(startDate, endDate int64, productNames []string, log *zap.SugaredLogger) (*testTrend, error) {
	todayDate := now.BeginningOfDay().Unix()
	var (
		totalSuccess = 0
		totalFailure = 0
		totalTimeout = 0
	)
	allTasks, err := commonmongodb.NewTaskColl().ListAllTasks(&commonmongodb.ListAllTaskOption{Type: config.WorkflowType, CreateTime: todayDate, ProductNames: productNames})
	if err != nil {
		log.Errorf("ListAllTasks err:%v", err)
		return nil, fmt.Errorf("ListAllTasks err:%v", err)
	}
	for _, taskPreview := range allTasks {
		stages := taskPreview.Stages
		for _, subStage := range stages {
			taskType := subStage.TaskType
			switch taskType {
			case config.TaskTestingV2:
				// 获取构建时长
				for _, subTask := range subStage.SubTasks {
					testInfo, err := base.ToTestingTask(subTask)
					if err != nil {
						log.Errorf("TestStat ToTestingTask err:%v", err)
						continue
					}
					if testInfo.TaskStatus == config.StatusPassed {
						totalSuccess++
					} else if testInfo.TaskStatus == config.StatusFailed {
						totalFailure++
					} else if testInfo.TaskStatus == config.StatusTimeout {
						totalTimeout++
					} else {
						continue
					}
				}
			}
		}
	}
	currDay := &CurrentDay{
		Success: totalSuccess,
		Failure: totalFailure,
		Timeout: totalTimeout,
	}
	sumDatas := make([]*sumData, 0)

	testStats, err := mongodb.NewTestStatColl().ListTestStat(&mongodb.TestStatOption{StartDate: startDate, EndDate: endDate, IsAsc: false, ProductNames: productNames})
	if err != nil {
		log.Errorf("ListTestStat err:%v", err)
		return nil, fmt.Errorf("ListTestStat err:%v", err)
	}
	testStatMap := make(map[string][]*models.TestStat)
	for _, testStat := range testStats {
		if _, isExist := testStatMap[testStat.Date]; isExist {
			testStatMap[testStat.Date] = append(testStatMap[testStat.Date], testStat)
		} else {
			tempTestStats := make([]*models.TestStat, 0)
			tempTestStats = append(tempTestStats, testStat)
			testStatMap[testStat.Date] = tempTestStats
		}
	}
	testStatDateKeys := make([]string, 0, len(testStatMap))
	for testStatDateMapKey := range testStatMap {
		testStatDateKeys = append(testStatDateKeys, testStatDateMapKey)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(testStatDateKeys)))

	totalSuccess = 0
	totalFailure = 0
	totalTimeout = 0
	if len(testStatMap) <= config.Day {
		for _, testStat := range testStats {
			totalSuccess += testStat.TotalSuccess
			totalFailure += testStat.TotalFailure
			totalTimeout += testStat.TotalTimeout
		}
		sumData := &sumData{
			Day: time.Now().Unix(),
			CurrentDay: &CurrentDay{
				Success: totalSuccess,
				Failure: totalFailure,
				Timeout: totalTimeout,
			},
		}
		sumDatas = append(sumDatas, sumData)
	} else {
		for index, testStatDate := range testStatDateKeys {
			for _, testStat := range testStatMap[testStatDate] {
				totalSuccess += testStat.TotalSuccess
				totalFailure += testStat.TotalFailure
				totalTimeout += testStat.TotalTimeout
			}

			if ((index + 1) % config.Day) == 0 {
				day, err := time.ParseInLocation("2006-01-02", testStatDate, time.Local)
				if err != nil {
					log.Errorf("Failed to parse %s as time, err:", testStatDate, err)
					return nil, err
				}
				sumData := &sumData{
					Day: day.Unix(),
					CurrentDay: &CurrentDay{
						Success: totalSuccess,
						Failure: totalFailure,
						Timeout: totalTimeout,
					},
				}
				sumDatas = append(sumDatas, sumData)
				totalSuccess = 0
				totalFailure = 0
				totalTimeout = 0
			}
		}
	}

	testTrend := &testTrend{
		CurrentDay: currDay,
		Sum:        sumDatas,
	}

	return testTrend, nil
}
