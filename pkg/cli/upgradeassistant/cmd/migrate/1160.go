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

package migrate

import (
	"context"
	"fmt"
	"time"

	"github.com/koderover/zadig/pkg/cli/upgradeassistant/internal/upgradepath"
	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/task"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/koderover/zadig/pkg/types"
	steptypes "github.com/koderover/zadig/pkg/types/step"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

func init() {
	upgradepath.RegisterHandler("1.15.0", "1.16.0", V1150ToV1160)
	upgradepath.RegisterHandler("1.16.0", "1.15.0", V1150ToV1140)
}

func V1150ToV1160() error {
	if err := initWorkflowV4TaskStats(); err != nil {
		log.Errorf("initWorkflowV4TaskStats err:%s", err)
		return err
	}
	if err := addDisplayNameToWorkflowV4(); err != nil {
		log.Errorf("addDisplayNameToWorkflowV4 err:%s", err)
		return err
	}
	if err := addDisplayNameToWorkflow(); err != nil {
		log.Errorf("addDisplayNameToWorkflow err:%s", err)
		return err
	}
	return nil
}

func V1160ToV1150() error {
	return nil
}

func initWorkflowV4TaskStats() error {
	taskCursor, err := mongodb.NewworkflowTaskv4Coll().ListByCursor(&mongodb.ListWorkflowTaskV4Option{})
	if err != nil {
		return err
	}
	statMap := map[string]*models.WorkflowStat{}
	for taskCursor.Next(context.Background()) {
		var workflowTask models.WorkflowTask
		if err := taskCursor.Decode(&workflowTask); err != nil {
			return err
		}
		if workflowTask.Status != config.StatusPassed && workflowTask.Status != config.StatusFailed && workflowTask.Status != config.StatusTimeout {
			continue
		}
		totalSuccess := 0
		totalFailure := 0
		if workflowTask.Status == config.StatusPassed {
			totalSuccess = 1
			totalFailure = 0
		} else {
			totalSuccess = 0
			totalFailure = 1
		}
		duration := workflowTask.EndTime - workflowTask.StartTime
		if stat, exist := statMap[workflowTask.WorkflowName]; !exist {
			statMap[workflowTask.WorkflowName] = &models.WorkflowStat{
				ProductName:   workflowTask.ProjectName,
				Name:          workflowTask.WorkflowName,
				Type:          string(config.WorkflowTypeV4),
				TotalDuration: duration,
				TotalSuccess:  totalSuccess,
				TotalFailure:  totalFailure,
				CreatedAt:     time.Now().Unix(),
				UpdatedAt:     time.Now().Unix(),
			}
		} else {
			stat.TotalDuration += duration
			stat.TotalSuccess += totalSuccess
			stat.TotalFailure += totalFailure
		}
	}
	var ms []mongo.WriteModel
	for _, stat := range statMap {
		ms = append(ms,
			mongo.NewUpdateOneModel().
				SetFilter(bson.D{{"name", stat.Name}, {"type", stat.Type}}).
				SetUpdate(bson.D{{"$set", stat}}).SetUpsert(true),
		)
	}
	if len(ms) > 0 {
		if _, err := mongodb.NewWorkflowStatColl().BulkWrite(context.TODO(), ms); err != nil {
			return fmt.Errorf("udpate workflowV4s stat error: %s", err)
		}
	}
	return nil
}

func addDisplayNameToWorkflowV4() error {
	cursor, err := mongodb.NewWorkflowV4Coll().ListByCursor(&mongodb.ListWorkflowV4Option{})
	if err != nil {
		return err
	}
	var ms []mongo.WriteModel
	for cursor.Next(context.Background()) {
		var workflow models.WorkflowV4
		if err := cursor.Decode(&workflow); err != nil {
			return err
		}
		displayName := workflow.DisplayName
		if workflow.DisplayName == "" {
			displayName = workflow.Name
		}
		for _, webhook := range workflow.HookCtls {
			setPRsforWorkflowV4(webhook.WorkflowArg)
		}
		ms = append(ms,
			mongo.NewUpdateOneModel().
				SetFilter(bson.D{{"_id", workflow.ID}}).
				SetUpdate(bson.D{{"$set",
					bson.D{
						{"display_name", displayName},
						{"hook_ctl", workflow.HookCtls},
					}},
				}),
		)
		if len(ms) >= 50 {
			log.Infof("update %d workflowv4s", len(ms))
			if _, err := mongodb.NewWorkflowV4Coll().BulkWrite(context.TODO(), ms); err != nil {
				return fmt.Errorf("udpate workflowV4s error: %s", err)
			}
			ms = []mongo.WriteModel{}
		}
	}
	if len(ms) > 0 {
		log.Infof("update %d workflowv4s", len(ms))
		if _, err := mongodb.NewWorkflowV4Coll().BulkWrite(context.TODO(), ms); err != nil {
			return fmt.Errorf("udpate workflowV4s error: %s", err)
		}
	}

	taskCursor, err := mongodb.NewworkflowTaskv4Coll().ListByCursor(&mongodb.ListWorkflowTaskV4Option{})
	if err != nil {
		return err
	}
	var mTasks []mongo.WriteModel
	for taskCursor.Next(context.Background()) {
		var workflowTask models.WorkflowTask
		if err := taskCursor.Decode(&workflowTask); err != nil {
			return err
		}
		displayName := workflowTask.WorkflowDisplayName
		if workflowTask.WorkflowDisplayName == "" {
			displayName = workflowTask.WorkflowName
		}
		setPRsforWorkflowV4(workflowTask.OriginWorkflowArgs)
		mTasks = append(mTasks,
			mongo.NewUpdateOneModel().
				SetFilter(bson.D{{"_id", workflowTask.ID}}).
				SetUpdate(bson.D{{"$set",
					bson.D{
						{"workflow_display_name", displayName},
						{"origin_workflow_args", workflowTask.OriginWorkflowArgs},
					}},
				}),
		)
		if len(mTasks) >= 50 {
			log.Infof("update %d workflowv4 tasks", len(mTasks))
			if _, err := mongodb.NewworkflowTaskv4Coll().BulkWrite(context.TODO(), mTasks); err != nil {
				return fmt.Errorf("udpate workflowV4 tasks error: %s", err)
			}
			mTasks = []mongo.WriteModel{}
		}
	}
	if len(mTasks) > 0 {
		log.Infof("update %d workflowv4 tasks", len(mTasks))
		if _, err := mongodb.NewworkflowTaskv4Coll().BulkWrite(context.TODO(), mTasks); err != nil {
			return fmt.Errorf("udpate workflowV4 tasks error: %s", err)
		}
	}

	return nil
}

func setPRsforWorkflowV4(workflow *models.WorkflowV4) {
	if workflow == nil {
		return
	}
	for _, stage := range workflow.Stages {
		for _, job := range stage.Jobs {
			switch job.JobType {
			case config.JobZadigBuild:
				spec := &models.ZadigBuildJobSpec{}
				if err := models.IToi(job.Spec, spec); err != nil {
					continue
				}
				for _, svc := range spec.ServiceAndBuilds {
					for _, repo := range svc.Repos {
						setPRs(repo)
					}
				}
				job.Spec = spec
			case config.JobFreestyle:
				spec := &models.FreestyleJobSpec{}
				if err := models.IToi(job.Spec, spec); err != nil {
					continue
				}
				for _, step := range spec.Steps {
					if step.StepType != config.StepGit {
						continue
					}
					stepSpec := &steptypes.StepGitSpec{}
					if err := models.IToi(step.Spec, stepSpec); err != nil {
						continue
					}
					for _, repo := range stepSpec.Repos {
						setPRs(repo)
					}
					step.Spec = stepSpec
				}
				job.Spec = spec
			case config.JobZadigTesting:
				spec := &models.ZadigTestingJobSpec{}
				if err := models.IToi(job.Spec, spec); err != nil {
					continue
				}
				for _, test := range spec.TestModules {
					for _, repo := range test.Repos {
						setPRs(repo)
					}
				}
				job.Spec = spec
			}
		}
	}
}

func addDisplayNameToWorkflow() error {
	cursor, err := mongodb.NewWorkflowColl().ListByCursor(&mongodb.ListWorkflowOption{})
	if err != nil {
		return err
	}
	var ms []mongo.WriteModel
	for cursor.Next(context.Background()) {
		var workflow models.Workflow
		if err := cursor.Decode(&workflow); err != nil {
			return err
		}
		if workflow.DisplayName != "" {
			continue
		}
		ms = append(ms,
			mongo.NewUpdateOneModel().
				SetFilter(bson.D{{"_id", workflow.ID}}).
				SetUpdate(bson.D{{"$set",
					bson.D{
						{"display_name", workflow.Name},
					}},
				}),
		)
		if len(ms) >= 50 {
			log.Infof("update %d workflows", len(ms))
			if _, err := mongodb.NewWorkflowColl().BulkWrite(context.TODO(), ms); err != nil {
				return fmt.Errorf("udpate workflows error: %s", err)
			}
			ms = []mongo.WriteModel{}
		}
	}
	if len(ms) > 0 {
		log.Infof("update %d workflows", len(ms))
		if _, err := mongodb.NewWorkflowColl().BulkWrite(context.TODO(), ms); err != nil {
			return fmt.Errorf("udpate workflows error: %s", err)
		}
	}

	taskCursor, err := mongodb.NewTaskColl().ListByCursor(&mongodb.ListAllTaskOption{})
	if err != nil {
		return err
	}
	var mTasks []mongo.WriteModel
	for taskCursor.Next(context.Background()) {
		var workflowTask task.Task
		if err := cursor.Decode(&workflowTask); err != nil {
			return err
		}
		displayName := workflowTask.PipelineDisplayName
		if workflowTask.PipelineDisplayName == "" {
			displayName = workflowTask.PipelineName
		}
		setPRsForWorkflowTask(&workflowTask)
		mTasks = append(mTasks,
			mongo.NewUpdateOneModel().
				SetFilter(bson.D{{"_id", workflowTask.ID}}).
				SetUpdate(bson.D{{"$set",
					bson.D{
						{"pipeline_display_name", displayName},
						{"workflow_args", workflowTask.WorkflowArgs},
					}},
				}),
		)
		if len(mTasks) >= 50 {
			log.Infof("update %d workflow tasks", len(mTasks))
			if _, err := mongodb.NewTaskColl().BulkWrite(context.TODO(), mTasks); err != nil {
				return fmt.Errorf("udpate workflow tasks error: %s", err)
			}
			mTasks = []mongo.WriteModel{}
		}
	}
	if len(mTasks) > 0 {
		log.Infof("update %d workflow tasks", len(mTasks))
		if _, err := mongodb.NewTaskColl().BulkWrite(context.TODO(), mTasks); err != nil {
			return fmt.Errorf("udpate workflow tasks error: %s", err)
		}
	}
	return nil
}

func setPRsForWorkflowTask(t *task.Task) {
	if t.WorkflowArgs == nil {
		return
	}
	for _, target := range t.WorkflowArgs.Target {
		if target.Build == nil {
			continue
		}
		for _, repo := range target.Build.Repos {
			setPRs(repo)
		}
	}
	for _, test := range t.WorkflowArgs.Tests {
		for _, repo := range test.Builds {
			setPRs(repo)
		}
	}
}

func setPRs(repo *types.Repository) {
	if len(repo.PRs) > 0 {
		return
	}
	if repo.PR > 0 {
		repo.PRs = []int{repo.PR}
	}
}
