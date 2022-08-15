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

	"go.uber.org/zap"

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
		StartDate: args.StartDate,
		EndDate:   args.EndDate,
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
