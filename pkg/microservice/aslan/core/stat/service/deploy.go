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

	models "github.com/koderover/zadig/pkg/microservice/aslan/core/stat/repository/models"
	repo "github.com/koderover/zadig/pkg/microservice/aslan/core/stat/repository/mongodb"
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
		StartDate: args.StartDate,
		EndDate:   args.EndDate,
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
