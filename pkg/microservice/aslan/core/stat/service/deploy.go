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
	"sort"
	"time"

	"go.uber.org/zap"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	models "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/stat/repository/models"
	repo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/stat/repository/mongodb"
)

type dashboardDeploy struct {
	Total                 int                     `json:"total"`
	Success               int                     `json:"success"`
	DashboardDeployDailys []*dashboardDeployDaily `json:"data"`
}

type dashboardDeployDaily struct {
	Date    string `json:"date"`
	Success int    `json:"success"`
	Failure int    `json:"failure"`
	Total   int    `json:"total"`
}

func GetDeployDailyTotalAndSuccess(args *models.DeployStatOption, log *zap.SugaredLogger) (*dashboardDeploy, error) {
	var (
		dashboardDeploy       = new(dashboardDeploy)
		dashboardDeployDailys = make([]*dashboardDeployDaily, 0)
		failure               int
		success               int
	)

	if deployItems, err := repo.NewDeployStatColl().GetDeployTotalAndSuccess(); err == nil {
		for _, deployItem := range deployItems {
			success += deployItem.TotalSuccess
			failure += deployItem.TotalFailure
		}
		dashboardDeploy.Total = success + failure
		dashboardDeploy.Success = success
	} else {
		log.Errorf("Failed to getDeployTotalAndSuccess err:%s", err)
		return nil, err
	}

	if deployDailyItems, err := repo.NewDeployStatColl().GetDeployDailyTotal(args); err == nil {
		sort.SliceStable(deployDailyItems, func(i, j int) bool { return deployDailyItems[i].Date < deployDailyItems[j].Date })
		for _, deployDailyItem := range deployDailyItems {
			dashboardDeployDaily := new(dashboardDeployDaily)
			dashboardDeployDaily.Date = deployDailyItem.Date
			dashboardDeployDaily.Success = deployDailyItem.TotalSuccess
			dashboardDeployDaily.Failure = deployDailyItem.TotalFailure
			dashboardDeployDaily.Total = deployDailyItem.TotalFailure + deployDailyItem.TotalSuccess

			dashboardDeployDailys = append(dashboardDeployDailys, dashboardDeployDaily)
		}
		dashboardDeploy.DashboardDeployDailys = dashboardDeployDailys
	} else {
		log.Errorf("Failed to getDeployDailyTotal err:%s", err)
		return nil, err
	}

	return dashboardDeploy, nil
}

func GetDeployStats(args *models.DeployStatOption, log *zap.SugaredLogger) (*dashboardDeploy, error) {
	var (
		dashboardDeploy       = new(dashboardDeploy)
		dashboardDeployDailys = make([]*dashboardDeployDaily, 0)
		failure               int
		success               int
	)

	if deployItems, err := repo.NewDeployStatColl().GetDeployStats(&models.DeployStatOption{
		StartDate:    args.StartDate,
		EndDate:      args.EndDate,
		ProductNames: args.ProductNames,
	}); err == nil {
		for _, deployItem := range deployItems {
			success += deployItem.TotalSuccess
			failure += deployItem.TotalFailure
		}
		dashboardDeploy.Total = success + failure
		dashboardDeploy.Success = success
	} else {
		log.Errorf("Failed to getDeployTotalAndSuccess err:%s", err)
		return nil, err
	}

	if deployDailyItems, err := repo.NewDeployStatColl().GetDeployDailyTotal(args); err == nil {
		sort.SliceStable(deployDailyItems, func(i, j int) bool { return deployDailyItems[i].Date < deployDailyItems[j].Date })
		for _, deployDailyItem := range deployDailyItems {
			dashboardDeployDaily := new(dashboardDeployDaily)
			dashboardDeployDaily.Date = deployDailyItem.Date
			dashboardDeployDaily.Success = deployDailyItem.TotalSuccess
			dashboardDeployDaily.Failure = deployDailyItem.TotalFailure
			dashboardDeployDaily.Total = deployDailyItem.TotalFailure + deployDailyItem.TotalSuccess

			dashboardDeployDailys = append(dashboardDeployDailys, dashboardDeployDaily)
		}
		dashboardDeploy.DashboardDeployDailys = dashboardDeployDailys
	} else {
		log.Errorf("Failed to getDeployDailyTotal err:%s", err)
		return nil, err
	}

	return dashboardDeploy, nil
}

type ProjectsDeployStatTotal struct {
	ProjectName     string           `json:"project_name"`
	DeployStatTotal *DeployStatTotal `json:"deploy_stat_total"`
}

