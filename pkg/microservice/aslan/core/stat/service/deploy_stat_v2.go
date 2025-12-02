/*
Copyright 2024 The KodeRover Authors.

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
	"context"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	templaterepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb/template"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/stat/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/stat/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/util"
)

// CreateWeeklyDeployStat creates deploy stats for both testing and production envs. Note that this function MUST be
// called in Monday otherwise it will CREATE STATS FOR INVALID DATE MAKING THE WHOLE STATS UNUSABLE.
func CreateWeeklyDeployStat(log *zap.SugaredLogger) error {
	projects, err := templaterepo.NewProductColl().List()
	if err != nil {
		log.Errorf("failed to list project list to create deploy stats, error: %s", err)
		return fmt.Errorf("failed to list project list to create deploy stats")
	}

	failedProjects := make([]string, 0)
	for _, project := range projects {
		testDeployStat, productionDeployStat, err := generateWeeklyDeployStatByProduct(project.ProductName, log)
		if err != nil {
			log.Errorf("failed to generate weekly deploy stat for project: %s, error: %s", project.ProjectName, err)
			// in order to minimize the damage dealt from unexpected errors, the error will not be returned immediately, the failed project list will be returned in final error
			failedProjects = append(failedProjects, project.ProductName)
			continue
		}

		inserted := false
		err = mongodb.NewWeeklyDeployStatColl().Upsert(testDeployStat)
		if err != nil {
			log.Errorf("failed to insert weekly deploy stat for project: %s, error: %s", project.ProjectName, err)
			// in order to minimize the damage dealt from unexpected errors, the error will not be returned immediately, the failed project list will be returned in final error
			failedProjects = append(failedProjects, project.ProductName)
			inserted = true
		}

		err = mongodb.NewWeeklyDeployStatColl().Upsert(productionDeployStat)
		if err != nil {
			log.Errorf("failed to insert weekly production deploy stat for project: %s, error: %s", project.ProjectName, err)
			// in order to minimize the damage dealt from unexpected errors, the error will not be returned immediately, the failed project list will be returned in final error
			if !inserted {
				failedProjects = append(failedProjects, project.ProductName)
			}
		}
	}

	if len(failedProjects) > 0 {
		return fmt.Errorf("failed to do full deploy stats, failed projects: %s", strings.Join(failedProjects, ", "))
	}

	return nil
}

// CreateMonthlyDeployStat creates deploy stats for both testing and production envs.
func CreateMonthlyDeployStat(log *zap.SugaredLogger) error {
	projects, err := templaterepo.NewProductColl().List()
	if err != nil {
		log.Errorf("failed to list project list to create deploy stats, error: %s", err)
		return fmt.Errorf("failed to list project list to create deploy stats")
	}

	failedProjects := make([]string, 0)
	for _, project := range projects {
		testDeployStat, productionDeployStat, err := generateMonthlyDeployStatByProduct(project.ProductName, log)
		if err != nil {
			log.Errorf("failed to generate weekly deploy stat for project: %s, error: %s", project.ProjectName, err)
			// in order to minimize the damage dealt from unexpected errors, the error will not be returned immediately, the failed project list will be returned in final error
			failedProjects = append(failedProjects, project.ProductName)
			continue
		}

		inserted := false
		err = mongodb.NewMonthlyDeployStatColl().Upsert(testDeployStat)
		if err != nil {
			log.Errorf("failed to insert weekly deploy stat for project: %s, error: %s", project.ProjectName, err)
			// in order to minimize the damage dealt from unexpected errors, the error will not be returned immediately, the failed project list will be returned in final error
			failedProjects = append(failedProjects, project.ProductName)
			inserted = true
		}

		err = mongodb.NewMonthlyDeployStatColl().Upsert(productionDeployStat)
		if err != nil {
			log.Errorf("failed to insert weekly production deploy stat for project: %s, error: %s", project.ProjectName, err)
			// in order to minimize the damage dealt from unexpected errors, the error will not be returned immediately, the failed project list will be returned in final error
			if !inserted {
				failedProjects = append(failedProjects, project.ProductName)
			}
		}
	}

	if len(failedProjects) > 0 {
		return fmt.Errorf("failed to do full deploy stats, failed projects: %s", strings.Join(failedProjects, ", "))
	}

	return nil
}

func GetDeployHeathStats(startTime, endTime int64, projects []string, production config.ProductionType, log *zap.SugaredLogger) (*DeployHealthStat, error) {
	deployJobs, err := commonrepo.NewJobInfoColl().GetDeployJobs(startTime, endTime, projects, production)
	if err != nil {
		log.Errorf("failed to get deploy jobs to calculate health stats, error: %s", err)
		return nil, fmt.Errorf("failed to get deploy jobs to calculate health stats, error: %s", err)
	}

	var (
		success = 0
		failure = 0
	)
	for _, job := range deployJobs {
		switch job.Status {
		case string(config.StatusPassed):
			success++
		case string(config.StatusFailed):
			failure++
		case string(config.StatusTimeout):
			failure++
		}
	}

	resp := &DeployHealthStat{
		Success: success,
		Failure: failure,
	}

	return resp, nil
}

func GetTopDeployedService(startTime, endTime int64, projects []string, top int, production config.ProductionType, log *zap.SugaredLogger) ([]*commonmodels.ServiceDeployCountWithStatus, error) {
	deployJobs, err := commonrepo.NewJobInfoColl().GetTopDeployedService(startTime, endTime, projects, production, top)
	if err != nil {
		log.Errorf("failed to get top deploy jobs, error: %s", err)
		return nil, fmt.Errorf("failed to get top deploy jobs, error: %s", err)
	}

	return deployJobs, nil
}

func GetTopDeployFailuresByService(startTime, endTime int64, projects []string, top int, production config.ProductionType, log *zap.SugaredLogger) ([]*commonmodels.ServiceDeployCountWithStatus, error) {
	deployJobs, err := commonrepo.NewJobInfoColl().GetTopDeployFailedService(startTime, endTime, projects, production, top)
	if err != nil {
		log.Errorf("failed to get top deploy jobs, error: %s", err)
		return nil, fmt.Errorf("failed to get top deploy jobs, error: %s", err)
	}

	return deployJobs, nil
}

func GetDeployWeeklyTrend(startTime, endTime int64, projects []string, production config.ProductionType, log *zap.SugaredLogger) ([]*models.WeeklyDeployStat, error) {
	// first get weekly stats
	weeklystats, err := mongodb.NewWeeklyDeployStatColl().CalculateStat(startTime, endTime, projects, production)
	if err != nil {
		log.Errorf("failed to get weekly deploy trend, error: %s", err)
		return nil, fmt.Errorf("failed to get weekly deploy trend, error: %s", err)
	}

	// then calculate the start time of this week, append it to the end of the array
	firstDayOfWeek := util.GetMonday(time.Now())

	allDeployJobs, err := commonrepo.NewJobInfoColl().GetDeployJobs(firstDayOfWeek.Unix(), time.Now().Unix(), projects, production)
	if err != nil {
		log.Errorf("failed to list deploy jobs for weeklytrend, error: %s", err)
		return nil, fmt.Errorf("failed to list deploy jobs for weeklytrend, error: %s", err)
	}

	var (
		testSuccess       = 0
		testFailed        = 0
		testTimeout       = 0
		productionSuccess = 0
		productionFailed  = 0
		productionTimeout = 0
	)

	// count the data for both production job
	for _, deployJob := range allDeployJobs {
		if deployJob.Production {
			switch deployJob.Status {
			case string(config.StatusPassed):
				productionSuccess++
			case string(config.StatusFailed):
				productionFailed++
			case string(config.StatusTimeout):
				productionTimeout++
			}
		} else {
			switch deployJob.Status {
			case string(config.StatusPassed):
				testSuccess++
			case string(config.StatusFailed):
				testFailed++
			case string(config.StatusTimeout):
				testTimeout++
			}
		}
	}

	date := firstDayOfWeek.Format(config.Date)

	switch production {
	case config.Production:
		weeklystats = append(weeklystats, &models.WeeklyDeployStat{
			Success: productionSuccess,
			Failed:  productionFailed,
			Timeout: productionTimeout,
			Date:    date,
		})
	case config.Testing:
		weeklystats = append(weeklystats, &models.WeeklyDeployStat{
			Success: testSuccess,
			Failed:  testFailed,
			Timeout: testTimeout,
			Date:    date,
		})
	case config.Both:
		weeklystats = append(weeklystats, &models.WeeklyDeployStat{
			Success: productionSuccess + testSuccess,
			Failed:  productionFailed + testFailed,
			Timeout: productionTimeout + testTimeout,
			Date:    date,
		})
	default:
		return nil, fmt.Errorf("invlid production type: %s", production)
	}

	return weeklystats, nil
}

func GetDeployMonthlyTrend(startTime, endTime int64, projects []string, production config.ProductionType, log *zap.SugaredLogger) ([]*models.WeeklyDeployStat, error) {
	// first get monthly stats
	monthlystats, err := mongodb.NewMonthlyDeployStatColl().CalculateStat(startTime, endTime, projects, production)
	if err != nil {
		log.Errorf("failed to get monthly deploy trend, error: %s", err)
		return nil, fmt.Errorf("failed to get monthly deploy trend, error: %s", err)
	}

	// then calculate the start time of this month, append it to the end of the array
	firstDayOfMonth := util.GetFirstOfMonthDay(time.Now())
	firstDayOfEndTimeMonth := util.GetFirstOfMonthDay(time.Unix(endTime, 0))
	if firstDayOfEndTimeMonth < firstDayOfMonth {
		return monthlystats, nil
	}

	allDeployJobs, err := commonrepo.NewJobInfoColl().GetDeployJobs(firstDayOfMonth, time.Now().Unix(), projects, production)
	if err != nil {
		log.Errorf("failed to list deploy jobs for monthly trend, error: %s", err)
		return nil, fmt.Errorf("failed to list deploy jobs for monthly trend, error: %s", err)
	}

	var (
		testSuccess        = 0
		testRollback       = 0
		testFailed         = 0
		testTimeout        = 0
		productionSuccess  = 0
		productionRollback = 0
		productionFailed   = 0
		productionTimeout  = 0
	)

	// count the data for both production job
	for _, deployJob := range allDeployJobs {
		if deployJob.Production {
			switch deployJob.Status {
			case string(config.StatusPassed):
				productionSuccess++
			case string(config.StatusFailed):
				productionFailed++
			case string(config.StatusTimeout):
				productionTimeout++
			}
		} else {
			switch deployJob.Status {
			case string(config.StatusPassed):
				testSuccess++
			case string(config.StatusFailed):
				testFailed++
			case string(config.StatusTimeout):
				testTimeout++
			}
		}
	}

	envInfos, _, err := commonrepo.NewEnvInfoColl().List(context.Background(), &commonrepo.ListEnvInfoOption{
		ProjectNames: projects,
		Operation:    config.EnvOperationRollback,
		StartTime:    firstDayOfMonth,
		EndTime:      time.Now().Unix(),
	})
	if err != nil {
		err = fmt.Errorf("failed to list env info for projects: %v, error: %s", projects, err)
		log.Error(err)
		return nil, err
	}

	for _, envInfo := range envInfos {
		if !envInfo.Production {
			testRollback++
		} else {
			productionRollback++
		}
	}

	date := time.Unix(firstDayOfMonth, 0).Format(config.Date)

	switch production {
	case config.Production:
		monthlystats = append(monthlystats, &models.WeeklyDeployStat{
			Production: true,
			Success:    productionSuccess,
			Rollback:   productionRollback,
			Failed:     productionFailed,
			Timeout:    productionTimeout,
			Date:       date,
		})
	case config.Testing:
		monthlystats = append(monthlystats, &models.WeeklyDeployStat{
			Production: false,
			Success:    testSuccess,
			Rollback:   testRollback,
			Failed:     testFailed,
			Timeout:    testTimeout,
			Date:       date,
		})
	case config.Both:
		monthlystats = append(monthlystats, &models.WeeklyDeployStat{
			Production: true,
			Success:    productionSuccess,
			Rollback:   productionRollback,
			Failed:     productionFailed,
			Timeout:    productionTimeout,
			Date:       date,
		})
		monthlystats = append(monthlystats, &models.WeeklyDeployStat{
			Production: false,
			Success:    testSuccess,
			Rollback:   testRollback,
			Failed:     testFailed,
			Timeout:    testTimeout,
			Date:       date,
		})
	default:
		return nil, fmt.Errorf("invlid production type: %s", production)
	}

	return monthlystats, nil
}

func GetDeployDashboard(startTime, endTime int64, projects []string, log *zap.SugaredLogger) (*DeployDashboard, error) {
	weeklyTrend, err := GetDeployWeeklyTrend(startTime, endTime, projects, config.Both, log)
	if err != nil {
		log.Errorf("failed to get weekly trend, error: %s", err)
		return nil, err
	}

	totalStats, err := commonrepo.NewJobInfoColl().GetDeployJobsStats(0, 0, projects, config.Both)
	if err != nil {
		log.Errorf("failed to get total deployment count, error: %s", err)
		return nil, err
	}

	var count, success int

	for _, totalStat := range totalStats {
		count += totalStat.Count
		success += totalStat.Success
	}

	resp := &DeployDashboard{
		Total:         count,
		Success:       success,
		WeeklyDeploys: weeklyTrend,
	}

	return resp, nil
}

// generateWeeklyDeployStatByProduct generates the deployment stats for the week before the calling time.
// (this should be called on monday)
func generateWeeklyDeployStatByProduct(projectKey string, log *zap.SugaredLogger) (testDeployStat, productionDeployStat *models.WeeklyDeployStat, retErr error) {
	testDeployStat = nil
	productionDeployStat = nil

	startTime := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 0, 0, 0, 0, time.Local).AddDate(0, 0, -7)
	endTime := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 23, 59, 59, 0, time.Local).AddDate(0, 0, -1)

	allDeployJobs, err := commonrepo.NewJobInfoColl().GetDeployJobs(startTime.Unix(), endTime.Unix(), []string{projectKey}, config.Both)
	if err != nil {
		log.Errorf("failed to list deploy jobs for product: %s, error: %s", projectKey, err)
		retErr = fmt.Errorf("failed to list deploy jobs for product: %s, error: %s", projectKey, err)
		return
	}

	var (
		testSuccess       = 0
		testFailed        = 0
		testTimeout       = 0
		productionSuccess = 0
		productionFailed  = 0
		productionTimeout = 0
	)

	// count the data for both production job
	for _, deployJob := range allDeployJobs {
		if deployJob.Production {
			switch deployJob.Status {
			case string(config.StatusPassed):
				productionSuccess++
			case string(config.StatusFailed):
				productionFailed++
			case string(config.StatusTimeout):
				productionTimeout++
			}
		} else {
			switch deployJob.Status {
			case string(config.StatusPassed):
				testSuccess++
			case string(config.StatusFailed):
				testFailed++
			case string(config.StatusTimeout):
				testTimeout++
			}
		}
	}

	date := startTime.Format(config.Date)

	testDeployStat = &models.WeeklyDeployStat{
		ProjectKey: projectKey,
		Production: false,
		Success:    testSuccess,
		Failed:     testFailed,
		Timeout:    testTimeout,
		Date:       date,
		CreateTime: endTime.Unix(),
	}

	productionDeployStat = &models.WeeklyDeployStat{
		ProjectKey: projectKey,
		Production: true,
		Success:    productionSuccess,
		Failed:     productionFailed,
		Timeout:    productionTimeout,
		Date:       date,
		CreateTime: endTime.Unix(),
	}

	return
}

// generateMonthlyDeployStatByProduct generates the deployment stats for the week before the calling time.
func generateMonthlyDeployStatByProduct(projectKey string, log *zap.SugaredLogger) (testDeployStat, productionDeployStat *models.WeeklyDeployStat, retErr error) {
	testDeployStat = nil
	productionDeployStat = nil

	startTime := time.Date(time.Now().Year(), time.Now().Month(), 1, 0, 0, 0, 0, time.Local).AddDate(0, -1, 0)
	endTime := startTime.AddDate(0, 1, 0).Add(-time.Second)

	allDeployJobs, err := commonrepo.NewJobInfoColl().GetDeployJobs(startTime.Unix(), endTime.Unix(), []string{projectKey}, config.Both)
	if err != nil {
		log.Errorf("failed to list deploy jobs for product: %s, error: %s", projectKey, err)
		retErr = fmt.Errorf("failed to list deploy jobs for product: %s, error: %s", projectKey, err)
		return
	}

	var (
		testSuccess       = 0
		testFailed        = 0
		testTimeout       = 0
		productionSuccess = 0
		productionFailed  = 0
		productionTimeout = 0
	)

	// count the data for both production job
	for _, deployJob := range allDeployJobs {
		if deployJob.Production {
			switch deployJob.Status {
			case string(config.StatusPassed):
				productionSuccess++
			case string(config.StatusFailed):
				productionFailed++
			case string(config.StatusTimeout):
				productionTimeout++
			}
		} else {
			switch deployJob.Status {
			case string(config.StatusPassed):
				testSuccess++
			case string(config.StatusFailed):
				testFailed++
			case string(config.StatusTimeout):
				testTimeout++
			}
		}
	}

	date := startTime.Format(config.Date)
	testDeployStat = &models.WeeklyDeployStat{
		ProjectKey: projectKey,
		Production: false,
		Success:    testSuccess,
		Failed:     testFailed,
		Timeout:    testTimeout,
		Date:       date,
		CreateTime: endTime.Unix(),
	}

	productionDeployStat = &models.WeeklyDeployStat{
		ProjectKey: projectKey,
		Production: true,
		Success:    productionSuccess,
		Failed:     productionFailed,
		Timeout:    productionTimeout,
		Date:       date,
		CreateTime: endTime.Unix(),
	}

	return
}
