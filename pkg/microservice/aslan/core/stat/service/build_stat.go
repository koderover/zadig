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
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/jinzhu/now"
	"github.com/pkg/errors"
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
	count, err := mongodb.NewBuildStatColl().FindCount()
	if err != nil {
		log.Errorf("BuildStat FindCount err:%v", err)
		return fmt.Errorf("BuildStat FindCount err:%v", err)
	}
	var createTime int64

	if count > 0 {
		createTime = time.Now().AddDate(0, 0, -1).Unix()
	}
	//获取所有的项目名称
	allProducts, err := templaterepo.NewProductColl().List()
	if err != nil {
		log.Errorf("BuildStat ProductTmpl List err:%v", err)
		return fmt.Errorf("BuildStat ProductTmpl List err:%v", err)
	}
	for _, product := range allProducts {
		buildStats, err := GetBuildStatByProdutName(product.ProductName, createTime, log)
		if err != nil {
			log.Errorf("list workflow build stat err: %v", err)
			return fmt.Errorf("list workflow build stat err: %v", err)
		}
		for _, buildStat := range buildStats {
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

func GetBuildStatByProdutName(productName string, startTimestamp int64, log *zap.SugaredLogger) ([]*models.BuildStat, error) {
	buildStats := []*models.BuildStat{}
	taskDateMap, err := getTaskDateMap(productName, startTimestamp)
	if err != nil {
		return buildStats, err
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
			switch taskP := taskPreview.(type) {
			case *taskmodels.Task:
				stages := taskP.Stages
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
							maxDurationPipeline.PipelineName = taskP.PipelineName
							maxDurationPipeline.DisplayName = taskP.PipelineDisplayName
							maxDurationPipeline.TaskID = taskP.TaskID
							maxDurationPipeline.Type = string(taskP.Type)
							maxDurationPipeline.MaxDuration = buildInfo.EndTime - buildInfo.StartTime
							maxDurationPipelines = append(maxDurationPipelines, maxDurationPipeline)
						}
					}
				}
			case *commonmodels.WorkflowTask:
				stages := taskP.Stages
				for _, stage := range stages {
					for _, job := range stage.Jobs {
						if job.JobType != string(config.JobZadigBuild) {
							continue
						}
						if job.Status == config.StatusPassed {
							totalSuccess++
						} else if job.Status == config.StatusFailed {
							totalFailure++
						} else if job.Status == config.StatusTimeout {
							totalTimeout++
						} else {
							continue
						}
						totalDuration += job.EndTime - job.StartTime
						maxDurationPipeline := new(models.PipelineInfo)
						maxDurationPipeline.PipelineName = taskP.WorkflowName
						maxDurationPipeline.DisplayName = taskP.WorkflowDisplayName
						maxDurationPipeline.TaskID = taskP.TaskID
						maxDurationPipeline.Type = string(config.WorkflowTypeV4)
						maxDurationPipeline.MaxDuration = job.EndTime - job.StartTime
						maxDurationPipelines = append(maxDurationPipelines, maxDurationPipeline)

					}
				}
			}
		}
		//比较maxDurationPipeline的MaxDuration的最大值
		sort.SliceStable(maxDurationPipelines, func(i, j int) bool { return maxDurationPipelines[i].MaxDuration > maxDurationPipelines[j].MaxDuration })
		buildStat := &models.BuildStat{}
		buildStat.ProductName = productName
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
		buildStats = append(buildStats, buildStat)
	}
	return buildStats, nil
}

