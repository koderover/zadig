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
	"fmt"
	"time"

	"github.com/pkg/errors"
	"github.com/samber/lo"
	"go.uber.org/zap"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/tool/cache"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

func WatchExecutingWorkflow() {
	log := log.SugaredLogger().With("service", "WatchExecutingWorkflow")
	for {
		time.Sleep(time.Second * 3)

		releasePlanListLock := cache.NewRedisLockWithExpiry(fmt.Sprint("release-plan-watch-lock"), time.Minute*5)
		err := releasePlanListLock.TryLock()
		if err != nil {
			continue
		}

		t := time.Now()
		list, _, err := mongodb.NewReleasePlanColl().ListByOptions(&mongodb.ListReleasePlanOption{
			Status: config.StatusExecuting,
		})
		if err != nil {
			log.Errorf("list executing workflow error: %v", err)
			releasePlanListLock.Unlock()
			continue
		}
		for _, plan := range list {
			updatePlanWorkflowReleaseJob(plan, log)
		}
		if time.Since(t) > time.Millisecond*200 {
			log.Warnf("watch executing workflow cost %s", time.Since(t))
		}
		releasePlanListLock.Unlock()
	}
}

func updatePlanWorkflowReleaseJob(plan *models.ReleasePlan, log *zap.SugaredLogger) {
	releaseLock := getLock(plan.ID.Hex())
	lockErr := releaseLock.TryLock()
	if lockErr != nil {
		return
	}
	defer releaseLock.Unlock()

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
	changed := false
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
				changed = true
			}
			if task.Status == config.StatusPassed {
				job.Status = config.ReleasePlanJobStatusDone
				changed = true
			}
			if checkReleasePlanJobsAllDone(plan) {
				//plan.ExecutingTime = time.Now().Unix()
				plan.SuccessTime = time.Now().Unix()
				plan.Status = config.StatusSuccess
				changed = true
			}
		}
	}

	if time.Now().Unix() > plan.EndTime && plan.EndTime != 0 {
		plan.Status = config.StatusTimeoutForWindow
		changed = true
	}

	if changed {
		if err := mongodb.NewReleasePlanColl().UpdateByID(ctx, plan.ID.Hex(), plan); err != nil {
			log.Errorf("update plan %s error: %v", plan.ID.Hex(), err)
		}
	}
	return
}

func WatchApproval() {
	log := log.SugaredLogger().With("service", "WatchApproval")
	for {
		releasePlanApprovalLock := cache.NewRedisLockWithExpiry(fmt.Sprint("release-plan-approval-lock"), time.Minute*5)
		err := releasePlanApprovalLock.TryLock()
		if err != nil {
			continue
		}

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

		releasePlanApprovalLock.Unlock()
	}
}

func updatePlanApproval(plan *models.ReleasePlan) error {
	approveLock := getLock(plan.ID.Hex())
	lockErr := approveLock.TryLock()
	if lockErr != nil {
		return nil
	}
	defer approveLock.Unlock()

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

	// skip if plan has been rejected
	if plan.Approval.Status == config.StatusReject {
		return nil
	}

	switch plan.Approval.Type {
	case config.LarkApproval:
		err = updateLarkApproval(ctx, plan.Approval)
	case config.DingTalkApproval:
		err = updateDingTalkApproval(ctx, plan.Approval)
	case config.WorkWXApproval:
		err = updateWorkWXApproval(ctx, plan.Approval)
	// NativeApproval is update when approve
	case config.NativeApproval:
		return nil
	default:
		err = errors.Errorf("unknown approval type %s", plan.Approval.Type)
	}
	if err != nil {
		return errors.Errorf("update plan %s approval error: %v", plan.Name, err)
	}
	var planLog *models.ReleasePlanLog
	switch plan.Approval.Status {
	case config.StatusPassed:
		planLog = &models.ReleasePlanLog{
			PlanID:     plan.ID.Hex(),
			Username:   "系统",
			Verb:       VerbUpdate,
			TargetName: TargetTypeReleasePlanStatus,
			TargetType: TargetTypeReleasePlanStatus,
			Detail:     "审批通过",
			After:      config.StatusExecuting,
			CreatedAt:  time.Now().Unix(),
		}
		plan.Status = config.StatusExecuting
		plan.ApprovalTime = time.Now().Unix()

		if err := upsertReleasePlanCron(plan.ID.Hex(), plan.Name, plan.Index, plan.Status, plan.ScheduleExecuteTime); err != nil {
			err = errors.Wrap(err, "upsert release plan cron")
			log.Error(err)
		}

		setReleaseJobsForExecuting(plan)
	case config.StatusReject:
		planLog = &models.ReleasePlanLog{
			PlanID:    plan.ID.Hex(),
			Detail:    "审批被拒绝",
			CreatedAt: time.Now().Unix(),
		}
		log.Infof("chaning status to denied")
		plan.Status = config.StatusApprovalDenied
		plan.ApprovalTime = time.Now().Unix()
	}

	if err := mongodb.NewReleasePlanColl().UpdateByID(ctx, plan.ID.Hex(), plan); err != nil {
		return errors.Errorf("update plan %s error: %v", plan.ID.Hex(), err)
	}

	go func() {
		if planLog == nil {
			return
		}
		if err := mongodb.NewReleasePlanLogColl().Create(planLog); err != nil {
			log.Errorf("create release plan log error: %v", err)
		}
	}()

	return nil
}
