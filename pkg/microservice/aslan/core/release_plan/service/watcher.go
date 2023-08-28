/*
 * Copyright 2023 The KodeRover Authors.
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

package service

import (
	"context"
	"time"

	"github.com/pkg/errors"
	"github.com/samber/lo"
	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/tool/log"
)

func WatchExecutingWorkflow() {
	log := log.SugaredLogger().With("service", "WatchExecutingWorkflow")
	for {
		time.Sleep(time.Second * 3)
		t := time.Now()
		list, _, err := mongodb.NewReleasePlanColl().ListByOptions(&mongodb.ListReleasePlanOption{
			Status: config.StatusExecuting,
		})
		if err != nil {
			log.Errorf("list executing workflow error: %v", err)
			continue
		}
		for _, plan := range list {
			updatePlanWorkflowReleaseJob(plan, log)
		}
		if time.Since(t) > time.Millisecond*200 {
			log.Warnf("watch executing workflow cost %s", time.Since(t))
		}
	}
}

func updatePlanWorkflowReleaseJob(plan *models.ReleasePlan, log *zap.SugaredLogger) {
	getLock(plan.ID.Hex()).Lock()
	defer getLock(plan.ID.Hex()).Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	plan, err := mongodb.NewReleasePlanColl().GetByID(ctx, plan.ID.Hex())
	if err != nil {
		log.Errorf("get plan %s error: %v", plan.ID.Hex(), err)
		return
	}
	// plan status maybe changed during no lock time
	if plan.Status != config.StatusExecuting {
		return
	}
	for _, job := range plan.Jobs {
		if job.Status == config.ReleasePlanJobStatusRunning && job.Type == config.JobWorkflow {
			spec := new(models.WorkflowReleaseJobSpec)
			if err := models.IToi(job.Spec, spec); err != nil {
				log.Errorf("convert spec error: %v", err)
				continue
			}
			task, err := mongodb.NewworkflowTaskv4Coll().Find(spec.Workflow.Name, spec.TaskID)
			if err != nil {
				log.Errorf("find task %s-%d error: %v", spec.Workflow.Name, spec.TaskID, err)
				continue
			}
			spec.Status = task.Status
			if lo.Contains(config.FailedStatus(), task.Status) {
				job.Status = config.ReleasePlanJobStatusFailed
			}
			if task.Status == config.StatusPassed {
				job.Status = config.ReleasePlanJobStatusDone
			}
			if checkReleasePlanJobsAllDone(plan) {
				plan.Status = config.StatusSuccess
			}
		}
	}
	if err := mongodb.NewReleasePlanColl().UpdateByID(ctx, plan.ID.Hex(), plan); err != nil {
		log.Errorf("update plan %s error: %v", plan.ID.Hex(), err)
	}
	return
}

func WatchApproval() {
	log := log.SugaredLogger().With("service", "WatchApproval")
	for {
		time.Sleep(time.Second * 3)
		t := time.Now()
		list, _, err := mongodb.NewReleasePlanColl().ListByOptions(&mongodb.ListReleasePlanOption{
			Status: config.StatusWaitForApprove,
		})
		if err != nil {
			log.Errorf("list approval workflow error: %v", err)
			continue
		}
		for _, plan := range list {
			if err := updatePlanApproval(plan); err != nil {
				log.Errorf("update plan %s approval error: %v", plan.Name, err)
			}
		}
		if time.Since(t) > time.Millisecond*200 {
			log.Warnf("watch approval workflow cost %s", time.Since(t))
		}
	}
}

func updatePlanApproval(plan *models.ReleasePlan) error {
	getLock(plan.ID.Hex()).Lock()
	defer getLock(plan.ID.Hex()).Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	plan, err := mongodb.NewReleasePlanColl().GetByID(ctx, plan.ID.Hex())
	if err != nil {
		return errors.Errorf("get plan %s error: %v", plan.ID.Hex(), err)
	}
	// plan status maybe changed during no lock time
	if plan.Status != config.StatusWaitForApprove {
		return nil
	}

	switch plan.Approval.Type {
	case config.LarkApproval:
		err = updateLarkApproval(ctx, plan.Approval)
	case config.DingTalkApproval:
		err = updateDingTalkApproval(ctx, plan.Approval)
	case config.NativeApproval:
		err = updateNativeApproval(ctx, plan.Approval)
	default:
		err = errors.Errorf("unknown approval type %s", plan.Approval.Type)
	}
	if err != nil {
		return errors.Errorf("update plan %s approval error: %v", plan.Name, err)
	}
	switch plan.Approval.Status {
	case config.StatusPassed:
		plan.Logs = append(plan.Logs, &models.ReleasePlanLog{
			Verb:       VerbUpdate,
			TargetName: plan.Name,
			TargetType: TargetTypeReleasePlanStatus,
			Detail:     "审批通过",
			CreatedAt:  time.Now().Unix(),
		})
		plan.Status = config.StatusExecuting
		// TODO
	case config.StatusReject:
		plan.Logs = append(plan.Logs, &models.ReleasePlanLog{
			Verb:       VerbUpdate,
			TargetName: plan.Name,
			TargetType: TargetTypeReleasePlanStatus,
			Detail:     "审批拒绝",
			CreatedAt:  time.Now().Unix(),
		})
	}

	if err := mongodb.NewReleasePlanColl().UpdateByID(ctx, plan.ID.Hex(), plan); err != nil {
		return errors.Errorf("update plan %s error: %v", plan.ID.Hex(), err)
	}
	return nil
}
