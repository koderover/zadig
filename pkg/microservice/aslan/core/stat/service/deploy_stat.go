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
	"sort"
	"time"

	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	taskmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/task"
	templaterepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb/template"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/base"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/stat/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/stat/repository/mongodb"
)

type serviceInfo struct {
	ServiceName   string
	DeploySuccess int
	DeployFailure int
}

type serviceTotalInfo struct {
	ServiceName        string
	DeployTotal        int
	DeployTotalFailure int
}

func InitDeployStat(log *zap.SugaredLogger) error {
	count, err := mongodb.NewDeployStatColl().FindCount()
	if err != nil {
		log.Errorf("deployStat FindCount err:%v", err)
		return fmt.Errorf("deployStat FindCount err:%v", err)
	}
	var createTime int64
	if count > 0 {
		createTime = time.Now().AddDate(0, 0, -1).Unix()
	}
	//获取所有的项目名称
	allProducts, err := templaterepo.NewProductColl().List()
	if err != nil {
		log.Errorf("deployStat ProductTmpl List err:%v", err)
		return fmt.Errorf("deployStat ProductTmpl List err:%v", err)
	}
	for _, product := range allProducts {
		deployStats, err := GetDeployStatByProdutName(product.ProductName, createTime, log)
		if err != nil {
			log.Errorf("list workkflow deploy stat err: %v", err)
			return fmt.Errorf("list workkflow deploy stat err: %v", err)
		}
		for _, deployStat := range deployStats {
			err := mongodb.NewDeployStatColl().Create(deployStat)
			if err != nil {
				err = mongodb.NewDeployStatColl().Update(deployStat)
				if err != nil {
					log.Errorf("deployStat Update err:%v", err)
					continue
				}
			}
		}
	}
	return nil
}

