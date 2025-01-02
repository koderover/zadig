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
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/samber/lo"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models/msg_queue"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	approvalservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/approval"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/workflow/service/workflow/job"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/shared/client/user"
	"github.com/koderover/zadig/v2/pkg/shared/handler"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/util"
)

var (
	defaultTimeout = time.Second * 30
)

func CreateReleasePlan(c *handler.Context, args *models.ReleasePlan) error {
	if args.Name == "" || args.ManagerID == "" {
		return errors.New("Required parameters are missing")
	}
	if err := lintReleaseTimeRange(args.StartTime, args.EndTime); err != nil {
		return errors.Wrap(err, "lint release time range error")
	}
	if err := lintScheduleExecuteTime(args.ScheduleExecuteTime, args.StartTime, args.EndTime); err != nil {
		return errors.Wrap(err, "lint schedule execute time error")
	}
	userInfo, err := user.New().GetUserByID(args.ManagerID)
	if err != nil {
		return errors.Errorf("Failed to get user by id %s, error: %v", args.ManagerID, err)
	}
	if args.Manager != userInfo.Name {
		return errors.Errorf("Manager %s is not consistent with the user name %s", args.Manager, userInfo.Name)
	}

	for _, job := range args.Jobs {
		if err := lintReleaseJob(job.Type, job.Spec); err != nil {
			return errors.Errorf("lintReleaseJob %s error: %v", job.Name, err)
		}
		job.ReleaseJobRuntime = models.ReleaseJobRuntime{}
		job.ID = uuid.New().String()
	}

	if args.Approval != nil {
		if err := lintApproval(args.Approval); err != nil {
			return errors.Errorf("lintApproval error: %v", err)
		}
		if args.Approval.Type == config.LarkApproval {
			if err := createLarkApprovalDefinition(args.Approval.LarkApproval); err != nil {
				return errors.Errorf("createLarkApprovalDefinition error: %v", err)
			}
		}
	}

	nextID, err := mongodb.NewCounterColl().GetNextSeq(setting.ReleasePlanFmt)
	if err != nil {
		log.Errorf("CreateReleasePlan.GetNextSeq error: %v", err)
		return e.ErrGetCounter.AddDesc(err.Error())
	}
	args.Index = nextID
	args.CreatedBy = c.UserName
	args.UpdatedBy = c.UserName
	args.CreateTime = time.Now().Unix()
	args.UpdateTime = time.Now().Unix()
	args.Status = config.StatusPlanning

	planID, err := mongodb.NewReleasePlanColl().Create(args)
	if err != nil {
		return errors.Wrap(err, "create release plan error")
	}

	go func() {
		if err := mongodb.NewReleasePlanLogColl().Create(&models.ReleasePlanLog{
			PlanID:     planID,
			Username:   c.UserName,
			Account:    c.Account,
			Verb:       VerbCreate,
			TargetName: args.Name,
			TargetType: TargetTypeReleasePlan,
			CreatedAt:  time.Now().Unix(),
		}); err != nil {
			log.Errorf("create release plan log error: %v", err)
		}
	}()

	return nil
}

