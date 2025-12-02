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

package service

import (
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/stat/repository/models"
	"github.com/koderover/zadig/v2/pkg/util"
)

type OpenAPIStatV2 struct {
	Total        int64        `json:"total"`
	SuccessCount int64        `json:"success_count"`
	DailyStat    []*DailyStat `json:"daily_stat"`
}

type DailyStat struct {
	Date         string `json:"date"`
	Total        int64  `json:"total"`
	SuccessCount int64  `json:"success_count"`
	FailCount    int64  `json:"fail_count"`
}

type StatDashboardConfig struct {
	Type      string     `json:"type"`
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Source    string     `json:"source"`
	APIConfig *APIConfig `json:"api_config,omitempty"`
	Function  string     `json:"function"`
	Weight    int64      `json:"weight"`
}

type APIConfig struct {
	ExternalSystemId string           `json:"external_system_id"`
	ApiPath          string           `json:"api_path"`
	Queries          []*util.KeyValue `json:"queries"`
}

type StatDashboardByProject struct {
	ProjectKey  string               `json:"project_key"`
	ProjectName string               `json:"project_name"`
	Score       float64              `json:"score"`
	Facts       []*StatDashboardItem `json:"facts"`
}

type StatDashboardItem struct {
	Type     string      `json:"type"`
	ID       string      `json:"id"`
	Data     interface{} `json:"data"`
	Score    float64     `json:"score"`
	Error    string      `json:"error,omitempty"`
	HasValue bool        `json:"has_value"`
}

type StatDashboardBasicData struct {
	BuildTotal              int64 `json:"build_total"`
	BuildSuccess            int64 `json:"build_success"`
	TestTotal               int64 `json:"test_total"`
	TestSuccess             int64 `json:"test_success"`
	DeployTotal             int64 `json:"deploy_total"`
	DeploySuccess           int64 `json:"deploy_success"`
	ProductionDeployTotal   int64 `json:"production_deploy_total"`
	ProductionDeploySuccess int64 `json:"production_deploy_success"`
}

type DeployHealthStat struct {
	Success int `json:"success"`
	Failure int `json:"failure"`
}

type DeployDashboard struct {
	Total         int                        `json:"total"`
	Success       int                        `json:"success"`
	WeeklyDeploys []*models.WeeklyDeployStat `json:"data"`
}

type ReleaseDashboard struct {
	Total           int                          `json:"total"`
	AverageDuration float64                      `json:"average_duration"`
	MonthlyRelease  []*models.MonthlyReleaseStat `json:"data"`
}

type RollbackTotalStat struct {
	Total int64 `json:"total"`
}
