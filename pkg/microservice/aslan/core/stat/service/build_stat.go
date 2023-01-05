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
		workflowBuildStats, err := GetworkflowBuildStatByProdutName(product.ProductName, createTime, log)
		if err != nil {
			log.Errorf("list workkflow build stat err: %v", err)
			return fmt.Errorf("list workkflow build stat err: %v", err)
		}
		workflowV4BuildStats, err := GetworkflowV4BuildStatByProdutName(product.ProductName, createTime, log)
		if err != nil {
			log.Errorf("list workkflow v4 build stat err: %v", err)
			return fmt.Errorf("list workkflow v4 build stat err: %v", err)
		}
		allStats := append(workflowBuildStats, workflowV4BuildStats...)
		stats := mergeBuildStat(allStats)
		for _, buildStat := range stats {
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

func GetworkflowBuildStatByProdutName(productName string, startTimestamp int64, log *zap.SugaredLogger) ([]*models.BuildStat, error) {
	buildStats := []*models.BuildStat{}
	option := &commonmongodb.ListAllTaskOption{Type: config.WorkflowType, ProductName: productName, CreateTime: startTimestamp}
	cursor, err := commonmongodb.NewTaskColl().ListByCursor(option)
	if err != nil {
		return buildStats, fmt.Errorf("pipeline list err:%v", err)
	}
	taskDateMap := make(map[string][]*taskmodels.Task)
	for cursor.Next(context.Background()) {
		var workflowTask *taskmodels.Task
		if err := cursor.Decode(&workflowTask); err != nil {
			return buildStats, fmt.Errorf("decode task err:%v", err)
		}
		time := time.Unix(workflowTask.CreateTime, 0)
		date := time.Format(config.Date)
		if _, isExist := taskDateMap[date]; isExist {
			taskDateMap[date] = append(taskDateMap[date], workflowTask)
		} else {
			tasks := make([]*taskmodels.Task, 0)
			tasks = append(tasks, workflowTask)
			taskDateMap[date] = tasks
		}
	}
	if len(taskDateMap) == 0 {
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

func GetworkflowV4BuildStatByProdutName(productName string, startTimestamp int64, log *zap.SugaredLogger) ([]*models.BuildStat, error) {
	buildStats := []*models.BuildStat{}
	option := &commonmongodb.ListWorkflowTaskV4Option{ProjectName: productName, CreateTime: startTimestamp}
	cursor, err := commonmongodb.NewworkflowTaskv4Coll().ListByCursor(option)
	if err != nil {
		return buildStats, fmt.Errorf("workflow v4 list err:%v", err)
	}
	taskDateMap := make(map[string][]*commonmodels.WorkflowTask)
	for cursor.Next(context.Background()) {
		var workflowTask *commonmodels.WorkflowTask
		if err := cursor.Decode(&workflowTask); err != nil {
			return buildStats, fmt.Errorf("decode task err:%v", err)
		}
		time := time.Unix(workflowTask.CreateTime, 0)
		date := time.Format(config.Date)
		if _, isExist := taskDateMap[date]; isExist {
			taskDateMap[date] = append(taskDateMap[date], workflowTask)
		} else {
			tasks := make([]*commonmodels.WorkflowTask, 0)
			tasks = append(tasks, workflowTask)
			taskDateMap[date] = tasks
		}
	}
	if len(taskDateMap) == 0 {
		localTime := time.Now().AddDate(0, 0, -1).In(time.Local)
		date := localTime.Format(config.Date)
		taskDateMap[date] = []*commonmodels.WorkflowTask{
			{Stages: []*commonmodels.StageTask{}},
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
					maxDurationPipeline.PipelineName = taskPreview.WorkflowName
					maxDurationPipeline.TaskID = taskPreview.TaskID
					maxDurationPipeline.MaxDuration = job.EndTime - job.StartTime
					maxDurationPipelines = append(maxDurationPipelines, maxDurationPipeline)

				}
			}
		}
		// get longgest duration.
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

func mergeBuildStat(buildStats []*models.BuildStat) []*models.BuildStat {
	resp := []*models.BuildStat{}
	buildMap := map[string][]*models.BuildStat{}
	for _, buildStat := range buildStats {
		key := strings.Join([]string{buildStat.ProductName, buildStat.Date}, "@")
		if _, isExist := buildMap[key]; isExist {
			buildMap[buildStat.Date] = append(buildMap[buildStat.Date], buildStat)
		} else {
			buildMap[buildStat.Date] = []*models.BuildStat{buildStat}
		}
	}
	for _, stats := range buildMap {
		buildStat := &models.BuildStat{}
		maxDurationPipelines := make([]*models.PipelineInfo, 0)
		for _, stat := range stats {
			buildStat.ProductName = stat.ProductName
			buildStat.Date = stat.Date
			buildStat.TotalSuccess += stat.TotalSuccess
			buildStat.TotalFailure += stat.TotalFailure
			buildStat.TotalTimeout += stat.TotalTimeout
			buildStat.TotalDuration += stat.TotalDuration
			buildStat.CreateTime = stat.CreateTime
			buildStat.UpdateTime = stat.UpdateTime
			maxDurationPipelines = append(maxDurationPipelines, stat.MaxDurationPipeline)
		}
		if len(maxDurationPipelines) > 0 {
			buildStat.TotalBuildCount = len(maxDurationPipelines)
			buildStat.MaxDuration = maxDurationPipelines[0].MaxDuration
			buildStat.MaxDurationPipeline = maxDurationPipelines[0]
		} else {
			buildStat.TotalBuildCount = 0
			buildStat.MaxDuration = 0
			buildStat.MaxDurationPipeline = &models.PipelineInfo{}
		}
		resp = append(resp, buildStat)
	}
	return resp
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