func upsertReleasePlanCron(id, name string, index int64, ScheduleExecuteTime int64) error {
	var (
		err             error
		payload         *commonservice.CronjobPayload
		releasePlanCron *commonmodels.Cronjob
	)

	enable := false
	if ScheduleExecuteTime != 0 {
		enable = true
	}

	found := false
	releasePlanCronName := util.GetReleasePlanCronName(id, name, index)
	releasePlanCron, err = commonrepo.NewCronjobColl().GetByName(releasePlanCronName, setting.ReleasePlanCronjob)
	if err != nil {
		if err != mongo.ErrNoDocuments && err != mongo.ErrNilDocument {
			return e.ErrUpsertCronjob.AddErr(fmt.Errorf("failed to get release plan cron job, err: %w", err))
		}
	} else {
		found = true
	}

	if found {
		origEnabled := releasePlanCron.Enabled
		releasePlanCron.Enabled = enable
		releasePlanCron.ReleasePlanArgs = &commonmodels.ReleasePlanArgs{
			ID:    id,
			Name:  name,
			Index: index,
		}
		releasePlanCron.Cron = util.UnixStampToCronExpr(ScheduleExecuteTime)

		err = commonrepo.NewCronjobColl().Upsert(releasePlanCron)
		if err != nil {
			fmtErr := fmt.Errorf("Failed to upsert cron job, error: %w", err)
			log.Error(fmtErr)
			return err
		}

		if origEnabled && !enable {
			// need to disable cronjob
			payload = &commonservice.CronjobPayload{
				Name:       releasePlanCronName,
				JobType:    setting.ReleasePlanCronjob,
				Action:     setting.TypeEnableCronjob,
				DeleteList: []string{releasePlanCron.ID.Hex()},
			}
		} else if !origEnabled && enable || origEnabled && enable {
			payload = &commonservice.CronjobPayload{
				Name:    releasePlanCronName,
				JobType: setting.ReleasePlanCronjob,
				Action:  setting.TypeEnableCronjob,
				JobList: []*commonmodels.Schedule{cronJobToSchedule(releasePlanCron)},
			}
		} else {
			// !origEnabled && !enable
			return nil
		}
	} else {
		input := &commonmodels.Cronjob{
			Name: releasePlanCronName,
			Type: setting.ReleasePlanCronjob,
		}
		input.Enabled = true
		input.Cron = util.UnixStampToCronExpr(ScheduleExecuteTime)
		input.ReleasePlanArgs = &commonmodels.ReleasePlanArgs{
			ID:    id,
			Name:  name,
			Index: index,
		}

		err = commonrepo.NewCronjobColl().Upsert(input)
		if err != nil {
			fmtErr := fmt.Errorf("Failed to upsert cron job, error: %w", err)
			log.Error(fmtErr)
			return err
		}
		payload = &commonservice.CronjobPayload{
			Name:    releasePlanCronName,
			JobType: setting.ReleasePlanCronjob,
			Action:  setting.TypeEnableCronjob,
			JobList: []*commonmodels.Schedule{cronJobToSchedule(input)},
		}
	}

	pl, _ := json.Marshal(payload)
	err = commonrepo.NewMsgQueueCommonColl().Create(&msg_queue.MsgQueueCommon{
		Payload:   string(pl),
		QueueType: setting.TopicCronjob,
	})
	if err != nil {
		log.Errorf("Failed to publish to msg queue: %s, the error is: %v", setting.TopicCronjob, err)
		return e.ErrUpsertCronjob.AddDesc(err.Error())
	}
	return nil
}

func GetReleasePlan(id string) (*models.ReleasePlan, error) {
	releasePlan, err := mongodb.NewReleasePlanColl().GetByID(context.Background(), id)
	if err != nil {
		return nil, errors.Wrap(err, "GetReleasePlan")
	}

	// native approval users may be user or user groups
	// convert to flat user when needed, this data is generated dynamically because group binding may be changed
	if releasePlan.Approval != nil && releasePlan.Approval.NativeApproval != nil {
		flatNativeApprovalUsers, _ := geneFlatNativeApprovalUsers(releasePlan.Approval.NativeApproval)
		releasePlan.Approval.NativeApproval.FloatApproveUsers = flatNativeApprovalUsers
	}

	for _, releasePlanJob := range releasePlan.Jobs {
		if releasePlanJob.Type == config.JobWorkflow {
			spec := new(models.WorkflowReleaseJobSpec)
			if err := models.IToi(releasePlanJob.Spec, spec); err != nil {
				return nil, fmt.Errorf("invalid spec for job: %s. decode error: %s", releasePlanJob.Name, err)
			}
			if spec.Workflow == nil {
				return nil, fmt.Errorf("workflow is nil")
			}

			originalWorkflow, err := mongodb.NewWorkflowV4Coll().Find(spec.Workflow.Name)
			if err != nil {
				log.Errorf("Failed to find WorkflowV4: %s, the error is: %v", spec.Workflow.Name, err)
				return nil, fmt.Errorf("failed to find WorkflowV4: %s, the error is: %v", spec.Workflow.Name, err)
			}

			if err := job.MergeArgs(originalWorkflow, spec.Workflow); err != nil {
				errMsg := fmt.Sprintf("merge workflow args error: %v", err)
				log.Error(errMsg)
				return nil, fmt.Errorf(errMsg)
			}

			for _, stage := range originalWorkflow.Stages {
				for _, item := range stage.Jobs {
					err := job.SetOptions(item, originalWorkflow, nil)
					if err != nil {
						errMsg := fmt.Sprintf("merge workflow args set options error: %v", err)
						log.Error(errMsg)
						return nil, fmt.Errorf(errMsg)
					}

					// additionally we need to update the user-defined args with the latest workflow configuration
					err = job.UpdateWithLatestSetting(item, originalWorkflow)
					if err != nil {
						errMsg := fmt.Sprintf("failed to merge user-defined workflow args with latest workflow configuration, error: %s", err)
						log.Error(errMsg)
						return nil, fmt.Errorf(errMsg)
					}
				}
			}

			spec.Workflow = originalWorkflow
			releasePlanJob.Spec = spec
		}
	}

	return releasePlan, nil
}

