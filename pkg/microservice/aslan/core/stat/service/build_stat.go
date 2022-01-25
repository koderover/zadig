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
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/jinzhu/now"
	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	taskmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/task"
	commonmongodb "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	templaterepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb/template"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/base"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/stat/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/stat/repository/mongodb"
)

func GetAllPipelineTask(log *zap.SugaredLogger) error {
	option := &commonmongodb.ListAllTaskOption{Type: config.WorkflowType}
	count, err := mongodb.NewBuildStatColl().FindCount()
	if err != nil {
		log.Errorf("BuildStat FindCount err:%v", err)
		return fmt.Errorf("BuildStat FindCount err:%v", err)
	}
	if count > 0 {
		option.CreateTime = time.Now().AddDate(0, 0, -1).Unix()
	}
	//获取所有的项目名称
	allProducts, err := templaterepo.NewProductColl().List()
	if err != nil {
		log.Errorf("BuildStat ProductTmpl List err:%v", err)
		return fmt.Errorf("BuildStat ProductTmpl List err:%v", err)
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
				if _, isExist := taskDateMap[date]; isExist {
					taskDateMap[date] = append(taskDateMap[date], task)
				} else {
					tasks := make([]*taskmodels.Task, 0)
					tasks = append(tasks, task)
					taskDateMap[date] = tasks
				}
			}
		} else {
			localTime := time.Now().AddDate(0, 0, -1).In(time.Local)
			date := localTime.Format(config.Date)
			taskDateMap[date] = []*taskmodels.Task{
				{Stages: []*commonmodels.Stage{}},
			}
		}

		taskDateKeys := make([]string, 0, len(taskDateMap))
		for taskDateMapKey := range taskDateMap {
			taskDateKeys = append(taskDateKeys, taskDateMapKey)
		}
		sort.Strings(taskDateKeys)

		for _, taskDate := range taskDateKeys {
			var (
				totalSuccess         = 0
				totalFailure         = 0
				totalTimeout         = 0
				totalDuration        int64
				maxDurationPipelines = make([]*models.PipelineInfo, 0)
			)
			//循环task任务获取需要的数据
			for _, taskPreview := range taskDateMap[taskDate] {
				stages := taskPreview.Stages
				for _, subStage := range stages {
					taskType := subStage.TaskType
					switch taskType {
					case config.TaskBuild:
						// 获取构建时长
						for _, subTask := range subStage.SubTasks {
							buildInfo, err := base.ToBuildTask(subTask)
							if err != nil {
								log.Errorf("BuildStat ToBuildTask err:%v", err)
								continue
							}
							if buildInfo.TaskStatus == config.StatusPassed {
								totalSuccess++
							} else if buildInfo.TaskStatus == config.StatusFailed {
								totalFailure++
							} else if buildInfo.TaskStatus == config.StatusTimeout {
								totalTimeout++
							} else {
								continue
							}

							totalDuration += buildInfo.EndTime - buildInfo.StartTime
							maxDurationPipeline := new(models.PipelineInfo)
							maxDurationPipeline.PipelineName = taskPreview.PipelineName
							maxDurationPipeline.TaskID = taskPreview.TaskID
							maxDurationPipeline.Type = string(taskPreview.Type)
							maxDurationPipeline.MaxDuration = buildInfo.EndTime - buildInfo.StartTime

							maxDurationPipelines = append(maxDurationPipelines, maxDurationPipeline)
						}
					}
				}
			}
			//比较maxDurationPipeline的MaxDuration的最大值
			sort.SliceStable(maxDurationPipelines, func(i, j int) bool { return maxDurationPipelines[i].MaxDuration > maxDurationPipelines[j].MaxDuration })
			buildStat := new(models.BuildStat)
			buildStat.ProductName = product.ProductName
			buildStat.TotalSuccess = totalSuccess
			buildStat.TotalFailure = totalFailure
			buildStat.TotalTimeout = totalTimeout
			buildStat.TotalDuration = totalDuration
			if len(maxDurationPipelines) > 0 {
				buildStat.TotalBuildCount = len(maxDurationPipelines)
				buildStat.MaxDuration = maxDurationPipelines[0].MaxDuration
				buildStat.MaxDurationPipeline = maxDurationPipelines[0]
			} else {
				buildStat.TotalBuildCount = 0
				buildStat.MaxDuration = 0
				buildStat.MaxDurationPipeline = &models.PipelineInfo{}
			}
			buildStat.Date = taskDate
			tt, _ := time.ParseInLocation(config.Date, taskDate, time.Local)
			buildStat.CreateTime = tt.Unix()
			buildStat.UpdateTime = time.Now().Unix()

			err := mongodb.NewBuildStatColl().Create(buildStat)
			if err != nil { //插入失败就更新
				err = mongodb.NewBuildStatColl().Update(buildStat)
				if err != nil {
					log.Errorf("BuildStat Update err:%v", err)
					continue
				}
			}
		}
	}
	return nil
}