func GetDeployStatByProdutName(productName string, startTimestamp int64, log *zap.SugaredLogger) ([]*models.DeployStat, error) {
	deployStats := []*models.DeployStat{}
	taskDateMap, err := getTaskDateMap(productName, startTimestamp)
	if err != nil {
		return deployStats, err
	}

	taskDateKeys := make([]string, 0, len(taskDateMap))
	for taskDateMapKey := range taskDateMap {
		taskDateKeys = append(taskDateKeys, taskDateMapKey)
	}
	sort.Strings(taskDateKeys)

	for _, taskDate := range taskDateKeys {
		var (
			totalTaskSuccess           = 0
			totalTaskFailure           = 0
			totalDeploySuccess         = 0
			totalDeployFailure         = 0
			deployServiceInfos         = make([]*serviceInfo, 0)
			maxDeployFailureServiceMap = make(map[string][]*serviceInfo)
			serviceInfoTotals          = make([]*serviceTotalInfo, 0)
		)
		//循环task任务获取需要的数据
		for _, taskPreview := range taskDateMap[taskDate] {
			switch taskP := taskPreview.(type) {
			case *taskmodels.Task:
				stages := taskP.Stages
				taskStatus := taskP.Status
				switch taskStatus {
				case config.StatusPassed:
					totalTaskSuccess++
				case config.StatusFailed:
					totalTaskFailure++
				}
				for _, subStage := range stages {
					taskType := subStage.TaskType
					switch taskType {
					case config.TaskDeploy:
						for _, subTask := range subStage.SubTasks {
							deployInfo, err := base.ToDeployTask(subTask)
							serviceInfo := new(serviceInfo)
							serviceInfo.ServiceName = deployInfo.ServiceName

							if err != nil {
								log.Errorf("deployStat ToDeployTask err:%v", err)
								continue
							}

							if deployInfo.TaskStatus == config.StatusPassed {
								totalDeploySuccess++
								serviceInfo.DeploySuccess = 1
							} else if deployInfo.TaskStatus == config.StatusFailed {
								totalDeployFailure++
								serviceInfo.DeployFailure = 1
							} else {
								continue
							}
							deployServiceInfos = append(deployServiceInfos, serviceInfo)
						}
					}
				}
			case *commonmodels.WorkflowTask:
				stages := taskP.Stages
				for _, stage := range stages {
					for _, job := range stage.Jobs {
						switch job.JobType {
						case string(config.JobZadigDeploy):
							jobTaskSpec := &commonmodels.JobTaskDeploySpec{}
							if err := commonmodels.IToi(job.Spec, jobTaskSpec); err != nil {
								continue
							}
							serviceInfo := new(serviceInfo)
							serviceInfo.ServiceName = jobTaskSpec.ServiceName
							if job.Status == config.StatusPassed {
								totalDeploySuccess++
								serviceInfo.DeploySuccess = 1
							} else if job.Status == config.StatusFailed {
								totalDeployFailure++
								serviceInfo.DeployFailure = 1
							} else {
								continue
							}
							deployServiceInfos = append(deployServiceInfos, serviceInfo)
						case string(config.JobZadigHelmDeploy):
							jobTaskSpec := &commonmodels.JobTaskHelmDeploySpec{}
							if err := commonmodels.IToi(job.Spec, jobTaskSpec); err != nil {
								continue
							}
							serviceInfo := new(serviceInfo)
							serviceInfo.ServiceName = jobTaskSpec.ServiceName
							if job.Status == config.StatusPassed {
								totalDeploySuccess++
								serviceInfo.DeploySuccess = 1
							} else if job.Status == config.StatusFailed {
								totalDeployFailure++
								serviceInfo.DeployFailure = 1
							} else {
								continue
							}
							deployServiceInfos = append(deployServiceInfos, serviceInfo)
						}
					}
				}
			}
		}
		//以服务名称分组
		for _, svcInfo := range deployServiceInfos {
			if _, isExsit := maxDeployFailureServiceMap[svcInfo.ServiceName]; isExsit {
				maxDeployFailureServiceMap[svcInfo.ServiceName] = append(maxDeployFailureServiceMap[svcInfo.ServiceName], svcInfo)
			} else {
				serviceInfos := make([]*serviceInfo, 0)
				serviceInfos = append(serviceInfos, svcInfo)
				maxDeployFailureServiceMap[svcInfo.ServiceName] = serviceInfos
			}
		}
		//统计部署次数最高和失败最高的服务
		for serviceName, serviceInfos := range maxDeployFailureServiceMap {
			totalDeploy := 0
			totalFailure := 0
			for _, serviceInfo := range serviceInfos {
				totalFailure += serviceInfo.DeployFailure
				totalDeploy += totalFailure + serviceInfo.DeploySuccess
			}
			serviceTotalInfo := &serviceTotalInfo{
				ServiceName:        serviceName,
				DeployTotal:        totalDeploy,
				DeployTotalFailure: totalFailure,
			}
			serviceInfoTotals = append(serviceInfoTotals, serviceTotalInfo)
		}

		deployStat := new(models.DeployStat)
		deployStat.ProductName = productName
		deployStat.TotalTaskSuccess = totalTaskSuccess
		deployStat.TotalTaskFailure = totalTaskFailure
		deployStat.TotalDeploySuccess = totalDeploySuccess
		deployStat.TotalDeployFailure = totalDeployFailure
		deployStat.Date = taskDate
		tt, _ := time.ParseInLocation(config.Date, taskDate, time.Local)
		deployStat.CreateTime = tt.Unix()
		deployStat.UpdateTime = time.Now().Unix()
		if len(serviceInfoTotals) > 0 {
			sort.SliceStable(serviceInfoTotals, func(i, j int) bool { return serviceInfoTotals[i].DeployTotal > serviceInfoTotals[j].DeployTotal })
			deployStat.MaxDeployServiceNum = serviceInfoTotals[0].DeployTotal
			deployStat.MaxDeployServiceFailureNum = serviceInfoTotals[0].DeployTotalFailure
			deployStat.MaxDeployServiceName = serviceInfoTotals[0].ServiceName

			sort.SliceStable(serviceInfoTotals, func(i, j int) bool {
				return serviceInfoTotals[i].DeployTotalFailure > serviceInfoTotals[j].DeployTotalFailure
			})
			deployStat.MaxDeployFailureServiceNum = serviceInfoTotals[0].DeployTotalFailure
			deployStat.MaxDeployFailureServiceName = serviceInfoTotals[0].ServiceName
		} else {
			deployStat.MaxDeployServiceNum = 0
			deployStat.MaxDeployServiceFailureNum = 0
			deployStat.MaxDeployServiceName = ""
			deployStat.MaxDeployFailureServiceNum = 0
			deployStat.MaxDeployFailureServiceName = ""
		}
		deployStats = append(deployStats, deployStat)
	}
	return deployStats, nil
}