func GetReleasePlanLogs(id string) ([]*models.ReleasePlanLog, error) {
	return mongodb.NewReleasePlanLogColl().ListByOptions(&mongodb.ListReleasePlanLogOption{
		PlanID: id,
		IsSort: true,
	})
}

func DeleteReleasePlan(c *gin.Context, username, id string) error {
	info, err := mongodb.NewReleasePlanColl().GetByID(context.Background(), id)
	if err != nil {
		return errors.Wrap(err, "get plan")
	}
	internalhandler.InsertOperationLog(c, username, "", "删除", "发布计划", info.Name, "", log.SugaredLogger())
	return mongodb.NewReleasePlanColl().DeleteByID(context.Background(), id)
}

type UpdateReleasePlanArgs struct {
	Verb string      `json:"verb"`
	Spec interface{} `json:"spec"`
}

func UpdateReleasePlan(c *handler.Context, planID string, args *UpdateReleasePlanArgs) error {
	approveLock := getLock(planID)
	approveLock.Lock()
	defer approveLock.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	plan, err := mongodb.NewReleasePlanColl().GetByID(ctx, planID)
	if err != nil {
		return errors.Wrap(err, "get plan")
	}

	if plan.Status != config.StatusPlanning {
		return errors.Errorf("plan status is %s, can not update", plan.Status)
	}

	updater, err := NewPlanUpdater(args)
	if err != nil {
		return errors.Wrap(err, "new plan updater")
	}
	if err = updater.Lint(); err != nil {
		return errors.Wrap(err, "lint")
	}
	before, after, err := updater.Update(plan)
	if err != nil {
		return errors.Wrap(err, "update")
	}

	plan.UpdatedBy = c.UserName
	plan.UpdateTime = time.Now().Unix()

	if err = mongodb.NewReleasePlanColl().UpdateByID(ctx, planID, plan); err != nil {
		return errors.Wrap(err, "update plan")
	}

	go func() {
		if err := mongodb.NewReleasePlanLogColl().Create(&models.ReleasePlanLog{
			PlanID:     planID,
			Username:   c.UserName,
			Account:    c.Account,
			Verb:       updater.Verb(),
			Before:     before,
			After:      after,
			TargetName: updater.TargetName(),
			TargetType: updater.TargetType(),
			CreatedAt:  time.Now().Unix(),
		}); err != nil {
			log.Errorf("create release plan log error: %v", err)
		}
	}()

	return nil
}

type ExecuteReleaseJobArgs struct {
	ID   string      `json:"id"`
	Name string      `json:"name"`
	Type string      `json:"type"`
	Spec interface{} `json:"spec"`
}