type dailyBuildStat struct {
	Date            string `bson:"date"                    json:"date"`
	AverageDuration int    `bson:"average_duration"        json:"averageDuration"`
}

func GetBuildDailyAverageMeasure(startDate int64, endDate int64, productNames []string, log *zap.SugaredLogger) ([]*dailyBuildStat, error) {
	buildStats, err := mongodb.NewBuildStatColl().ListBuildStat(&models.BuildStatOption{StartDate: startDate, EndDate: endDate, IsAsc: true, ProductNames: productNames})
	if err != nil {
		log.Errorf("ListBuildStat err:%v", err)
		return nil, fmt.Errorf("ListBuildStat err:%v", err)
	}
	buildStatMap := make(map[string][]*models.BuildStat)
	for _, buildStat := range buildStats {
		if _, isExist := buildStatMap[buildStat.Date]; isExist {
			buildStatMap[buildStat.Date] = append(buildStatMap[buildStat.Date], buildStat)
		} else {
			tempBuildStats := make([]*models.BuildStat, 0)
			tempBuildStats = append(tempBuildStats, buildStat)
			buildStatMap[buildStat.Date] = tempBuildStats
		}
	}
	buildStatDateKeys := make([]string, 0, len(buildStatMap))
	for buildStatDateMapKey := range buildStatMap {
		buildStatDateKeys = append(buildStatDateKeys, buildStatDateMapKey)
	}
	sort.Strings(buildStatDateKeys)
	buildStatDailyArgs := make([]*dailyBuildStat, 0)
	for _, buildStatDateKey := range buildStatDateKeys {
		totalDuration := 0
		totalBuildCount := 0
		for _, buildStat := range buildStatMap[buildStatDateKey] {
			totalDuration += int(buildStat.TotalDuration)
			totalBuildCount += buildStat.TotalBuildCount
		}
		buildStatDailyArg := new(dailyBuildStat)
		buildStatDailyArg.Date = buildStatDateKey
		if totalBuildCount > 0 {
			buildStatDailyArg.AverageDuration = int(math.Floor(float64(totalDuration)/float64(totalBuildCount) + 0.5))
		}

		buildStatDailyArgs = append(buildStatDailyArgs, buildStatDailyArg)
	}
	return buildStatDailyArgs, nil
}

type buildStatDaily struct {
	Date    string `bson:"date"                    json:"date"`
	Success int    `bson:"success"                 json:"success"`
	Failure int    `bson:"failure"                 json:"failure"`
}

func GetBuildDailyMeasure(startDate int64, endDate int64, productNames []string, log *zap.SugaredLogger) ([]*buildStatDaily, error) {
	buildStats, err := mongodb.NewBuildStatColl().ListBuildStat(&models.BuildStatOption{StartDate: startDate, EndDate: endDate, IsAsc: true, ProductNames: productNames})
	if err != nil {
		log.Errorf("ListBuildStat err:%v", err)
		return nil, fmt.Errorf("ListBuildStat err:%v", err)
	}

	buildStatMap := make(map[string][]*models.BuildStat)
	for _, buildStat := range buildStats {
		if _, isExist := buildStatMap[buildStat.Date]; isExist {
			buildStatMap[buildStat.Date] = append(buildStatMap[buildStat.Date], buildStat)
		} else {
			tempBuildStats := make([]*models.BuildStat, 0)
			tempBuildStats = append(tempBuildStats, buildStat)
			buildStatMap[buildStat.Date] = tempBuildStats
		}

	}
	buildStatDateKeys := make([]string, 0, len(buildStatMap))
	for buildStatDateMapKey := range buildStatMap {
		buildStatDateKeys = append(buildStatDateKeys, buildStatDateMapKey)
	}
	sort.Strings(buildStatDateKeys)

	buildStatDailys := make([]*buildStatDaily, 0)
	for _, buildStatDateKey := range buildStatDateKeys {
		totalSuccess := 0
		totalFailure := 0
		for _, buildStat := range buildStatMap[buildStatDateKey] {
			totalSuccess += buildStat.TotalSuccess
			totalFailure += buildStat.TotalFailure
		}

		buildStatDaily := new(buildStatDaily)
		buildStatDaily.Date = buildStatDateKey
		buildStatDaily.Success = totalSuccess
		buildStatDaily.Failure = totalFailure

		buildStatDailys = append(buildStatDailys, buildStatDaily)
	}
	return buildStatDailys, nil
}

