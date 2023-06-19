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

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/util"
	"go.uber.org/zap"

	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/stat/repository/models"
	repo "github.com/koderover/zadig/pkg/microservice/aslan/core/stat/repository/mongodb"
)

type dashboardBuild struct {
	Total                int                    `json:"total"`
	Success              int                    `json:"success"`
	DashboardBuildDailys []*dashboardBuildDaily `json:"data"`
}

type dashboardBuildDaily struct {
	Date    string `json:"date"`
	Success int    `json:"success"`
	Failure int    `json:"failure"`
	Total   int    `json:"total"`
}

func GetBuildTotalAndSuccess(args *models.BuildStatOption, log *zap.SugaredLogger) (*dashboardBuild, error) {
	var (
		dashboardBuild       = new(dashboardBuild)
		dashboardBuildDailys = make([]*dashboardBuildDaily, 0)
		total                = 0
		success              = 0
	)
	if buildItems, err := repo.NewBuildStatColl().GetBuildTotalAndSuccess(); err == nil {
		for _, buildItem := range buildItems {
			success += buildItem.TotalSuccess
			total += buildItem.TotalBuildCount
		}
		dashboardBuild.Success = success
		dashboardBuild.Total = total
	} else {
		log.Errorf("Failed to getBuildTotalAndSuccess err:%s", err)
		return nil, err
	}

	if buildDailyItems, err := repo.NewBuildStatColl().GetBuildDailyTotal(args); err == nil {
		sort.SliceStable(buildDailyItems, func(i, j int) bool { return buildDailyItems[i].Date < buildDailyItems[j].Date })
		for _, buildDailyItem := range buildDailyItems {
			dashboardBuildDaily := new(dashboardBuildDaily)
			dashboardBuildDaily.Date = buildDailyItem.Date
			dashboardBuildDaily.Success = buildDailyItem.TotalSuccess
			dashboardBuildDaily.Failure = buildDailyItem.TotalFailure
			dashboardBuildDaily.Total = buildDailyItem.TotalBuildCount

			dashboardBuildDailys = append(dashboardBuildDailys, dashboardBuildDaily)
		}
		dashboardBuild.DashboardBuildDailys = dashboardBuildDailys
	} else {
		log.Errorf("Failed to getDeployDailyTotal err:%s", err)
		return nil, err
	}
	return dashboardBuild, nil
}

func GetBuildStats(args *models.BuildStatOption, log *zap.SugaredLogger) (*dashboardBuild, error) {
	var (
		dashboardBuild       = new(dashboardBuild)
		dashboardBuildDailys = make([]*dashboardBuildDaily, 0)
	)
	if buildItem, err := repo.NewBuildStatColl().GetBuildStats(&models.BuildStatOption{
		StartDate:    args.StartDate,
		EndDate:      args.EndDate,
		ProductNames: args.ProductNames,
	}); err == nil {
		dashboardBuild.Success = buildItem.TotalSuccess
		dashboardBuild.Total = buildItem.TotalBuildCount
	} else {
		log.Errorf("Failed to getBuildTotalAndSuccess err:%s", err)
		return nil, err
	}

	if buildDailyItems, err := repo.NewBuildStatColl().GetBuildDailyTotal(args); err == nil {
		sort.SliceStable(buildDailyItems, func(i, j int) bool { return buildDailyItems[i].Date < buildDailyItems[j].Date })
		for _, buildDailyItem := range buildDailyItems {
			dashboardBuildDaily := new(dashboardBuildDaily)
			dashboardBuildDaily.Date = buildDailyItem.Date
			dashboardBuildDaily.Success = buildDailyItem.TotalSuccess
			dashboardBuildDaily.Failure = buildDailyItem.TotalFailure
			dashboardBuildDaily.Total = buildDailyItem.TotalBuildCount

			dashboardBuildDailys = append(dashboardBuildDailys, dashboardBuildDaily)
		}
		dashboardBuild.DashboardBuildDailys = dashboardBuildDailys
	} else {
		log.Errorf("Failed to getDeployDailyTotal err:%s", err)
		return nil, err
	}
	return dashboardBuild, nil
}

type WeeklyBuildStat struct {
	Project         string        `json:"project"`
	WeeklyBuildStat []*WeeklyStat `json:"weekly_build_stat"`
}

type WeeklyStat struct {
	StartTime        int64 `json:"start_time"`
	Success          int   `json:"success"`
	Failure          int   `json:"failure"`
	Timeout          int   `json:"timeout"`
	AverageBuildTime int   `json:"average_build_time"`
}