type deployStatTotal struct {
	TotalSuccess int `bson:"total_success"                 json:"totalSuccess"`
	TotalFailure int `bson:"total_failure"                 json:"totalFailure"`
}

func GetPipelineHealthMeasure(startDate, endDate int64, productNames []string, log *zap.SugaredLogger) (*deployStatTotal, error) {
	deployStats, err := mongodb.NewDeployStatColl().ListDeployStat(&models.DeployStatOption{StartDate: startDate, EndDate: endDate, IsAsc: true, ProductNames: productNames})
	if err != nil {
		log.Errorf("ListDeployStat err:%v", err)
		return nil, fmt.Errorf("ListDeployStat err:%v", err)
	}
	var (
		totalSuccess = 0
		totalFailure = 0
	)
	for _, deployStat := range deployStats {
		totalSuccess += deployStat.TotalTaskSuccess
		totalFailure += deployStat.TotalTaskFailure
	}
	deployStatTotal := &deployStatTotal{
		TotalSuccess: totalSuccess,
		TotalFailure: totalFailure,
	}
	return deployStatTotal, nil
}

func GetDeployHealthMeasure(startDate, endDate int64, productNames []string, log *zap.SugaredLogger) (*deployStatTotal, error) {
	deployStats, err := mongodb.NewDeployStatColl().ListDeployStat(&models.DeployStatOption{StartDate: startDate, EndDate: endDate, IsAsc: true, ProductNames: productNames})
	if err != nil {
		log.Errorf("ListDeployStat err:%v", err)
		return nil, fmt.Errorf("ListDeployStat err:%v", err)
	}
	var (
		totalSuccess = 0
		totalFailure = 0
	)
	for _, deployStat := range deployStats {
		totalSuccess += deployStat.TotalDeploySuccess
		totalFailure += deployStat.TotalDeployFailure
	}
	deployStatTotal := &deployStatTotal{
		TotalSuccess: totalSuccess,
		TotalFailure: totalFailure,
	}
	return deployStatTotal, nil
}

type deployStatWeekly struct {
	Day          int64 `bson:"day"                     json:"day"`
	TotalSuccess int   `bson:"total_success"           json:"totalSuccess"`
	TotalFailure int   `bson:"total_failure"           json:"totalFailure"`
}

func GetDeployWeeklyMeasure(startDate, endDate int64, productNames []string, log *zap.SugaredLogger) ([]*deployStatWeekly, error) {
	deployStats, err := mongodb.NewDeployStatColl().ListDeployStat(&models.DeployStatOption{StartDate: startDate, EndDate: endDate, IsAsc: true, ProductNames: productNames})
	if err != nil {
		log.Errorf("ListDeployStat err:%v", err)
		return nil, fmt.Errorf("ListDeployStat err:%v", err)
	}
	deployStatMap := make(map[string][]*models.DeployStat)
	for _, deployStat := range deployStats {
		if _, isExist := deployStatMap[deployStat.Date]; isExist {
			deployStatMap[deployStat.Date] = append(deployStatMap[deployStat.Date], deployStat)
		} else {
			tempDeployStats := make([]*models.DeployStat, 0)
			tempDeployStats = append(tempDeployStats, deployStat)
			deployStatMap[deployStat.Date] = tempDeployStats
		}
	}
	deployStatDateKeys := make([]string, 0, len(deployStatMap))
	for deployStatDateMapKey := range deployStatMap {
		deployStatDateKeys = append(deployStatDateKeys, deployStatDateMapKey)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(deployStatDateKeys)))

	var (
		totalSuccess = 0
		totalFailure = 0
	)
	deployStatWeeklys := make([]*deployStatWeekly, 0)
	if len(deployStatDateKeys) <= config.Day && len(deployStatDateKeys) > 0 {
		for _, deployStat := range deployStats {
			totalSuccess += deployStat.TotalDeploySuccess
			totalFailure += deployStat.TotalDeployFailure
		}
		deployStatWeekly := &deployStatWeekly{
			Day:          time.Now().Unix(),
			TotalSuccess: totalSuccess,
			TotalFailure: totalFailure,
		}
		deployStatWeeklys = append(deployStatWeeklys, deployStatWeekly)
	} else {
		for index, deployStatDate := range deployStatDateKeys {
			for _, deployStat := range deployStatMap[deployStatDate] {
				totalSuccess += deployStat.TotalDeploySuccess
				totalFailure += deployStat.TotalDeployFailure
			}

			if ((index + 1) % config.Day) == 0 {
				date, err := time.ParseInLocation(config.Date, deployStatDate, time.Local)
				if err != nil {
					log.Errorf("Failed to parse date: %s, the error is: %+v", deployStatDate, err)
					return nil, err
				}
				deployStatWeekly := &deployStatWeekly{
					Day:          date.Unix(),
					TotalSuccess: totalSuccess,
					TotalFailure: totalFailure,
				}
				deployStatWeeklys = append(deployStatWeeklys, deployStatWeekly)
				totalSuccess = 0
				totalFailure = 0
			}
		}
	}

	return deployStatWeeklys, nil
}

