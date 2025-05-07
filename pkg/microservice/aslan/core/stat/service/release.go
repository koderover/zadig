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
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/stat/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/stat/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/util"
)

func GetReleaseStatOpenAPI(startDate, endDate int64, productName string, log *zap.SugaredLogger) (*OpenAPIStatV2, error) {
	jobList, err := commonrepo.NewJobInfoColl().GetProductionDeployJobs(startDate, endDate, productName)
	if err != nil {
		log.Errorf("failed to get release job list from mongodb, error: %s", err)
		return nil, errors.New("db error when getting release jobs")
	}

	resp := CalculateReleaseStatsFromJobList(jobList)
	return resp, nil
}

func CalculateReleaseStatsFromJobList(jobList []*commonmodels.JobInfo) *OpenAPIStatV2 {
	// first save all the jobs into a map with the date as the key
	dateJobMap := make(map[string][]*commonmodels.JobInfo)

	successCounter := 0

	for _, job := range jobList {
		date := time.Unix(job.StartTime, 0).Format("2006-01-02")
		if _, ok := dateJobMap[date]; !ok {
			dateJobMap[date] = make([]*commonmodels.JobInfo, 0)
		}
		dateJobMap[date] = append(dateJobMap[date], job)
		if job.Status == string(config.StatusPassed) {
			successCounter++
		}
	}

	dailyStat := make([]*DailyStat, 0)

	// then get the stats for each date by iterating the map
	for date, dateJobList := range dateJobMap {
		dailySuccessCounter := 0
		dailyFailCounter := 0
		for _, job := range dateJobList {
			if job.Status == string(config.StatusPassed) {
				dailySuccessCounter++
			}
			if job.Status == string(config.StatusTimeout) || job.Status == string(config.StatusFailed) {
				dailyFailCounter++
			}
		}
		dailyStat = append(dailyStat, &DailyStat{
			Date:         date,
			Total:        int64(len(dateJobList)),
			SuccessCount: int64(dailySuccessCounter),
			FailCount:    int64(dailyFailCounter),
		})
	}

	resp := new(OpenAPIStatV2)
	resp.Total = int64(len(jobList))
	resp.SuccessCount = int64(successCounter)
	resp.DailyStat = dailyStat
	return resp
}

type ReleaseStat struct {
	ProjectName string `json:"project_name"`
	Success     int    `json:"success"`
	Failure     int    `json:"failure"`
	Timeout     int    `json:"timeout"`
	Total       int    `json:"total"`
	Duration    int    `json:"duration"`
}

func GetProjectReleaseStat(start, end int64, project string) (ReleaseStat, error) {
	result, err := commonrepo.NewJobInfoColl().GetProductionDeployJobs(start, end, project)
	if err != nil {
		return ReleaseStat{}, err
	}

	resp := ReleaseStat{
		ProjectName: project,
	}
	for _, job := range result {
		switch job.Status {
		case string(config.StatusPassed):
			resp.Success++
		case string(config.StatusFailed):
			resp.Failure++
		case string(config.StatusTimeout):
			resp.Timeout++
		}
		resp.Duration += int(job.Duration)
	}
	resp.Total = len(result)
	return resp, nil
}

// CreateMonthlyReleaseStat creates stats for release plans.
func CreateMonthlyReleaseStat(log *zap.SugaredLogger) error {
	startTime := time.Date(time.Now().Year(), time.Now().Month(), 1, 0, 0, 0, 0, time.Local).AddDate(0, -1, 0)
	endTime := startTime.AddDate(0, 1, 0).Add(-time.Second)

	releasePlans, err := commonrepo.NewReleasePlanColl().ListFinishedReleasePlan(startTime.Unix(), endTime.Unix())
	if err != nil {
		log.Errorf("failed to list release plan to calculate the statistics, error: %s", err)
		return fmt.Errorf("failed to list release plan to calculate the statistics, error: %s", err)
	}

	var stat *models.MonthlyReleaseStat

	if len(releasePlans) == 0 {
		stat = &models.MonthlyReleaseStat{
			Total:                    0,
			AverageExecutionDuration: 0,
			AverageApprovalDuration:  0,
			Date:                     startTime.Format(config.Date),
			CreateTime:               time.Now().Unix(),
			UpdateTime:               0,
		}
	} else {
		var executionDuration int64
		var approvalDuration int64

		for _, releasePlan := range releasePlans {
			// compatibility code, some executing time is 0 due to fixed bug
			if releasePlan.ExecutingTime != 0 {
				executionDuration += releasePlan.SuccessTime - releasePlan.ExecutingTime
			}
			if releasePlan.ApprovalTime != 0 {
				approvalDuration += releasePlan.ExecutingTime - releasePlan.ApprovalTime
			}
		}

		var averageExecutionDuration, averageApprovalDuration float64

		averageApprovalDuration = float64(approvalDuration) / float64(len(releasePlans))
		averageExecutionDuration = float64(executionDuration) / float64(len(releasePlans))

		stat = &models.MonthlyReleaseStat{
			Total:                    len(releasePlans),
			AverageExecutionDuration: averageExecutionDuration,
			AverageApprovalDuration:  averageApprovalDuration,
			Date:                     startTime.Format(config.Date),
			CreateTime:               time.Now().Unix(),
			UpdateTime:               0,
		}
	}

	err = mongodb.NewMonthlyReleaseStatColl().Upsert(stat)
	if err != nil {
		log.Errorf("failed to save monthly release data into mongodb for date: %s, error: %s", startTime.Format(config.Date), err)
		return fmt.Errorf("failed to save monthly release data into mongodb for date: %s, error: %s", startTime.Format(config.Date), err)
	}

	return nil
}