func ExecuteReleaseJob(c *handler.Context, planID string, args *ExecuteReleaseJobArgs, isSystemAdmin bool) error {
	approveLock := getLock(planID)
	approveLock.Lock()
	defer approveLock.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	plan, err := mongodb.NewReleasePlanColl().GetByID(ctx, planID)
	if err != nil {
		return errors.Wrap(err, "get plan")
	}

	if plan.Status != config.StatusExecuting {
		return errors.Errorf("plan status is %s, can not execute", plan.Status)
	}

	if !(plan.StartTime == 0 && plan.EndTime == 0) {
		now := time.Now().Unix()
		if now < plan.StartTime || now > plan.EndTime {
			return errors.Errorf("plan is not in the release time range")
		}
	}

	if plan.ManagerID != c.UserID && !isSystemAdmin {
		return errors.Errorf("only manager can execute")
	}

	executor, err := NewReleaseJobExecutor(&ExecuteReleaseJobContext{
		AuthResources: c.Resources,
		UserID:        c.UserID,
		Account:       c.Account,
		UserName:      c.UserName,
	}, args)
	if err != nil {
		return errors.Wrap(err, "new release job executor")
	}
	if err = executor.Execute(plan); err != nil {
		return errors.Wrap(err, "execute")
	}

	plan.UpdatedBy = c.UserName
	plan.UpdateTime = time.Now().Unix()

	if checkReleasePlanJobsAllDone(plan) {
		//plan.ExecutingTime = time.Now().Unix()
		plan.SuccessTime = time.Now().Unix()
		plan.Status = config.StatusSuccess
	}

	if err = mongodb.NewReleasePlanColl().UpdateByID(ctx, planID, plan); err != nil {
		return errors.Wrap(err, "update plan")
	}

	go func() {
		if err := mongodb.NewReleasePlanLogColl().Create(&models.ReleasePlanLog{
			PlanID:     planID,
			Username:   c.UserName,
			Account:    c.Account,
			Verb:       VerbExecute,
			TargetName: args.Name,
			TargetType: TargetTypeReleaseJob,
			CreatedAt:  time.Now().Unix(),
		}); err != nil {
			log.Errorf("create release plan log error: %v", err)
		}
	}()

	return nil
}

func ScheduleExecuteReleasePlan(c *handler.Context, planID string) error {
	approveLock := getLock(planID)
	approveLock.Lock()
	defer approveLock.Unlock()

	ctx := context.Background()
	plan, err := mongodb.NewReleasePlanColl().GetByID(ctx, planID)
	if err != nil {
		err = errors.Wrap(err, "get plan")
		log.Error(err)
		return err
	}

	if plan.Status != config.StatusExecuting {
		err = errors.Errorf("plan ID is %s, name is %s, index is %d, status is %s, can not execute", plan.ID, plan.Name, plan.Index, plan.Status)
		log.Error(err)
		return err
	}

	if !(plan.StartTime == 0 && plan.EndTime == 0) {
		now := time.Now().Unix()
		if now < plan.StartTime || now > plan.EndTime {
			err = errors.Errorf("plan ID is %s, name is %s, index is %d, it's not in the release time range", plan.ID, plan.Name, plan.Index)
			log.Error(err)
			return err
		}
	}

	if plan.Approval != nil && plan.Approval.Enabled == true && plan.Approval.Status != config.StatusPassed {
		err = errors.Errorf("plan ID is %s, name is %s, index is %d, it's approval status is %s, can not execute", plan.ID, plan.Name, plan.Index, plan.Approval.Status)
		log.Error(err)
		return err
	}

	log.Infof("schedule execute release plan, plan ID: %s, name: %s, index: %d", plan.ID, plan.Name, plan.Index)

	for _, job := range plan.Jobs {
		if job.Type == config.JobWorkflow {
			if job.Status == config.ReleasePlanJobStatusDone || job.Status == config.ReleasePlanJobStatusSkipped || job.Status == config.ReleasePlanJobStatusRunning {
				continue
			}

			args := &ExecuteReleaseJobArgs{
				ID:   job.ID,
				Name: job.Name,
				Type: string(job.Type),
			}
			executor, err := NewReleaseJobExecutor(&ExecuteReleaseJobContext{
				AuthResources: c.Resources,
				UserID:        c.UserID,
				Account:       "",
				UserName:      "系统",
			}, args)
			if err != nil {
				err = errors.Wrap(err, "new release job executor")
				log.Error(err)
				return err
			}
			if err = executor.Execute(plan); err != nil {
				err = errors.Wrap(err, "execute")
				log.Error(err)
				return err
			}

			plan.UpdatedBy = "系统"
			plan.UpdateTime = time.Now().Unix()

			if checkReleasePlanJobsAllDone(plan) {
				//plan.ExecutingTime = time.Now().Unix()
				plan.SuccessTime = time.Now().Unix()
				plan.Status = config.StatusSuccess
			}

			log.Infof("schedule execute release job, plan ID: %s, name: %s, index: %d, job ID: %s, job name: %s", plan.ID, plan.Name, plan.Index, job.ID, job.Name)

			if err = mongodb.NewReleasePlanColl().UpdateByID(ctx, planID, plan); err != nil {
				err = errors.Wrap(err, "update plan")
				log.Error(err)
				return err
			}

			go func() {
				if err := mongodb.NewReleasePlanLogColl().Create(&models.ReleasePlanLog{
					PlanID:     planID,
					Username:   "系统",
					Account:    "",
					Verb:       VerbExecute,
					TargetName: args.Name,
					TargetType: TargetTypeReleaseJob,
					CreatedAt:  time.Now().Unix(),
				}); err != nil {
					log.Errorf("create release plan log error: %v", err)
				}
			}()
		}
	}

	return nil
}

