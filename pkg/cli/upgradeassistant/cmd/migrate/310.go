/*
 * Copyright 2024 The KodeRover Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package migrate

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/koderover/zadig/v2/pkg/cli/upgradeassistant/internal/upgradepath"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	templaterepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb/template"
	statmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/stat/repository/models"
	statrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/stat/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/types"
	"github.com/koderover/zadig/v2/pkg/util"
)

func init() {
	upgradepath.RegisterHandler("3.0.0", "3.1.0", V300ToV310)
	upgradepath.RegisterHandler("3.1.0", "3.0.0", V310ToV300)
}

func V300ToV310() error {
	log.Infof("-------- start migrate infrastructure filed in testing, scanning and sacnning template module --------")
	if err := migrateTestingAndScaningInfraField(); err != nil {
		log.Infof("migrate infrastructure filed in testing, scanning and sacnning template module job err: %v", err)
		return err
	}

	log.Infof("-------- start creating weekly and monthly deployment stats --------")
	if err := migrateDeploymentWeeklyAndMonthlyStats(); err != nil {
		log.Infof("migrate deployment weekly and monthly deployment stats err: %v", err)
		return err
	}

	log.Infof("-------- start creating monthly release stats --------")
	if err := migrateReleaseMonthlyStats(); err != nil {
		log.Infof("migrate deployment weekly and monthly deployment stats err: %v", err)
		return err
	}

	log.Infof("-------- start to migrate approval config for all workflows --------")
	if err := migrateApprovalForAllWorkflows(); err != nil {
		log.Infof("migrate: %v", err)
		return err
	}

	log.Infof("-------- start to migrate approval config for all user supplied workflow templates--------")
	if err := migrateApprovalForAllWorkflowTemplates(); err != nil {
		log.Infof("migrate: %v", err)
		return err
	}

	return nil
}

func V310ToV300() error {
	return nil
}

// migrateDeploymentWeeklyAndMonthlyStats only migrate the data generated less than a year ago
func migrateDeploymentWeeklyAndMonthlyStats() error {
	// find all projects to do the migration
	projects, err := templaterepo.NewProductColl().List()
	if err != nil {
		log.Errorf("failed to list project list to create deploy stats, error: %s", err)
		return fmt.Errorf("failed to list project list to create deploy stats")
	}

	now := time.Now()
	// rollback to a year ago
	yearAgo := now.AddDate(-1, 0, 0)
	// find the start of the first day of that month and that week
	startOfMonth := time.Date(yearAgo.Year(), yearAgo.Month(), 1, 0, 0, 0, 0, time.Local)
	endOfMonth := startOfMonth.AddDate(0, 1, 0).Add(-time.Second)
	startOfWeek := util.GetMonday(yearAgo)
	endOfWeek := startOfWeek.AddDate(0, 0, 7).Add(-time.Second)

	// first do the monthly migration. if the endOfMonth is greater than now, then there is nothing to be migrated
	for endOfMonth.Before(now) {
		for _, project := range projects {
			monthlyTestingDeployStat, monthlyProductionDeployStat, err := generateDeployStatByProduct(startOfMonth, endOfMonth, project.ProductName)
			if err != nil {
				return err
			}
			monthlyTestingDeployStat.CreateTime = startOfMonth.Unix()
			monthlyProductionDeployStat.CreateTime = startOfMonth.Unix()
			err = statrepo.NewMonthlyDeployStatColl().Upsert(monthlyTestingDeployStat)
			if err != nil {
				log.Errorf("failed to create monthly deployment stat for testing env for date: %s, project: %s, err: %s", startOfMonth.Format(config.Date), project.ProductName, err)
				return err
			}
			err = statrepo.NewMonthlyDeployStatColl().Upsert(monthlyProductionDeployStat)
			if err != nil {
				log.Errorf("failed to create monthly deployment stat for production env for date: %s, project: %s, err: %s", startOfMonth.Format(config.Date), project.ProductName, err)
				return err
			}
		}

		startOfMonth = startOfMonth.AddDate(0, 1, 0)
		endOfMonth = startOfMonth.AddDate(0, 1, 0).Add(-time.Second)
	}

	for endOfWeek.Before(now) {
		for _, project := range projects {
			weeklyTestingDeployStat, weeklyProductionDeployStat, err := generateDeployStatByProduct(startOfMonth, endOfMonth, project.ProductName)
			if err != nil {
				return err
			}
			weeklyTestingDeployStat.CreateTime = startOfWeek.Unix()
			weeklyProductionDeployStat.CreateTime = startOfWeek.Unix()
			err = statrepo.NewWeeklyDeployStatColl().Upsert(weeklyTestingDeployStat)
			if err != nil {
				log.Errorf("failed to create weekly deployment stat for testing env for date: %s, project: %s, err: %s", startOfMonth.Format(config.Date), project.ProductName, err)
				return err
			}
			err = statrepo.NewWeeklyDeployStatColl().Upsert(weeklyProductionDeployStat)
			if err != nil {
				log.Errorf("failed to create weekly deployment stat for production env for date: %s, project: %s, err: %s", startOfMonth.Format(config.Date), project.ProductName, err)
				return err
			}
		}

		startOfWeek = startOfWeek.AddDate(0, 0, 7)
		endOfWeek = startOfWeek.AddDate(0, 0, 7).Add(-time.Second)
	}

	return nil
}

// migrateReleaseMonthlyStats only migrate the data generated less than a year ago
func migrateReleaseMonthlyStats() error {
	now := time.Now()
	// rollback to a year ago
	yearAgo := now.AddDate(-1, 0, 0)
	// find the start of the first day of that month and that week
	startOfMonth := time.Date(yearAgo.Year(), yearAgo.Month(), 1, 0, 0, 0, 0, time.Local)
	endOfMonth := startOfMonth.AddDate(0, 1, 0).Add(-time.Second)

	// first do the monthly migration. if the endOfMonth is greater than now, then there is nothing to be migrated
	for endOfMonth.Before(now) {
		monthlyReleaseStat, err := generateReleaseStat(startOfMonth, endOfMonth)
		if err != nil {
			return err
		}

		err = statrepo.NewMonthlyReleaseStatColl().Upsert(monthlyReleaseStat)
		if err != nil {
			log.Errorf("failed to create monthly release stat date: %s, err: %s", startOfMonth.Format(config.Date), err)
			return err
		}

		startOfMonth = startOfMonth.AddDate(0, 1, 0)
		endOfMonth = startOfMonth.AddDate(0, 1, 0).Add(-time.Second)
	}

	return nil
}

// generateDeployStatByProduct generates the deployment stats counting from startTime to endTime, and marks the date of the startTime
func generateDeployStatByProduct(startTime, endTime time.Time, projectKey string) (testDeployStat, productionDeployStat *statmodels.WeeklyDeployStat, retErr error) {
	testDeployStat = nil
	productionDeployStat = nil

	allDeployJobs, err := mongodb.NewJobInfoColl().GetDeployJobs(startTime.Unix(), endTime.Unix(), []string{projectKey}, config.Both)
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

	testDeployStat = &statmodels.WeeklyDeployStat{
		ProjectKey: projectKey,
		Production: false,
		Success:    testSuccess,
		Failed:     testFailed,
		Timeout:    testTimeout,
		Date:       date,
	}

	productionDeployStat = &statmodels.WeeklyDeployStat{
		ProjectKey: projectKey,
		Production: true,
		Success:    productionSuccess,
		Failed:     productionFailed,
		Timeout:    productionTimeout,
		Date:       date,
	}

	return
}

// generateReleaseStat generates the release stats counting from startTime to endTime
func generateReleaseStat(startTime, endTime time.Time) (*statmodels.MonthlyReleaseStat, error) {
	releasePlans, err := mongodb.NewReleasePlanColl().ListFinishedReleasePlan(startTime.Unix(), endTime.Unix())
	if err != nil {
		log.Errorf("failed to list release plan to calculate the statistics, error: %s", err)
		return nil, fmt.Errorf("failed to list release plan to calculate the statistics, error: %s", err)
	}

	var stat *statmodels.MonthlyReleaseStat

	if len(releasePlans) == 0 {
		stat = &statmodels.MonthlyReleaseStat{
			Total:                    0,
			AverageExecutionDuration: 0,
			AverageApprovalDuration:  0,
			Date:                     startTime.Format(config.Date),
			CreateTime:               startTime.Unix(),
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

		stat = &statmodels.MonthlyReleaseStat{
			Total:                    len(releasePlans),
			AverageExecutionDuration: averageExecutionDuration,
			AverageApprovalDuration:  averageApprovalDuration,
			Date:                     startTime.Format(config.Date),
			CreateTime:               time.Now().Unix(),
			UpdateTime:               0,
		}
	}

	return stat, nil
}

func migrateApprovalForAllWorkflows() error {
	// list all workflows
	workflows, _, err := mongodb.NewWorkflowV4Coll().List(&mongodb.ListWorkflowV4Option{}, 0, 0)

	if err != nil {
		log.Errorf("failed to list all custom workflows to do the migration, error: %s", err)
		return fmt.Errorf("failed to list all custom workflows to do the migration, error: %s", err)
	}

	for _, workflow := range workflows {
		newStages := make([]*models.WorkflowStage, 0)
		changed := false
		count := 0

		for _, stage := range workflow.Stages {
			if stage.Approval != nil && stage.Approval.Enabled {
				changed = true

				// default timeout is 60
				timeout := 60
				switch stage.Approval.Type {
				case config.NativeApproval:
					timeout = stage.Approval.NativeApproval.Timeout
				case config.DingTalkApproval:
					timeout = stage.Approval.DingTalkApproval.Timeout
				case config.LarkApproval:
					timeout = stage.Approval.LarkApproval.Timeout
				case config.WorkWXApproval:
					timeout = stage.Approval.WorkWXApproval.Timeout
				}

				approvalJob := []*models.Job{
					{
						Name:    fmt.Sprintf("approval-%d", count),
						JobType: config.JobApproval,
						Skipped: false,
						Spec: &models.ApprovalJobSpec{
							Timeout:          int64(timeout),
							Type:             stage.Approval.Type,
							Description:      stage.Approval.Description,
							NativeApproval:   stage.Approval.NativeApproval,
							LarkApproval:     stage.Approval.LarkApproval,
							DingTalkApproval: stage.Approval.DingTalkApproval,
							WorkWXApproval:   stage.Approval.WorkWXApproval,
						},
					},
				}

				newStages = append(newStages, &models.WorkflowStage{
					Name:     fmt.Sprintf("approval-%d", count),
					Parallel: false,
					Jobs:     approvalJob,
				})

				count++
			}

			newStages = append(newStages, stage)
		}

		// if there are approval stage in the workflow, we use the generated stages and update it
		if changed {
			workflow.Stages = newStages

			err := mongodb.NewWorkflowV4Coll().Update(workflow.ID.Hex(), workflow)
			if err != nil {
				log.Errorf("failed to update workflow: %s, error: %s", workflow.Name, err)
				return fmt.Errorf("failed to update workflow: %s, error: %s", workflow.Name, err)
			}
		}
	}
	return nil
}

func migrateApprovalForAllWorkflowTemplates() error {
	// list all workflow templates
	workflowTemplates, err := mongodb.NewWorkflowV4TemplateColl().List(&mongodb.WorkflowTemplateListOption{
		ExcludeBuildIn: true,
	})

	if err != nil {
		log.Errorf("failed to list all custom workflow templates to do the migration, error: %s", err)
		return fmt.Errorf("failed to list all custom workflow templates to do the migration, error: %s", err)
	}

	for _, workflow := range workflowTemplates {
		newStages := make([]*models.WorkflowStage, 0)
		changed := false
		count := 0

		for _, stage := range workflow.Stages {
			if stage.Approval != nil && stage.Approval.Enabled {
				changed = true

				// default timeout is 60
				timeout := 60
				switch stage.Approval.Type {
				case config.NativeApproval:
					timeout = stage.Approval.NativeApproval.Timeout
				case config.DingTalkApproval:
					timeout = stage.Approval.DingTalkApproval.Timeout
				case config.LarkApproval:
					timeout = stage.Approval.LarkApproval.Timeout
				case config.WorkWXApproval:
					timeout = stage.Approval.WorkWXApproval.Timeout
				}

				approvalJob := []*models.Job{
					{
						Name:    fmt.Sprintf("approval-%d", count),
						JobType: config.JobApproval,
						Skipped: false,
						Spec: &models.ApprovalJobSpec{
							Timeout:          int64(timeout),
							Type:             stage.Approval.Type,
							Description:      stage.Approval.Description,
							NativeApproval:   stage.Approval.NativeApproval,
							LarkApproval:     stage.Approval.LarkApproval,
							DingTalkApproval: stage.Approval.DingTalkApproval,
							WorkWXApproval:   stage.Approval.WorkWXApproval,
						},
					},
				}

				newStages = append(newStages, &models.WorkflowStage{
					Name:     fmt.Sprintf("approval-%d", count),
					Parallel: false,
					Jobs:     approvalJob,
				})

				count++
			}

			newStages = append(newStages, stage)
		}

		// if there are approval stage in the workflow, we use the generated stages and update it
		if changed {
			workflow.Stages = newStages

			err := mongodb.NewWorkflowV4TemplateColl().Update(workflow)
			if err != nil {
				log.Errorf("failed to update workflow template: %s, error: %s", workflow.TemplateName, err)
				return fmt.Errorf("failed to update workflow: %s, error: %s", workflow.TemplateName, err)
			}
		}
	}
	return nil
}

func migrateTestingAndScaningInfraField() error {
	// change testing infrastructure field
	cursor, err := mongodb.NewTestingColl().ListByCursor()
	if err != nil {
		return fmt.Errorf("failed to list testing cursor for infrastructure field in migrateTestingAndScaningInfraField method, err: %v", err)
	}

	var ms []mongo.WriteModel
	for cursor.Next(context.Background()) {
		var testing models.Testing
		if err := cursor.Decode(&testing); err != nil {
			return err
		}

		if testing.Infrastructure == "" {
			testing.Infrastructure = setting.JobK8sInfrastructure
			testing.ScriptType = types.ScriptTypeShell
			ms = append(ms,
				mongo.NewUpdateOneModel().
					SetFilter(bson.D{{"_id", testing.ID}}).
					SetUpdate(bson.D{{"$set",
						bson.D{
							{"infrastructure", testing.Infrastructure},
							{"script_type", testing.ScriptType},
						}},
					}),
			)
		}

		if len(ms) >= 50 {
			log.Infof("update %d testing", len(ms))
			if _, err := mongodb.NewTestingColl().BulkWrite(context.Background(), ms); err != nil {
				return fmt.Errorf("update testing for infrastructure field in migrateTestingAndScaningInfraField method, error: %s", err)
			}
			ms = []mongo.WriteModel{}
		}
	}

	if len(ms) > 0 {
		log.Infof("update %d testing", len(ms))
		if _, err := mongodb.NewTestingColl().BulkWrite(context.Background(), ms); err != nil {
			return fmt.Errorf("update testing for infrastructure field in migrateTestingAndScaningInfraField method, error: %s", err)
		}
	}

	// change scanning infrastructure field
	cursor, err = mongodb.NewScanningColl().ListByCursor()
	if err != nil {
		return fmt.Errorf("failed to list scanning cursor for infrastructure field in migrateTestingAndScaningInfraField method, err: %v", err)
	}

	ms = []mongo.WriteModel{}
	for cursor.Next(context.Background()) {
		var scanning models.Scanning
		if err := cursor.Decode(&scanning); err != nil {
			return err
		}

		if scanning.Infrastructure == "" {
			scanning.Infrastructure = setting.JobK8sInfrastructure
			scanning.ScriptType = types.ScriptTypeShell
			ms = append(ms,
				mongo.NewUpdateOneModel().
					SetFilter(bson.D{{"_id", scanning.ID}}).
					SetUpdate(bson.D{{"$set",
						bson.D{
							{"infrastructure", scanning.Infrastructure},
							{"script_type", scanning.ScriptType},
						}},
					}),
			)
		}

		if len(ms) >= 50 {
			log.Infof("update %d scanning", len(ms))
			if _, err := mongodb.NewScanningColl().BulkWrite(context.Background(), ms); err != nil {
				return fmt.Errorf("update sacnning for infrastructure field in migrateTestingAndScaningInfraField method, error: %s", err)
			}
			ms = []mongo.WriteModel{}
		}
	}

	if len(ms) > 0 {
		log.Infof("update %d scanning", len(ms))
		if _, err := mongodb.NewScanningColl().BulkWrite(context.Background(), ms); err != nil {
			return fmt.Errorf("update scanning for infrastructure field in migrateTestingAndScaningInfraField method, error: %s", err)
		}
	}

	// change scanning template infrastructure field
	cursor, err = mongodb.NewScanningTemplateColl().ListByCursor()
	if err != nil {
		return fmt.Errorf("failed to list scanning template cursor for infrastructure field in migrateTestingAndScaningInfraField method, err: %v", err)
	}

	ms = []mongo.WriteModel{}
	for cursor.Next(context.Background()) {
		var scanningTemplate models.ScanningTemplate
		if err := cursor.Decode(&scanningTemplate); err != nil {
			return err
		}

		if scanningTemplate.Infrastructure == "" {
			scanningTemplate.Infrastructure = setting.JobK8sInfrastructure
			scanningTemplate.ScriptType = types.ScriptTypeShell
			ms = append(ms,
				mongo.NewUpdateOneModel().
					SetFilter(bson.D{{"_id", scanningTemplate.ID}}).
					SetUpdate(bson.D{{"$set",
						bson.D{
							{"infrastructure", scanningTemplate.Infrastructure},
							{"script_type", scanningTemplate.ScriptType},
						}},
					}),
			)
		}

		if len(ms) >= 50 {
			log.Infof("update %d scanning template", len(ms))
			if _, err := mongodb.NewScanningTemplateColl().BulkWrite(context.Background(), ms); err != nil {
				return fmt.Errorf("update scanning template for infrastructure field in migrateTestingAndScaningInfraField method, error: %s", err)
			}
			ms = []mongo.WriteModel{}
		}
	}

	if len(ms) > 0 {
		log.Infof("update %d scanning template", len(ms))
		if _, err := mongodb.NewScanningTemplateColl().BulkWrite(context.Background(), ms); err != nil {
			return fmt.Errorf("update scanning template for infrastructure field in migrateTestingAndScaningInfraField method, error: %s", err)
		}
	}

	return nil
}