type buildStatTotal struct {
	TotalSuccess int `json:"totalSuccess"`
	TotalFailure int `bson:"total_failure"                 json:"totalFailure"`
}

func GetBuildHealthMeasure(startDate int64, endDate int64, productNames []string, log *zap.SugaredLogger) (*buildStatTotal, error) {
	buildStats, err := mongodb.NewBuildStatColl().ListBuildStat(&models.BuildStatOption{StartDate: startDate, EndDate: endDate, IsAsc: true, ProductNames: productNames})
	if err != nil {
		log.Errorf("ListBuildStat err:%v", err)
		return nil, fmt.Errorf("ListBuildStat err:%v", err)
	}
	var (
		totalSuccess = 0
		totalFailure = 0
	)
	for _, buildStat := range buildStats {
		totalSuccess += buildStat.TotalSuccess
		totalFailure += buildStat.TotalFailure
	}
	buildStatTotal := &buildStatTotal{
		TotalSuccess: totalSuccess,
		TotalFailure: totalFailure,
	}
	return buildStatTotal, nil
}

type buildStatLatestTen struct {
	*models.PipelineInfo

	ProductName string `bson:"product_name"           json:"productName"`
	Duration    int64  `bson:"duration"               json:"duration"`
	Status      string `bson:"status"                 json:"status"`
	TaskCreator string `bson:"task_creator"           json:"taskCreator"`
	CreateTime  int64  `bson:"create_time"            json:"createTime"`
}

func GetLatestTenBuildMeasure(productNames []string, log *zap.SugaredLogger) ([]*buildStatLatestTen, error) {
	latestTenTasks, err := commonmongodb.NewTaskColl().ListAllTasks(&commonmongodb.ListAllTaskOption{Type: config.WorkflowType, Limit: config.LatestDay, Skip: 0, ProductNames: productNames})
	if err != nil {
		log.Errorf("pipeline ListAllTasks err:%v", err)
		return nil, fmt.Errorf("pipeline ListAllTasks err:%v", err)
	}
	latestTenPipelines := make([]*buildStatLatestTen, 0)
	for _, latestTask := range latestTenTasks {
		buildStatLatestTen := &buildStatLatestTen{
			PipelineInfo: &models.PipelineInfo{
				TaskID:       latestTask.TaskID,
				Type:         string(latestTask.Type),
				PipelineName: latestTask.PipelineName,
			},
			ProductName: latestTask.ProductName,
			Duration:    latestTask.EndTime - latestTask.StartTime,
			Status:      string(latestTask.Status),
			TaskCreator: latestTask.TaskCreator,
			CreateTime:  latestTask.CreateTime,
		}
		latestTenPipelines = append(latestTenPipelines, buildStatLatestTen)
	}

	return latestTenPipelines, nil
}

func GetTenDurationMeasure(startDate int64, endDate int64, productNames []string, log *zap.SugaredLogger) ([]*buildStatLatestTen, error) {
	maxTenDurationBuildStats, err := mongodb.NewBuildStatColl().ListBuildStat(&models.BuildStatOption{StartDate: startDate, EndDate: endDate, Limit: config.LatestDay, Skip: 0, ProductNames: productNames})
	if err != nil {
		log.Errorf("ListBuildStat err:%v", err)
		return nil, fmt.Errorf("ListBuildStat err:%v", err)
	}
	latestTenPipelines := make([]*buildStatLatestTen, 0)
	for _, buidStat := range maxTenDurationBuildStats {
		task, err := commonmongodb.NewTaskColl().Find(buidStat.MaxDurationPipeline.TaskID, buidStat.MaxDurationPipeline.PipelineName, config.PipelineType(buidStat.MaxDurationPipeline.Type))
		if err != nil {
			log.Errorf("PipelineTask Find err:%v", err)
			continue
		}

		buildStatLatestTen := &buildStatLatestTen{
			PipelineInfo: buidStat.MaxDurationPipeline,
			ProductName:  task.ProductName,
			Duration:     task.EndTime - task.StartTime,
			Status:       string(task.Status),
			TaskCreator:  task.TaskCreator,
			CreateTime:   task.CreateTime,
		}
		latestTenPipelines = append(latestTenPipelines, buildStatLatestTen)
	}
	return latestTenPipelines, nil
}

