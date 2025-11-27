/*
Copyright 2025 The KodeRover Authors.

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

	"go.uber.org/zap"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/stat/repository/mongodb"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

func GetTestCount(projects []string, logger *zap.SugaredLogger) (int, error) {
	// if projects is empty, get all testings
	if len(projects) == 0 {
		testings, err := commonrepo.NewTestingColl().List(&commonrepo.ListTestOption{})
		if err != nil {
			logger.Errorf("TestingModule.List error: %v", err)
			return 0, e.ErrListTestModule.AddDesc(err.Error())
		}
		return len(testings), nil
	}

	// otherwise, get testings for the specified projects
	testings, err := commonrepo.NewTestingColl().List(&commonrepo.ListTestOption{ProductNames: projects})
	if err != nil {
		logger.Errorf("TestingModule.List error: %v", err)
		return 0, e.ErrListTestModule.AddDesc(err.Error())
	}
	return len(testings), nil
}

type DailyTestHealthStat struct {
	Date         string  `json:"date"`
	TotalSuccess int     `json:"totalSuccess"`
	TotalFailure int     `json:"totalFailure"`
	SuccessRate  float64 `json:"successRate"`
}

func GetDailyTestHealthTrend(startDate, endDate int64, projects []string, logger *zap.SugaredLogger) ([]*DailyTestHealthStat, error) {
	// Get test stats from database
	testStats, err := mongodb.NewTestStatColl().ListTestStat(&mongodb.TestStatOption{
		StartDate:    startDate,
		EndDate:      endDate,
		IsAsc:        true,
		ProductNames: projects,
	})
	if err != nil {
		logger.Errorf("ListTestStat error: %v", err)
		return nil, fmt.Errorf("ListTestStat error: %v", err)
	}

	// Group by date
	dailyStatMap := make(map[string]*DailyTestHealthStat)
	for _, testStat := range testStats {
		dateKey := testStat.Date

		if _, exists := dailyStatMap[dateKey]; !exists {
			dailyStatMap[dateKey] = &DailyTestHealthStat{
				Date:         dateKey,
				TotalSuccess: 0,
				TotalFailure: 0,
			}
		}

		dailyStatMap[dateKey].TotalSuccess += testStat.TotalSuccess
		dailyStatMap[dateKey].TotalFailure += testStat.TotalFailure
	}

	// Convert map to sorted array
	dateKeys := make([]string, 0, len(dailyStatMap))
	for dateKey := range dailyStatMap {
		dateKeys = append(dateKeys, dateKey)
	}
	sort.Strings(dateKeys)

	result := make([]*DailyTestHealthStat, 0, len(dateKeys))
	for _, dateKey := range dateKeys {
		stat := dailyStatMap[dateKey]
		totalTests := stat.TotalSuccess + stat.TotalFailure
		if totalTests > 0 {
			stat.SuccessRate = float64(stat.TotalSuccess) / float64(totalTests) * 100
		}
		result = append(result, stat)
	}

	return result, nil
}

type RecentTestTask struct {
	TaskID       int64         `json:"task_id"`
	TaskCreator  string        `json:"task_creator"`
	ProductName  string        `json:"product_name"`
	TestName     string        `json:"test_name"`
	WorkflowName string        `json:"workflow_name"`
	Status       config.Status `json:"status"`
	CreateTime   int64         `json:"create_time"`
	StartTime    int64         `json:"start_time"`
	EndTime      int64         `json:"end_time"`
}

func GetRecentTestTask(projects []string, number int, logger *zap.SugaredLogger) ([]*RecentTestTask, error) {
	// Default to 10 if number is not specified or invalid
	if number <= 0 {
		number = 10
	}

	// Query workflow tasks filtered by testing type and projects at database level
	option := &commonrepo.ListWorkflowTaskV4Option{
		Type:   config.WorkflowTaskTypeTesting, // Filter by testing type in database
		Limit:  number,
		IsSort: true,
	}

	// If projects are specified, filter by them
	if len(projects) > 0 {
		option.ProjectNames = projects
	}

	workflowTasks, _, err := commonrepo.NewworkflowTaskv4Coll().List(option)
	if err != nil {
		logger.Errorf("failed to list workflow tasks, error: %s", err)
		return nil, fmt.Errorf("failed to list workflow tasks, error: %s", err)
	}

	// Build response
	recentTasks := make([]*RecentTestTask, 0, len(workflowTasks))
	for _, task := range workflowTasks {
		// Extract test name from workflow display name
		testName := task.WorkflowDisplayName
		if testName == "" && task.WorkflowArgs != nil {
			testName = task.WorkflowArgs.DisplayName
		}
		if testName == "" {
			testName = task.WorkflowName
		}

		recentTasks = append(recentTasks, &RecentTestTask{
			TaskID:       task.TaskID,
			TaskCreator:  task.TaskCreator,
			ProductName:  task.ProjectName,
			TestName:     testName,
			WorkflowName: task.WorkflowName,
			Status:       task.Status,
			CreateTime:   task.CreateTime,
			StartTime:    task.StartTime,
			EndTime:      task.EndTime,
		})
	}

	return recentTasks, nil
}