func getTaskDateMap(productName string, startTimestamp int64) (map[string][]interface{}, error) {
	taskDateMap := make(map[string][]interface{})
	option := &commonmongodb.ListAllTaskOption{Type: config.WorkflowType, ProductName: productName, CreateTime: startTimestamp}
	cursor, err := commonmongodb.NewTaskColl().ListByCursor(option)
	if err != nil {
		return taskDateMap, fmt.Errorf("pipeline list err:%v", err)
	}
	for cursor.Next(context.Background()) {
		var workflowTask taskmodels.Task
		if err := cursor.Decode(&workflowTask); err != nil {
			return taskDateMap, fmt.Errorf("decode workflow task err:%v", err)
		}
		time := time.Unix(workflowTask.CreateTime, 0)
		date := time.Format(config.Date)
		if _, isExist := taskDateMap[date]; isExist {
			taskDateMap[date] = append(taskDateMap[date], &workflowTask)
		} else {
			tasks := make([]interface{}, 0)
			tasks = append(tasks, &workflowTask)
			taskDateMap[date] = tasks
		}
	}
	v4Option := &commonmongodb.ListWorkflowTaskV4Option{ProjectName: productName, CreateTime: startTimestamp}
	v4Cursor, err := commonmongodb.NewworkflowTaskv4Coll().ListByCursor(v4Option)
	if err != nil {
		return taskDateMap, fmt.Errorf("workflow v4 list err:%v", err)
	}
	for v4Cursor.Next(context.Background()) {
		var workflowTask commonmodels.WorkflowTask
		if err := v4Cursor.Decode(&workflowTask); err != nil {
			return taskDateMap, fmt.Errorf("decode workflow v4 task err:%v", err)
		}
		time := time.Unix(workflowTask.CreateTime, 0)
		date := time.Format(config.Date)
		if _, isExist := taskDateMap[date]; isExist {
			taskDateMap[date] = append(taskDateMap[date], &workflowTask)
		} else {
			tasks := make([]interface{}, 0)
			tasks = append(tasks, &workflowTask)
			taskDateMap[date] = tasks
		}
	}
	if len(taskDateMap) == 0 {
		localTime := time.Now().AddDate(0, 0, -1).In(time.Local)
		date := localTime.Format(config.Date)
		taskDateMap[date] = make([]interface{}, 0)
		taskDateMap[date] = append(taskDateMap[date], []*taskmodels.Task{
			{Stages: []*commonmodels.Stage{}},
		})
	}
	return taskDateMap, nil
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
	cursor, err := commonmongodb.NewworkflowTaskv4Coll().ListByCursor(&commonmongodb.ListWorkflowTaskV4Option{ProjectNames: productNames, IsSort: true})
	if err != nil {
		log.Errorf("list workflow v4 err: %v", err)
		return nil, fmt.Errorf("list workflow v4 err: %v", err)
	}
	latestPipelines := make([]*buildStatLatestTen, 0)
	for cursor.Next(context.Background()) {
		var workflowTask commonmodels.WorkflowTask
		if err := cursor.Decode(&workflowTask); err != nil {
			return nil, fmt.Errorf("decode workflow v4 task err:%v", err)
		}
		containBuild := false
		for _, stage := range workflowTask.Stages {
			for _, job := range stage.Jobs {
				if job.JobType != string(config.JobZadigBuild) {
					continue
				}
				containBuild = true
			}
		}
		if !containBuild {
			continue
		}
		latestPipelines = append(latestPipelines, &buildStatLatestTen{
			PipelineInfo: &models.PipelineInfo{
				TaskID:       workflowTask.TaskID,
				Type:         string(config.WorkflowTypeV4),
				PipelineName: workflowTask.WorkflowName,
				DisplayName:  getDisplayName(workflowTask.WorkflowName, workflowTask.WorkflowDisplayName),
			},
			ProductName: workflowTask.ProjectName,
			Duration:    workflowTask.EndTime - workflowTask.StartTime,
			Status:      string(workflowTask.Status),
			TaskCreator: workflowTask.TaskCreator,
			CreateTime:  workflowTask.CreateTime,
		})
		if len(latestPipelines) >= config.LatestDay {
			break
		}
	}
	latestTenTasks, err := commonmongodb.NewTaskColl().ListAllTasks(&commonmongodb.ListAllTaskOption{Type: config.WorkflowType, Limit: config.LatestDay, Skip: 0, ProductNames: productNames})
	if err != nil {
		log.Errorf("pipeline ListAllTasks err:%v", err)
		return nil, fmt.Errorf("pipeline ListAllTasks err:%v", err)
	}
	for _, latestTask := range latestTenTasks {
		buildStatLatestTen := &buildStatLatestTen{
			PipelineInfo: &models.PipelineInfo{
				TaskID:       latestTask.TaskID,
				Type:         string(latestTask.Type),
				PipelineName: latestTask.PipelineName,
				DisplayName:  getDisplayName(latestTask.PipelineName, latestTask.PipelineDisplayName),
			},
			ProductName: latestTask.ProductName,
			Duration:    latestTask.EndTime - latestTask.StartTime,
			Status:      string(latestTask.Status),
			TaskCreator: latestTask.TaskCreator,
			CreateTime:  latestTask.CreateTime,
		}
		latestPipelines = append(latestPipelines, buildStatLatestTen)
	}
	sort.SliceStable(latestPipelines, func(i, j int) bool {
		return latestPipelines[i].CreateTime > latestPipelines[j].CreateTime
	})

	if len(latestPipelines) >= 10 {
		latestPipelines = latestPipelines[:10]
	}

	return latestPipelines, nil
}