type deployHigherStat struct {
	ServiceName  string `bson:"service_name"            json:"serviceName"`
	TotalSuccess int    `bson:"total_success"           json:"totalSuccess"`
	TotalFailure int    `bson:"total_failure"           json:"totalFailure"`
}

func GetDeployTopFiveHigherMeasure(startDate, endDate int64, productNames []string, log *zap.SugaredLogger) ([]*deployHigherStat, error) {
	deployStats, err := mongodb.NewDeployStatColl().ListDeployStat(&models.DeployStatOption{StartDate: startDate, EndDate: endDate, IsAsc: true, ProductNames: productNames, Limit: 5, IsMaxDeploy: true})
	if err != nil {
		log.Errorf("ListDeployStat err:%v", err)
		return nil, fmt.Errorf("ListDeployStat err:%v", err)
	}
	deployHigherStats := make([]*deployHigherStat, 0)
	for _, deployStat := range deployStats {
		tempDeployStat, err := mongodb.NewDeployStatColl().Get(&mongodb.DeployStatGetOption{ServiceName: deployStat.MaxDeployServiceName, MaxDeployServiceNum: deployStat.MaxDeployServiceNum})
		if err != nil {
			log.Errorf("Get deployStat err:%v", err)
			continue
		}
		deployHigherStat := &deployHigherStat{
			ServiceName:  tempDeployStat.MaxDeployServiceName,
			TotalSuccess: tempDeployStat.MaxDeployServiceNum - tempDeployStat.MaxDeployServiceFailureNum,
			TotalFailure: tempDeployStat.MaxDeployServiceFailureNum,
		}
		if tempDeployStat.MaxDeployServiceName != "" {
			deployHigherStats = append(deployHigherStats, deployHigherStat)
		}
	}
	sort.SliceStable(deployHigherStats, func(i, j int) bool {
		return deployHigherStats[i].TotalSuccess > deployHigherStats[j].TotalSuccess
	})
	return deployHigherStats, nil
}

type deployFailureHigherStat struct {
	ProductName  string `bson:"product_name"  json:"productName"`
	ServiceName  string `bson:"service_name"  json:"serviceName"`
	TotalFailure int    `bson:"total_failure" json:"totalFailure"`
}

func GetDeployTopFiveFailureMeasure(startDate, endDate int64, productNames []string, log *zap.SugaredLogger) ([]*deployFailureHigherStat, error) {
	deployStats, err := mongodb.NewDeployStatColl().ListDeployStat(&models.DeployStatOption{StartDate: startDate, EndDate: endDate, IsAsc: true, ProductNames: productNames, Limit: 5})
	if err != nil {
		log.Errorf("ListDeployStat err:%v", err)
		return nil, fmt.Errorf("ListDeployStat err:%v", err)
	}
	deployFailureHigherStats := make([]*deployFailureHigherStat, 0)
	for _, deployStat := range deployStats {
		deployFailureHigherStat := &deployFailureHigherStat{
			ProductName:  deployStat.ProductName,
			ServiceName:  deployStat.MaxDeployFailureServiceName,
			TotalFailure: deployStat.MaxDeployFailureServiceNum,
		}
		if deployStat.MaxDeployFailureServiceName != "" {
			deployFailureHigherStats = append(deployFailureHigherStats, deployFailureHigherStat)
		}
	}

	sort.SliceStable(deployFailureHigherStats, func(i, j int) bool {
		return deployFailureHigherStats[i].TotalFailure > deployFailureHigherStats[j].TotalFailure
	})
	return deployFailureHigherStats, nil
}
