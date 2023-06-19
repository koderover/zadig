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
	"time"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/util"
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

func GetTestDashboard(startTime, endTime int64, productName string, log *zap.SugaredLogger) (*testDashboard, error) {
	var (
		testDashboard  = new(testDashboard)
		existTestTasks = make([]*models.TestTaskStat, 0)
		totalCaseCount = 0
		totalSuccess   = 0
		totalFailure   = 0
		totalDuration  int64
	)

	tests, err := commonrepo.NewTestingColl().List(&commonrepo.ListTestOption{
		ProductName: productName,
	})
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

type ProjectsTestStatTotal struct {
	ProjectName   string         `json:"project_name"`
	TestStatTotal *TestStatTotal `json:"test_stat_total"`
}

type TestStatTotal struct {
	TotalSuccess int `json:"total_success"`
	TotalFailure int `json:"total_failure"`
	TotalTimeout int `json:"total_timeout"`
}

func GetTestHealth(start, end int64, projects []string) ([]*ProjectsTestStatTotal, error) {
	result, err := commonrepo.NewJobInfoColl().GetTestTrend(start, end, projects)
	if err != nil {
		return nil, err
	}

	stats := make([]*ProjectsTestStatTotal, 0)
	for _, project := range projects {
		testStatTotal := &ProjectsTestStatTotal{
			ProjectName:   project,
			TestStatTotal: &TestStatTotal{},
		}
		for _, item := range result {
			if item.ProductName == project {
				switch item.Status {
				case string(config.StatusPassed):
					testStatTotal.TestStatTotal.TotalSuccess++
				case string(config.StatusFailed):
					testStatTotal.TestStatTotal.TotalFailure++
				case string(config.StatusTimeout):
					testStatTotal.TestStatTotal.TotalTimeout++
				}
			}
		}
		stats = append(stats, testStatTotal)
	}
	return stats, nil
}

type ProjectsWeeklyTestStat struct {
	Project        string            `json:"project"`
	WeeklyTestStat []*WeeklyTestStat `json:"weekly_test_stat"`
}

type WeeklyTestStat struct {
	StartTime        string `json:"start_time"`
	Success          int    `json:"success"`
	Failure          int    `json:"failure"`
	Timeout          int    `json:"timeout"`
	AverageBuildTime int    `json:"average_test_time"`
}

func GetWeeklyTestStatus(start, end int64, projects []string) ([]*ProjectsWeeklyTestStat, error) {
	result, err := commonrepo.NewJobInfoColl().GetTestTrend(start, end, projects)
	if err != nil {
		return nil, err
	}

	stats := make([]*ProjectsWeeklyTestStat, 0)
	for _, project := range projects {
		weeklyTestStat := &ProjectsWeeklyTestStat{
			Project:        project,
			WeeklyTestStat: make([]*WeeklyTestStat, 0),
		}
		for i := 0; i < len(result); i++ {
			start := util.GetMidnightTimestamp(result[i].StartTime)
			end := time.Unix(start, 0).Add(time.Hour*24*7 - time.Second).Unix()
			weekStat := &WeeklyTestStat{StartTime: time.Unix(start, 0).Format("2006-01-02")}
			duration, count := 0, 0
			for j := 0; j < len(result); j++ {
				if result[i].ProductName == project && result[i].StartTime >= start && result[i].StartTime < end {
					switch result[i].Status {
					case string(config.StatusPassed):
						weekStat.Success++
					case string(config.StatusFailed):
						weekStat.Failure++
					case string(config.StatusTimeout):
						weekStat.Timeout++
					}
					duration += int(result[j].Duration)
					count++
				} else {
					weekStat.AverageBuildTime = duration / count
					weeklyTestStat.WeeklyTestStat = append(weeklyTestStat.WeeklyTestStat, weekStat)
					i = j
					break
				}
			}

		}
		stats = append(stats, weeklyTestStat)
	}
	return stats, nil
}

type ProjectsDailyTestStat struct {
	Project       string           `json:"project"`
	DailyTestStat []*DailyTestStat `json:"daily_test_stat"`
}

type DailyTestStat struct {
	StartTime        string `json:"start_time"`
	Success          int    `json:"success"`
	Failure          int    `json:"failure"`
	Timeout          int    `json:"timeout"`
	AverageBuildTime int    `json:"average_test_time"`
}

func GetDailyTestStatus(start, end int64, projects []string) ([]*ProjectsDailyTestStat, error) {
	result, err := commonrepo.NewJobInfoColl().GetTestTrend(start, end, projects)
	if err != nil {
		return nil, err
	}

	stats := make([]*ProjectsDailyTestStat, 0)
	for _, project := range projects {
		dailyTestStat := &ProjectsDailyTestStat{
			Project:       project,
			DailyTestStat: make([]*DailyTestStat, 0),
		}
		for i := 0; i < len(result); i++ {
			start := util.GetMidnightTimestamp(result[i].StartTime)
			end := time.Unix(start, 0).Add(time.Hour*24 - time.Second).Unix()
			dailyStat := &DailyTestStat{StartTime: time.Unix(start, 0).Format("2006-01-02")}
			duration, count := 0, 0
			for j := 0; j < len(result); j++ {
				if result[i].ProductName == project && result[i].StartTime >= start && result[i].StartTime < end {
					switch result[i].Status {
					case string(config.StatusPassed):
						dailyStat.Success++
					case string(config.StatusFailed):
						dailyStat.Failure++
					case string(config.StatusTimeout):
						dailyStat.Timeout++
					}
					duration += int(result[j].Duration)
					count++
				} else {
					dailyStat.AverageBuildTime = duration / count
					dailyTestStat.DailyTestStat = append(dailyTestStat.DailyTestStat, dailyStat)
					i = j
					break
				}
			}

		}
		stats = append(stats, dailyTestStat)
	}
	return stats, nil
}

type TestStat struct {
	ProjectName string `json:"project_name"`
	Success     int    `json:"success"`
	Failure     int    `json:"failure"`
	Timeout     int    `json:"timeout"`
	Total       int    `json:"total"`
}

func getTestStat(start, end int64, project string) (TestStat, error) {
	result, err := commonrepo.NewJobInfoColl().GetTestJobs(start, end, project)
	if err != nil {
		return TestStat{}, err
	}

	resp := TestStat{
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