type SkipReleaseJobArgs struct {
	ID   string      `json:"id"`
	Name string      `json:"name"`
	Type string      `json:"type"`
	Spec interface{} `json:"spec"`
}

func SkipReleaseJob(c *handler.Context, planID string, args *SkipReleaseJobArgs, isSystemAdmin bool) error {
	approveLock := getLock(planID)
	approveLock.Lock()
	defer approveLock.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	plan, err := mongodb.NewReleasePlanColl().GetByID(ctx, planID)
	if err != nil {
		return errors.Wrap(err, "get plan")
	}

	if plan.Status != config.StatusExecuting {
		return errors.Errorf("plan status is %s, can not skip", plan.Status)
	}

	if !(plan.StartTime == 0 && plan.EndTime == 0) {
		now := time.Now().Unix()
		if now < plan.StartTime || now > plan.EndTime {
			return errors.Errorf("plan is not in the release time range")
		}
	}

	if plan.ManagerID != c.UserID && !isSystemAdmin {
		return errors.Errorf("only manager can skip")
	}

	skipper, err := NewReleaseJobSkipper(&SkipReleaseJobContext{
		AuthResources: c.Resources,
		UserID:        c.UserID,
		Account:       c.Account,
		UserName:      c.UserName,
	}, args)
	if err != nil {
		return errors.Wrap(err, "new release job skipper")
	}
	if err = skipper.Skip(plan); err != nil {
		return errors.Wrap(err, "skip")
	}

	plan.UpdatedBy = c.UserName
	plan.UpdateTime = time.Now().Unix()

	if checkReleasePlanJobsAllDone(plan) {
		//plan.ExecutingTime = time.Now().Unix()
		plan.SuccessTime = time.Now().Unix()
		plan.Status = config.StatusSuccess
	}

	if err = mongodb.NewReleasePlanColl().UpdateByID(ctx, planID, plan); err != nil {
		return errors.Wrap(err, "update plan")
	}

	go func() {
		if err := mongodb.NewReleasePlanLogColl().Create(&models.ReleasePlanLog{
			PlanID:     planID,
			Username:   c.UserName,
			Account:    c.Account,
			Verb:       VerbSkip,
			TargetName: args.Name,
			TargetType: TargetTypeReleaseJob,
			CreatedAt:  time.Now().Unix(),
		}); err != nil {
			log.Errorf("create release plan log error: %v", err)
		}
	}()

	return nil
}

