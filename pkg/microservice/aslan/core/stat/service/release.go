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
	"time"

	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
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