func GetTenDurationMeasure(startDate int64, endDate int64, productNames []string, log *zap.SugaredLogger) ([]*buildStatLatestTen, error) {
	maxTenDurationBuildStats, err := mongodb.NewBuildStatColl().ListBuildStat(&models.BuildStatOption{StartDate: startDate, EndDate: endDate, Limit: config.LatestDay, Skip: 0, ProductNames: productNames})
	if err != nil {
		log.Errorf("ListBuildStat err:%v", err)
		return nil, fmt.Errorf("ListBuildStat err:%v", err)
	}
	latestTenPipelines := make([]*buildStatLatestTen, 0)
	for _, buidStat := range maxTenDurationBuildStats {
		task, err := getTaskDetail(buidStat.MaxDurationPipeline.Type, buidStat.MaxDurationPipeline.PipelineName, buidStat.MaxDurationPipeline.TaskID)
		if err != nil {
			log.Errorf("PipelineTask Find err:%v", err)
			continue
		}

		buildStatLatestTen := &buildStatLatestTen{
			PipelineInfo: buidStat.MaxDurationPipeline,
			ProductName:  task.Project,
			Duration:     task.Duration,
			Status:       string(task.Status),
			TaskCreator:  task.TaskCreator,
			CreateTime:   task.CreateTime,
		}
		buildStatLatestTen.DisplayName = getDisplayName(buildStatLatestTen.PipelineName, buildStatLatestTen.DisplayName)
		latestTenPipelines = append(latestTenPipelines, buildStatLatestTen)
	}
	return latestTenPipelines, nil
}

func getDisplayName(name, displayName string) string {
	if displayName != "" {
		return displayName
	}
	return name
}

type TaskPreview struct {
	Project     string
	Duration    int64
	Status      string
	TaskCreator string
	CreateTime  int64
}

func getTaskDetail(taskType, workflowName string, taskID int64) (*TaskPreview, error) {
	if config.PipelineType(taskType) == config.WorkflowTypeV4 {
		task, err := commonmongodb.NewworkflowTaskv4Coll().Find(workflowName, taskID)
		if err != nil {
			return nil, errors.Errorf("find workflow v4 task err:%v", err)
		}
		return &TaskPreview{
			Project:     task.ProjectName,
			Duration:    task.EndTime - task.StartTime,
			Status:      string(task.Status),
			TaskCreator: task.TaskCreator,
			CreateTime:  task.CreateTime,
		}, nil
	}
	task, err := commonmongodb.NewTaskColl().Find(taskID, workflowName, config.PipelineType(taskType))
	if err != nil {
		return nil, errors.Errorf("find workflow task err:%v", err)
	}
	return &TaskPreview{
		Project:     task.ProductName,
		Duration:    task.EndTime - task.StartTime,
		Status:      string(task.Status),
		TaskCreator: task.TaskCreator,
		CreateTime:  task.CreateTime,
	}, nil
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