func UpdateReleasePlanStatus(c *handler.Context, planID, status string, isSystemAdmin bool) error {
	approveLock := getLock(planID)
	approveLock.Lock()
	defer approveLock.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	plan, err := mongodb.NewReleasePlanColl().GetByID(ctx, planID)
	if err != nil {
		return errors.Wrap(err, "get plan")
	}

	if c.UserID != plan.ManagerID && !isSystemAdmin {
		return errors.Errorf("only manager can update plan status")
	}

	if !lo.Contains(config.ReleasePlanStatusMap[plan.Status], config.ReleasePlanStatus(status)) {
		return errors.Errorf("can't convert plan status %s to %s", plan.Status, status)
	}

	userInfo, err := user.New().GetUserByID(c.UserID)
	if err != nil {
		return errors.Wrap(err, "get user")
	}

	detail := ""

	// target status check and update
	switch config.ReleasePlanStatus(status) {
	case config.StatusPlanning:
		for _, job := range plan.Jobs {
			job.LastStatus = job.Status
			job.Status = config.ReleasePlanJobStatusTodo
			job.Updated = false
		}
	case config.StatusExecuting:
		if plan.Approval != nil && plan.Approval.Enabled == true && plan.Approval.Status != config.StatusPassed {
			return errors.Errorf("approval status is %s, can not execute", plan.Approval.Status)
		}

		if err := upsertReleasePlanCron(plan.ID.Hex(), plan.Name, plan.Index, plan.ScheduleExecuteTime); err != nil {
			return errors.Wrap(err, "upsert release plan cron")
		}

		plan.ExecutingTime = time.Now().Unix()
		setReleaseJobsForExecuting(plan)
	case config.StatusWaitForApprove:
		if err := clearApprovalData(plan.Approval); err != nil {
			return errors.Wrap(err, "clear approval data")
		}
		if err := createApprovalInstance(plan, userInfo.Phone); err != nil {
			return errors.Wrap(err, "create approval instance")
		}
	case config.StatusCancel:
		// set executing status final time
		// plan.ExecutingTime = time.Now().Unix()
	}

	// original status check and update
	switch plan.Status {
	// other status will not be done by this function
	case config.StatusPlanning:
		if len(plan.Jobs) == 0 {
			return errors.Errorf("plan must not have no jobs")
		}
		plan.PlanningTime = time.Now().Unix()
	}
	plan.Status = config.ReleasePlanStatus(status)

	if err = mongodb.NewReleasePlanColl().UpdateByID(ctx, planID, plan); err != nil {
		return errors.Wrap(err, "update plan")
	}

	go func() {
		if err := mongodb.NewReleasePlanLogColl().Create(&models.ReleasePlanLog{
			PlanID:     planID,
			Username:   c.UserName,
			Account:    c.Account,
			Verb:       VerbUpdate,
			TargetName: TargetTypeReleasePlanStatus,
			TargetType: TargetTypeReleasePlanStatus,
			Detail:     detail,
			Before:     plan.Status,
			After:      status,
			CreatedAt:  time.Now().Unix(),
		}); err != nil {
			log.Errorf("create release plan log error: %v", err)
		}
	}()

	return nil
}

type ApproveRequest struct {
	Approve bool   `json:"approve"`
	Comment string `json:"comment"`
}

