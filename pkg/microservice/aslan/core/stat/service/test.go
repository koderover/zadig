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

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
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
	WeekStartDay    string `json:"week_start_day"`
	Success         int    `json:"success_total"`
	Failure         int    `json:"failure_total"`
	Timeout         int    `json:"timeout_total"`
	AverageTestTime int    `json:"average_test_time"`
}

func GetWeeklyTestStatus(start, end int64, projects []string) ([]*ProjectsWeeklyTestStat, error) {
	result, err := commonrepo.NewJobInfoColl().GetTestTrend(start, end, projects)
	if err != nil {
		return nil, err
	}

	resp := make([]*ProjectsWeeklyTestStat, 0)
	for _, project := range projects {
		weeklyTestStat := &ProjectsWeeklyTestStat{
			Project:        project,
			WeeklyTestStat: make([]*WeeklyTestStat, 0),
		}

		var end int64
		var start int64
		var stat *WeeklyTestStat
		total, duration := 0, 0
		for i := 0; i < len(result); i++ {
			if result[i].ProductName != project {
				if result[i].StartTime >= end {
					if stat != nil {
						if total > 0 {
							stat.AverageTestTime = duration / total
						}
						weeklyTestStat.WeeklyTestStat = append(weeklyTestStat.WeeklyTestStat, stat)
						stat, total, duration = nil, 0, 0
					}
				}
				continue
			}

			if start == 0 {
				start = result[i].StartTime
				end = time.Unix(start, 0).AddDate(0, 0, 7).Unix()
				stat = &WeeklyTestStat{
					WeekStartDay: time.Unix(start, 0).Format("2006-01-02"),
				}
			} else {
				if result[i].StartTime >= end {
					if stat != nil {
						if total > 0 {
							stat.AverageTestTime = duration / total
						}
						weeklyTestStat.WeeklyTestStat = append(weeklyTestStat.WeeklyTestStat, stat)
						stat, total, duration = nil, 0, 0
					}
					start = end
					stat = &WeeklyTestStat{
						WeekStartDay: time.Unix(start, 0).Format("2006-01-02"),
					}
					end = time.Unix(start, 0).AddDate(0, 0, 7).Unix()
				}
			}

			if result[i].StartTime >= start && result[i].StartTime < end {
				switch result[i].Status {
				case string(config.StatusPassed):
					stat.Success++
				case string(config.StatusFailed):
					stat.Failure++
				case string(config.StatusTimeout):
					stat.Timeout++
				}
				total++
				duration += int(result[i].Duration)
			}
		}
		resp = append(resp, weeklyTestStat)
	}
	return resp, nil
}

type ProjectsDailyTestStat struct {
	Project       string           `json:"project"`
	DailyTestStat []*DailyTestStat `json:"daily_test_stat"`
}

type DailyTestStat struct {
	DayStartTime    string `json:"day_start_time"`
	Success         int    `json:"test_success_total"`
	Failure         int    `json:"test_failure_total"`
	Timeout         int    `json:"test_timeout_total"`
	AverageTestTime int    `json:"average_test_time"`
}

func GetDailyTestStatus(start, end int64, projects []string) ([]*ProjectsDailyTestStat, error) {
	result, err := commonrepo.NewJobInfoColl().GetTestTrend(start, end, projects)
	if err != nil {
		return nil, err
	}

	resp := make([]*ProjectsDailyTestStat, 0)
	for _, project := range projects {
		dailyTestStat := &ProjectsDailyTestStat{
			Project:       project,
			DailyTestStat: make([]*DailyTestStat, 0),
		}
		var end int64
		var start int64
		var stat *DailyTestStat
		total, duration := 0, 0
		for i := 0; i < len(result); i++ {
			if result[i].ProductName != project {
				if result[i].StartTime >= end {
					if stat != nil {
						if total > 0 {
							stat.AverageTestTime = duration / total
						}
						dailyTestStat.DailyTestStat = append(dailyTestStat.DailyTestStat, stat)
						stat, total, duration = nil, 0, 0
					}
				}
				continue
			}

			if start == 0 {
				start = result[i].StartTime
				end = time.Unix(start, 0).AddDate(0, 0, 1).Unix()
				stat = &DailyTestStat{
					DayStartTime: time.Unix(start, 0).Format("2006-01-02"),
				}
			} else {
				if result[i].StartTime >= end {
					if stat != nil {
						if total > 0 {
							stat.AverageTestTime = duration / total
						}
						dailyTestStat.DailyTestStat = append(dailyTestStat.DailyTestStat, stat)
						stat, total, duration = nil, 0, 0
					}
					start = end
					stat = &DailyTestStat{
						DayStartTime: time.Unix(start, 0).Format("2006-01-02"),
					}
					end = time.Unix(start, 0).AddDate(0, 0, 1).Unix()
				}
			}

			if result[i].StartTime >= start && result[i].StartTime < end {
				switch result[i].Status {
				case string(config.StatusPassed):
					stat.Success++
				case string(config.StatusFailed):
					stat.Failure++
				case string(config.StatusTimeout):
					stat.Timeout++
				}
				total++
				duration += int(result[i].Duration)
			}
		}
		resp = append(resp, dailyTestStat)
	}
	return resp, nil
}

type TestStat struct {
	ProjectName string `json:"project_name"`
	Success     int    `json:"success"`
	Failure     int    `json:"failure"`
	Timeout     int    `json:"timeout"`
	Total       int    `json:"total"`
	Duration    int    `json:"duration"`
}

func GetProjectTestStat(start, end int64, project string) (TestStat, error) {
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
		case string(config.StatusTimeout):
			resp.Timeout++
		}
		resp.Duration += int(job.Duration)
	}
	resp.Total = len(result)
	return resp, nil
}
