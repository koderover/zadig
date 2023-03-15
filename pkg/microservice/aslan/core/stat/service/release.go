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

	"go.uber.org/zap"

	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
)

func GetReleaseStatOpenAPI(startDate, endDate int64, productName string, log *zap.SugaredLogger) (interface{}, error) {
	stats, err := commonrepo.NewJobInfoColl().GetProductionDeployJobs(startDate, endDate, productName)
	if err != nil {
		log.Errorf("failed to get release statistics from mongodb, error: %s", err)
		return nil, errors.New("db error when getting release statistics")
	}

	resp := ReleaseStatToStatResp(stats)
	return resp, nil
}

func ReleaseStatToStatResp(stats *commonrepo.ProductionDeployJobStats) *OpenAPIStatV2 {
	resp := &OpenAPIStatV2{
		Total:        stats.Total,
		SuccessCount: stats.SuccessCount,
	}

	dailyStat := make([]*DailyStat, 0)
	for _, dailyInfo := range stats.DailyStat {
		dailyStat = append(dailyStat, &DailyStat{
			Date:         dailyInfo.Date,
			Total:        dailyInfo.Total,
			SuccessCount: dailyInfo.SuccessCount,
			FailCount:    dailyInfo.FailCount,
		})
	}
	resp.DailyStat = dailyStat
	return resp
}