func ApproveReleasePlan(c *handler.Context, planID string, req *ApproveRequest) error {
	approveLock := getLock(planID)
	approveLock.Lock()
	defer approveLock.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	plan, err := mongodb.NewReleasePlanColl().GetByID(ctx, planID)
	if err != nil {
		return errors.Wrap(err, "get plan")
	}

	if plan.Status != config.StatusWaitForApprove {
		return errors.Errorf("plan status is %s, can not approve", plan.Status)
	}

	if plan.Approval == nil || plan.Approval.Type != config.NativeApproval || plan.Approval.NativeApproval == nil {
		return errors.Errorf("plan approval is nil or not native approval")
	}

	approvalKey := plan.Approval.NativeApproval.InstanceCode
	_, ok := approvalservice.GlobalApproveMap.GetApproval(approvalKey)
	if !ok {
		// restore data after restart aslan
		log.Infof("updateNativeApproval: approval instance code %s not found, set it", plan.Approval.NativeApproval.InstanceCode)
		approvalUsers, _ := geneFlatNativeApprovalUsers(plan.Approval.NativeApproval)
		originApprovalUsers := plan.Approval.NativeApproval.ApproveUsers
		plan.Approval.NativeApproval.ApproveUsers = approvalUsers
		approvalservice.GlobalApproveMap.SetApproval(plan.Approval.NativeApproval.InstanceCode, plan.Approval.NativeApproval)
		plan.Approval.NativeApproval.ApproveUsers = originApprovalUsers
	}

	approval, err := approvalservice.GlobalApproveMap.DoApproval(approvalKey, c.UserName, c.UserID, req.Comment, req.Approve)
	if err != nil {
		return errors.Wrap(err, "do approval")
	}

	plan.Approval.NativeApproval = approval
	approved, rejected, _, err := approvalservice.GlobalApproveMap.IsApproval(approvalKey)
	if err != nil {
		return errors.Wrap(err, "is approval")
	}
	if rejected {
		plan.Approval.Status = config.StatusReject
	} else if approved {
		plan.Approval.Status = config.StatusPassed
	}
	var planLog *models.ReleasePlanLog
	switch plan.Approval.Status {
	case config.StatusPassed:
		planLog = &models.ReleasePlanLog{
			PlanID:     planID,
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

		if err := upsertReleasePlanCron(plan.ID.Hex(), plan.Name, plan.Index, plan.ScheduleExecuteTime); err != nil {
			err = errors.Wrap(err, "upsert release plan cron")
			log.Error(err)
		}

		setReleaseJobsForExecuting(plan)
	case config.StatusReject:
		planLog = &models.ReleasePlanLog{
			PlanID:    planID,
			Detail:    "审批被拒绝",
			CreatedAt: time.Now().Unix(),
		}
	}
	if err = mongodb.NewReleasePlanColl().UpdateByID(ctx, planID, plan); err != nil {
		return errors.Wrap(err, "update plan")
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

func clearApprovalData(approval *models.Approval) error {
	if approval == nil {
		return errors.New("nil approval")
	}
	approval.Status = ""
	switch approval.Type {
	case config.LarkApproval:
		if approval.LarkApproval == nil {
			return errors.New("nil lark approval")
		}
		for _, node := range approval.LarkApproval.ApprovalNodes {
			node.RejectOrApprove = ""
			for _, user := range node.ApproveUsers {
				user.RejectOrApprove = ""
				user.OperationTime = 0
				user.Comment = ""
			}
		}
	case config.DingTalkApproval:
		if approval.DingTalkApproval == nil {
			return errors.New("nil dingtalk approval")
		}
		for _, node := range approval.DingTalkApproval.ApprovalNodes {
			node.RejectOrApprove = ""
			for _, user := range node.ApproveUsers {
				user.RejectOrApprove = ""
				user.OperationTime = 0
				user.Comment = ""
			}
		}
	case config.WorkWXApproval:
		if approval.WorkWXApproval == nil {
			return errors.New("nil workwx approval")
		}
		approval.WorkWXApproval.ApprovalNodeDetails = nil
	case config.NativeApproval:
		if approval.NativeApproval == nil {
			return errors.New("nil native approval")
		}
		for _, user := range approval.NativeApproval.ApproveUsers {
			user.RejectOrApprove = ""
			user.OperationTime = 0
			user.Comment = ""
		}
	}
	return nil
}

func checkReleasePlanJobsAllDone(plan *models.ReleasePlan) bool {
	for _, job := range plan.Jobs {
		if job.Status != config.ReleasePlanJobStatusDone && job.Status != config.ReleasePlanJobStatusSkipped {
			return false
		}
	}
	return true
}

func setReleaseJobsForExecuting(plan *models.ReleasePlan) {
	for _, job := range plan.Jobs {
		if job.LastStatus == config.ReleasePlanJobStatusDone && !job.Updated {
			job.Status = config.ReleasePlanJobStatusDone
			continue
		}
		job.Status = config.ReleasePlanJobStatusTodo
		job.ExecutedBy = ""
		job.ExecutedTime = 0
	}
}

func cronJobToSchedule(input *commonmodels.Cronjob) *commonmodels.Schedule {
	return &commonmodels.Schedule{
		ID:              input.ID,
		Number:          input.Number,
		Frequency:       input.Frequency,
		Time:            input.Time,
		MaxFailures:     input.MaxFailure,
		ReleasePlanArgs: input.ReleasePlanArgs,
		Type:            config.ScheduleType(input.JobType),
		Cron:            input.Cron,
		Enabled:         input.Enabled,
	}
}

type ListReleasePlanType string

const (
	ListReleasePlanTypeName        ListReleasePlanType = "name"
	ListReleasePlanTypeManager     ListReleasePlanType = "manager"
	ListReleasePlanTypeSuccessTime ListReleasePlanType = "success_time"
	ListReleasePlanTypeUpdateTime  ListReleasePlanType = "update_time"
	ListReleasePlanTypeStatus      ListReleasePlanType = "status"
)

type ListReleasePlanOption struct {
	PageNum  int64               `form:"pageNum" binding:"required"`
	PageSize int64               `form:"pageSize" binding:"required"`
	Type     ListReleasePlanType `form:"type" binding:"required"`
	Keyword  string              `form:"keyword"`
}

type ListReleasePlanResp struct {
	List  []*models.ReleasePlan `json:"list"`
	Total int64                 `json:"total"`
}

func ListReleasePlans(opt *ListReleasePlanOption) (*ListReleasePlanResp, error) {
	var (
		list  []*commonmodels.ReleasePlan
		total int64
		err   error
	)
	switch opt.Type {
	case ListReleasePlanTypeName:
		list, total, err = mongodb.NewReleasePlanColl().ListByOptions(&mongodb.ListReleasePlanOption{
			Name:           opt.Keyword,
			IsSort:         true,
			PageNum:        opt.PageNum,
			PageSize:       opt.PageSize,
			ExcludedFields: []string{"jobs", "logs"},
		})
	case ListReleasePlanTypeManager:
		list, total, err = mongodb.NewReleasePlanColl().ListByOptions(&mongodb.ListReleasePlanOption{
			Manager:        opt.Keyword,
			IsSort:         true,
			PageNum:        opt.PageNum,
			PageSize:       opt.PageSize,
			ExcludedFields: []string{"jobs", "logs"},
		})
	case ListReleasePlanTypeSuccessTime:
		timeArr := strings.Split(opt.Keyword, "-")
		if len(timeArr) != 2 {
			return nil, errors.New("invalid success time range")
		}

		timeStart := int64(0)
		timeEnd := int64(0)
		timeStart, err = strconv.ParseInt(timeArr[0], 10, 64)
		if err != nil {
			return nil, errors.Wrap(err, "invalid success time start")
		}
		timeEnd, err = strconv.ParseInt(timeArr[1], 10, 64)
		if err != nil {
			return nil, errors.Wrap(err, "invalid success time end")
		}

		list, total, err = mongodb.NewReleasePlanColl().ListByOptions(&mongodb.ListReleasePlanOption{
			SuccessTimeStart: timeStart,
			SuccessTimeEnd:   timeEnd,
			IsSort:           true,
			PageNum:          opt.PageNum,
			PageSize:         opt.PageSize,
			ExcludedFields:   []string{"jobs", "logs"},
		})
	case ListReleasePlanTypeUpdateTime:
		timeArr := strings.Split(opt.Keyword, "-")
		if len(timeArr) != 2 {
			return nil, errors.New("invalid update time range")
		}

		timeStart := int64(0)
		timeEnd := int64(0)
		timeStart, err = strconv.ParseInt(timeArr[0], 10, 64)
		if err != nil {
			return nil, errors.Wrap(err, "invalid update time start")
		}
		timeEnd, err = strconv.ParseInt(timeArr[1], 10, 64)
		if err != nil {
			return nil, errors.Wrap(err, "invalid update time end")
		}

		list, total, err = mongodb.NewReleasePlanColl().ListByOptions(&mongodb.ListReleasePlanOption{
			UpdateTimeStart: timeStart,
			UpdateTimeEnd:   timeEnd,
			IsSort:          true,
			PageNum:         opt.PageNum,
			PageSize:        opt.PageSize,
			ExcludedFields:  []string{"jobs", "logs"},
		})
	case ListReleasePlanTypeStatus:
		list, total, err = mongodb.NewReleasePlanColl().ListByOptions(&mongodb.ListReleasePlanOption{
			Status:         config.ReleasePlanStatus(opt.Keyword),
			IsSort:         true,
			PageNum:        opt.PageNum,
			PageSize:       opt.PageSize,
			ExcludedFields: []string{"jobs", "logs"},
		})
	}
	if err != nil {
		return nil, errors.Wrap(err, "SearchReleasePlans")
	}

	return &ListReleasePlanResp{
		List:  list,
		Total: total,
	}, nil
}
