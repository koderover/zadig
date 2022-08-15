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
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
)

type testDashboard struct {
	TotalCaseCount  int   `json:"total_case_count"`
	TotalExecCount  int   `json:"total_exec_count"`
	Success         int   `json:"success"`
	AverageDuration int64 `json:"average_duration"`
}

func GetTestDashboard(startTime, endTime int64, log *zap.SugaredLogger) (*testDashboard, error) {
	var (
		testDashboard  = new(testDashboard)
		existTestTasks = make([]*models.TestTaskStat, 0)
		totalCaseCount = 0
		totalSuccess   = 0
		totalFailure   = 0
		totalDuration  int64
	)

	tests, err := commonrepo.NewTestingColl().List(&commonrepo.ListTestOption{})
	if err != nil {
		log.Errorf("Failed to list testing err:%s", err)
		return nil, err
	}

	testNames := sets.String{}
	for _, test := range tests {
		testNames.Insert(test.Name)
	}
	testTasks, err := commonrepo.NewTestTaskStatColl().GetTestTasks(startTime, endTime)
	if err != nil {
		log.Errorf("Failed to list TestTaskStat err:%s", err)
		return nil, err
	}
	for _, testTask := range testTasks {
		if testNames.Has(testTask.Name) {
			existTestTasks = append(existTestTasks, testTask)
		}
	}

	for _, existTestTask := range existTestTasks {
		totalCaseCount += existTestTask.TestCaseNum
		totalSuccess += existTestTask.TotalSuccess
		totalFailure += existTestTask.TotalFailure
		totalDuration += existTestTask.TotalDuration
	}
	testDashboard.TotalCaseCount = totalCaseCount
	testDashboard.TotalExecCount = totalSuccess + totalFailure
	testDashboard.Success = totalSuccess
	if testDashboard.TotalExecCount == 0 {
		testDashboard.AverageDuration = 0
		return testDashboard, nil
	}
	testDashboard.AverageDuration = totalDuration / int64(testDashboard.TotalExecCount)

	return testDashboard, nil
}