type DeployStatTotal struct {
	TotalSuccess int `json:"total_success"`
	TotalFailure int `json:"total_failure"`
	TotalTimeout int `json:"total_timeout"`
}

func GetDeployHealth(start, end int64, projects []string) ([]*ProjectsDeployStatTotal, error) {
	result, err := commonrepo.NewJobInfoColl().GetDeployTrend(start, end, projects)
	if err != nil {
		return nil, err
	}

	stats := make([]*ProjectsDeployStatTotal, 0)
	for _, project := range projects {
		deployStatTotal := &ProjectsDeployStatTotal{
			ProjectName:     project,
			DeployStatTotal: &DeployStatTotal{},
		}
		for _, item := range result {
			if item.ProductName == project {
				switch item.Status {
				case string(config.StatusPassed):
					deployStatTotal.DeployStatTotal.TotalSuccess++
				case string(config.StatusFailed):
					deployStatTotal.DeployStatTotal.TotalFailure++
				case string(config.StatusTimeout):
					deployStatTotal.DeployStatTotal.TotalTimeout++
				}
			}
		}
		stats = append(stats, deployStatTotal)
	}
	return stats, nil
}

type ProjectsWeeklyDeployStat struct {
	Project          string              `json:"project_name"`
	WeeklyDeployStat []*WeeklyDeployStat `json:"weekly_deploy_stat_data"`
}

type WeeklyDeployStat struct {
	WeekStartDate     string `json:"week_start_date"`
	Success           int    `json:"success_total"`
	Failure           int    `json:"failure_total"`
	Timeout           int    `json:"timeout_total"`
	AverageDeployTime int    `json:"average_deploy_time"`
}

func GetProjectsWeeklyDeployStat(start, end int64, projects []string) ([]*ProjectsWeeklyDeployStat, error) {
	result, err := commonrepo.NewJobInfoColl().GetDeployTrend(start, end, projects)
	if err != nil {
		return nil, err
	}

	stats := make([]*ProjectsWeeklyDeployStat, 0)
	for _, project := range projects {
		deployStat := &ProjectsWeeklyDeployStat{
			Project: project,
		}

		var start int64
		var end int64
		var stat *WeeklyDeployStat
		total, duration := 0, 0
		for i := 0; i < len(result); i++ {
			if result[i].ProductName != project {
				if result[i].StartTime >= end {
					if stat != nil {
						if total > 0 {
							stat.AverageDeployTime = duration / total
						}
						deployStat.WeeklyDeployStat = append(deployStat.WeeklyDeployStat, stat)
						stat, total, duration = nil, 0, 0
					}
				}
				continue
			}

			if start == 0 {
				start = result[i].StartTime
				end = time.Unix(start, 0).AddDate(0, 0, 7).Unix()
				stat = &WeeklyDeployStat{
					WeekStartDate: time.Unix(start, 0).Format("2006-01-02"),
				}
			} else {
				if result[i].StartTime >= end {
					if stat != nil {
						if total > 0 {
							stat.AverageDeployTime = duration / total
						}
						deployStat.WeeklyDeployStat = append(deployStat.WeeklyDeployStat, stat)
						stat, total, duration = nil, 0, 0
					}
					start = end
					stat = &WeeklyDeployStat{
						WeekStartDate: time.Unix(start, 0).Format("2006-01-02"),
					}
					end = time.Unix(start, 0).AddDate(0, 0, 7).Unix()
				}
			}

			if result[i].StartTime >= start && result[i].StartTime < end {
				switch result[i].Status {
				case string(config.StatusPassed):
					stat.Success++
				case string(config.StatusFailed):
					stat.Failure++
				case string(config.StatusTimeout):
					stat.Timeout++
				}
				total++
				duration += int(result[i].Duration)
			}
		}
		stats = append(stats, deployStat)
	}
	return stats, nil
}

type DeployStat struct {
	ProjectName string `json:"project_name"`
	Success     int    `json:"success"`
	Failure     int    `json:"failure"`
	Timeout     int    `json:"timeout"`
	Total       int    `json:"total"`
	Duration    int    `json:"duration"`
}

func GetProjectDeployStat(start, end int64, project string) (DeployStat, error) {
	result, err := commonrepo.NewJobInfoColl().GetDeployJobs(start, end, project)
	if err != nil {
		return DeployStat{}, err
	}

	resp := DeployStat{
		ProjectName: project,
	}
	for _, job := range result {
		switch job.Status {
		case string(config.StatusPassed):
			resp.Success++
		case string(config.StatusFailed):
			resp.Failure++
		}
		resp.Duration += int(job.Duration)
	}
	resp.Total = len(result)
	return resp, nil
}