type buildTrend struct {
	*CurrentDay
	Sum []*sumData `json:"sum"`
}

type CurrentDay struct {
	Success int `json:"success"`
	Failure int `json:"failure"`
	Timeout int `json:"timeout"`
}

type sumData struct {
	*CurrentDay
	Day int64 `json:"day"`
}

func GetBuildTrendMeasure(startDate int64, endDate int64, productNames []string, log *zap.SugaredLogger) (*buildTrend, error) {
	dayStartTimestamp := now.BeginningOfDay().Unix()
	var (
		totalSuccess = 0
		totalFailure = 0
		totalTimeout = 0
	)
	allTasks, err := commonmongodb.NewTaskColl().ListAllTasks(&commonmongodb.ListAllTaskOption{Type: config.WorkflowType, CreateTime: dayStartTimestamp, ProductNames: productNames})
	if err != nil {
		log.Errorf("ListAllTasks err:%v", err)
		return nil, fmt.Errorf("ListAllTasks err:%v", err)
	}
	for _, taskPreview := range allTasks {
		stages := taskPreview.Stages
		for _, subStage := range stages {
			taskType := subStage.TaskType
			switch taskType {
			case config.TaskBuild:
				// 获取构建时长
				for _, subTask := range subStage.SubTasks {
					buildInfo, err := base.ToBuildTask(subTask)
					if err != nil {
						log.Errorf("BuildStat ToBuildTask err:%v", err)
						continue
					}
					if buildInfo.TaskStatus == config.StatusPassed {
						totalSuccess++
					} else if buildInfo.TaskStatus == config.StatusFailed {
						totalFailure++
					} else if buildInfo.TaskStatus == config.StatusTimeout {
						totalTimeout++
					} else {
						continue
					}
				}
			}
		}
	}
	curr := &CurrentDay{
		Success: totalSuccess,
		Failure: totalFailure,
		Timeout: totalTimeout,
	}
	sumDatas := make([]*sumData, 0)
	buildStats, err := mongodb.NewBuildStatColl().ListBuildStat(&models.BuildStatOption{StartDate: startDate, EndDate: endDate, IsAsc: false, ProductNames: productNames})
	if err != nil {
		log.Errorf("ListBuildStat err:%v", err)
		return nil, fmt.Errorf("ListBuildStat err:%v", err)
	}
	buildStatMap := make(map[string][]*models.BuildStat)
	for _, buildStat := range buildStats {
		if _, isExist := buildStatMap[buildStat.Date]; isExist {
			buildStatMap[buildStat.Date] = append(buildStatMap[buildStat.Date], buildStat)
		} else {
			tempBuildStats := make([]*models.BuildStat, 0)
			tempBuildStats = append(tempBuildStats, buildStat)
			buildStatMap[buildStat.Date] = tempBuildStats
		}
	}
	buildStatDateKeys := make([]string, 0, len(buildStatMap))
	for buildStatDateMapKey := range buildStatMap {
		buildStatDateKeys = append(buildStatDateKeys, buildStatDateMapKey)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(buildStatDateKeys)))

	totalSuccess = 0
	totalFailure = 0
	totalTimeout = 0
	if len(buildStatMap) <= config.Day {
		for _, buildStat := range buildStats {
			totalSuccess += buildStat.TotalSuccess
			totalFailure += buildStat.TotalFailure
			totalTimeout += buildStat.TotalTimeout
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
		for index, buildStatDate := range buildStatDateKeys {
			for _, buildStat := range buildStatMap[buildStatDate] {
				totalSuccess += buildStat.TotalSuccess
				totalFailure += buildStat.TotalFailure
				totalTimeout += buildStat.TotalTimeout
			}

			if ((index + 1) % config.Day) == 0 {
				date, err := time.ParseInLocation(config.Date, buildStatDate, time.Local)
				if err != nil {
					log.Errorf("Failed to parse date: %s, the error is: %+v", buildStatDate, err)
					return nil, err
				}
				sumData := &sumData{
					Day: date.Unix(),
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

	buildTrend := &buildTrend{
		CurrentDay: curr,
		Sum:        sumDatas,
	}

	return buildTrend, nil
}