func GetWeeklyBuildStat(start, end int64, projects []string) ([]*WeeklyBuildStat, error) {
	result, err := commonrepo.NewJobInfoColl().GetBuildTrend(start, end, projects)
	if err != nil {
		return nil, err
	}

	resp := make([]*WeeklyBuildStat, 0)
	for _, project := range projects {
		buildStat := &WeeklyBuildStat{
			Project:         project,
			WeeklyBuildStat: make([]*WeeklyStat, 0),
		}

		for i := 0; i < len(result); i++ {
			if result[i].ProductName != project {
				continue
			}

			start := result[i].StartTime
			end := time.Unix(result[i].StartTime, 0).Add(time.Hour * 24 * 7).Unix()
			stat := &WeeklyStat{
				StartTime: start,
			}
			for j := i; j < len(result); j++ {
				if result[j].StartTime >= start && result[j].StartTime < end {
					switch result[j].Status {
					case string(config.StatusPassed):
						stat.Success++
					case string(config.StatusFailed):
						stat.Failure++
					case string(config.StatusTimeout):
						stat.Timeout++
					}
				} else {
					buildStat.WeeklyBuildStat = append(buildStat.WeeklyBuildStat, stat)
					i = j
					break
				}
			}
		}
		resp = append(resp, buildStat)
	}
	return resp, nil
}

type ProjectsBuildStatTotal struct {
	ProjectName    string          `json:"project_name"`
	BuildStatTotal *BuildStatTotal `json:"build_stat_total"`
}

type BuildStatTotal struct {
	TotalSuccess int `json:"total_success"`
	TotalFailure int `json:"total_failure"`
}

func GetBuildHealthMeasureV2(start, end int64, projects []string) ([]*ProjectsBuildStatTotal, error) {
	result, err := commonrepo.NewJobInfoColl().GetBuildTrend(start, end, projects)
	if err != nil {
		return nil, err
	}

	resp := make([]*ProjectsBuildStatTotal, 0)
	for _, project := range projects {
		buildStat := &ProjectsBuildStatTotal{
			ProjectName:    project,
			BuildStatTotal: &BuildStatTotal{},
		}
		for _, item := range result {
			if item.ProductName == project {
				if item.Status == string(config.StatusPassed) {
					buildStat.BuildStatTotal.TotalSuccess++
				} else if item.Status == string(config.StatusFailed) {
					buildStat.BuildStatTotal.TotalFailure++
				}
			}
		}
		resp = append(resp, buildStat)
	}
	return resp, nil
}

type ProjectDailyBuildStat struct {
	Project        string            `json:"project_name"`
	DailyBuildStat []*DailyBuildStat `json:"daily_build_stat"`
}

type DailyBuildStat struct {
	StartTime        int64 `json:"start_time"`
	Success          int   `json:"success"`
	Failure          int   `json:"failure"`
	Timeout          int   `json:"timeout"`
	AverageBuildTime int   `json:"average_build_time"`
}

func GetDailyBuildMeasure(start, end int64, projects []string) ([]*ProjectDailyBuildStat, error) {
	result, err := commonrepo.NewJobInfoColl().GetBuildTrend(start, end, projects)
	if err != nil {
		return nil, err
	}

	resp := make([]*ProjectDailyBuildStat, 0)
	for _, project := range projects {
		buildStat := &ProjectDailyBuildStat{
			Project:        project,
			DailyBuildStat: make([]*DailyBuildStat, 0),
		}

		for i := 0; i < len(result); i++ {
			if result[i].ProductName != project {
				continue
			}

			start := util.GetMidnightTimestamp(result[i].StartTime)
			end := time.Unix(start, 0).Add(time.Hour*24 - time.Second).Unix()
			stat := &DailyBuildStat{
				StartTime: start,
			}
			count, duration := 0, 0
			for j := i; j < len(result); j++ {
				if result[j].StartTime >= start && result[j].StartTime < end {
					switch result[j].Status {
					case string(config.StatusPassed):
						stat.Success++
					case string(config.StatusFailed):
						stat.Failure++
					case string(config.StatusTimeout):
						stat.Timeout++
					}
					count++
					duration += int(result[j].Duration)
				} else {
					stat.AverageBuildTime = duration / count
					buildStat.DailyBuildStat = append(buildStat.DailyBuildStat, stat)
					i = j
					break
				}
			}
		}
		resp = append(resp, buildStat)
	}
	return resp, nil
}

type BuildStat struct {
	ProjectName string `json:"project_name"`
	Success     int    `json:"success"`
	Failure     int    `json:"failure"`
	Timeout     int    `json:"timeout"`
	Total       int    `json:"total"`
}

func getBuildStat(start, end int64, project string) (BuildStat, error) {
	result, err := commonrepo.NewJobInfoColl().GetBuildTrend(start, end, []string{project})
	if err != nil {
		return BuildStat{}, err
	}

	resp := BuildStat{
		ProjectName: project,
	}
	for _, job := range result {
		switch job.Status {
		case string(config.StatusPassed):
			resp.Success++
		case string(config.StatusFailed):
			resp.Failure++
		}
	}
	resp.Total = len(result)
	return resp, nil
}