func GetReleaseDashboard(startTime, endTime int64, log *zap.SugaredLogger) (*ReleaseDashboard, error) {
	monthlyTrend, err := GetReleaseMonthlyTrend(startTime, endTime, log)
	if err != nil {
		log.Errorf("failed to get weekly trend, error: %s", err)
		return nil, err
	}

	resp := &ReleaseDashboard{
		MonthlyRelease: monthlyTrend,
	}

	releasePlans, err := commonrepo.NewReleasePlanColl().ListFinishedReleasePlan(0, 0)
	if err != nil {
		log.Errorf("failed to list release plan to calculate the statistics, error: %s", err)
		return nil, fmt.Errorf("failed to list release plan to calculate the statistics, error: %s", err)
	}

	if len(releasePlans) == 0 {
		resp.AverageDuration = 0
		resp.Total = 0
	} else {
		var executionDuration int64

		for _, releasePlan := range releasePlans {
			executionDuration += releasePlan.SuccessTime - releasePlan.ExecutingTime
		}

		var averageExecutionDuration float64

		averageExecutionDuration = float64(executionDuration) / float64(len(releasePlans))

		resp.AverageDuration = averageExecutionDuration
		resp.Total = len(releasePlans)
	}

	return resp, nil
}

func GetReleaseMonthlyTrend(startTime, endTime int64, log *zap.SugaredLogger) ([]*models.MonthlyReleaseStat, error) {
	// first get weekly stats
	monthlyStats, err := mongodb.NewMonthlyReleaseStatColl().List(startTime, endTime)
	if err != nil {
		log.Errorf("failed to get weekly deploy trend, error: %s", err)
		return nil, fmt.Errorf("failed to get weekly deploy trend, error: %s", err)
	}

	// then calculate the start time of this week, append it to the end of the array
	firstDayOfMonth := util.GetFirstOfMonthDay(time.Now())

	releasePlans, err := commonrepo.NewReleasePlanColl().ListFinishedReleasePlan(firstDayOfMonth, time.Now().Unix())
	if err != nil {
		log.Errorf("failed to list release plans to calculate current month stats, error: %s", err)
		return nil, fmt.Errorf("failed to list release plans to calculate current month stats, error: %s", err)
	}

	var stat *models.MonthlyReleaseStat

	date := time.Unix(firstDayOfMonth, 0).Format(config.Date)

	if len(releasePlans) == 0 {
		stat = &models.MonthlyReleaseStat{
			Total:                    0,
			AverageExecutionDuration: 0,
			AverageApprovalDuration:  0,
			Date:                     date,
			CreateTime:               time.Now().Unix(),
			UpdateTime:               0,
		}
	} else {
		var executionDuration int64
		var approvalDuration int64

		for _, releasePlan := range releasePlans {
			executionDuration += releasePlan.SuccessTime - releasePlan.ExecutingTime
			if releasePlan.ApprovalTime != 0 {
				approvalDuration += releasePlan.ExecutingTime - releasePlan.ApprovalTime
			}
		}

		var averageExecutionDuration, averageApprovalDuration float64

		averageApprovalDuration = float64(approvalDuration) / float64(len(releasePlans))
		averageExecutionDuration = float64(executionDuration) / float64(len(releasePlans))

		stat = &models.MonthlyReleaseStat{
			Total:                    len(releasePlans),
			AverageExecutionDuration: averageExecutionDuration,
			AverageApprovalDuration:  averageApprovalDuration,
			Date:                     date,
			CreateTime:               time.Now().Unix(),
			UpdateTime:               0,
		}
	}

	monthlyStats = append(monthlyStats, stat)

	return monthlyStats, nil
}
