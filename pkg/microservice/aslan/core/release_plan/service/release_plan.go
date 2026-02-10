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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/koderover/zadig/v2/pkg/types"
	"github.com/pkg/errors"
	"github.com/samber/lo"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models/msg_queue"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	approvalservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/approval"
	dingservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/dingtalk"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/webhooknotify"
	runtimeWorkflowController "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/workflowcontroller"
	workwxservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/workwx"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/workflow/service/workflow/controller"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/shared/client/user"
	"github.com/koderover/zadig/v2/pkg/shared/handler"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/lark"
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
		// release job will be linted when we finish planning instead of saving
		// if err := lintReleaseJob(job.Type, job.Spec); err != nil {
		// 	return errors.Errorf("lintReleaseJob %s error: %v", job.Name, err)
		// }
		job.ReleaseJobRuntime = models.ReleaseJobRuntime{}
		job.ID = uuid.New().String()
	}

	if args.Approval != nil {
		if err := lintApproval(args.Approval); err != nil {
			return errors.Errorf("lintApproval error: %v", err)
		}
		if args.Approval.Type == config.LarkApproval || args.Approval.Type == config.LarkApprovalIntl {
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
	args.Status = config.ReleasePlanStatusPlanning

	args.InstanceCode, err = generateInstanceCode(args)
	if err != nil {
		return errors.Wrap(err, "generate instance code")
	}

	hookSetting, err := mongodb.NewSystemSettingColl().GetReleasePlanHookSetting()
	if err != nil {
		fmtErr := fmt.Errorf("failed get release plan hook setting, err: %v", err)
		log.Error(fmtErr)
		return fmtErr
	}
	args.HookSettings = hookSetting.ToHookSettings()

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

func upsertReleasePlanCron(id, name string, index int64, status config.ReleasePlanStatus, ScheduleExecuteTime int64) error {
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

	if status != config.ReleasePlanStatusExecuting {
		// delete cron job if status is not executing
		if found {
			err = commonrepo.NewCronjobColl().Delete(&commonrepo.CronjobDeleteOption{
				IDList: []string{releasePlanCron.ID.Hex()},
			})
			if err != nil {
				fmtErr := fmt.Errorf("Failed to delete release plan schedule job %s, error: %w", releasePlanCron.ID.Hex(), err)
				log.Error(fmtErr)
			}

			payload = &commonservice.CronjobPayload{
				Name:         releasePlanCronName,
				JobType:      setting.ReleasePlanCronjob,
				Action:       setting.TypeEnableCronjob,
				ScheduleType: setting.UnixStampSchedule,
				DeleteList:   []string{releasePlanCron.ID.Hex()},
			}
		}
	} else {
		// upsert cron job if status is executing
		if found {
			origEnabled := releasePlanCron.Enabled
			releasePlanCron.Enabled = enable
			releasePlanCron.ReleasePlanArgs = &commonmodels.ReleasePlanArgs{
				ID:    id,
				Name:  name,
				Index: index,
			}
			releasePlanCron.JobType = setting.UnixStampSchedule
			releasePlanCron.UnixStamp = ScheduleExecuteTime

			if origEnabled && !enable {
				// need to disable cronjob
				err = commonrepo.NewCronjobColl().Delete(&commonrepo.CronjobDeleteOption{
					IDList: []string{releasePlanCron.ID.Hex()},
				})
				if err != nil {
					fmtErr := fmt.Errorf("Failed to delete cron job %s, error: %w", releasePlanCron.ID.Hex(), err)
					log.Error(fmtErr)
				}

				payload = &commonservice.CronjobPayload{
					Name:         releasePlanCronName,
					JobType:      setting.ReleasePlanCronjob,
					Action:       setting.TypeEnableCronjob,
					ScheduleType: setting.UnixStampSchedule,
					DeleteList:   []string{releasePlanCron.ID.Hex()},
				}
			} else if !origEnabled && enable || origEnabled && enable {
				err = commonrepo.NewCronjobColl().Upsert(releasePlanCron)
				if err != nil {
					fmtErr := fmt.Errorf("Failed to upsert cron job, error: %w", err)
					log.Error(fmtErr)
					return err
				}

				payload = &commonservice.CronjobPayload{
					Name:         releasePlanCronName,
					JobType:      setting.ReleasePlanCronjob,
					Action:       setting.TypeEnableCronjob,
					ScheduleType: setting.UnixStampSchedule,
					JobList:      []*commonmodels.Schedule{cronJobToSchedule(releasePlanCron)},
				}
			} else {
				// !origEnabled && !enable
				return nil
			}
		} else {
			if !enable {
				return nil
			}

			input := &commonmodels.Cronjob{
				Enabled:   enable,
				Name:      releasePlanCronName,
				Type:      setting.ReleasePlanCronjob,
				JobType:   setting.UnixStampSchedule,
				UnixStamp: ScheduleExecuteTime,
				ReleasePlanArgs: &commonmodels.ReleasePlanArgs{
					ID:    id,
					Name:  name,
					Index: index,
				},
			}

			err = commonrepo.NewCronjobColl().Upsert(input)
			if err != nil {
				fmtErr := fmt.Errorf("Failed to upsert cron job, error: %w", err)
				log.Error(fmtErr)
				return err
			}
			payload = &commonservice.CronjobPayload{
				Name:         releasePlanCronName,
				JobType:      setting.ReleasePlanCronjob,
				Action:       setting.TypeEnableCronjob,
				ScheduleType: setting.UnixStampSchedule,
				JobList:      []*commonmodels.Schedule{cronJobToSchedule(input)},
			}
		}
	}

	if payload == nil {
		return nil
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
			releasePlanJob.Spec = nil
		}
	}

	return releasePlan, nil
}

type ReleasePlanLogI18N struct {
	VerbI18Map       map[string]string `json:"verb_i18_map"`
	TargetTypeI18Map map[string]string `json:"target_type_i18_map"`
	DetailI18Map     map[string]string `json:"detail_i18_map"`
	UserNameI18Map   map[string]string `json:"user_name_i18_map"`
}

type GetReleasePlanLogsResponse struct {
	I18N *ReleasePlanLogI18N      `json:"i18n"`
	List []*models.ReleasePlanLog `json:"list"`
}

func GetReleasePlanLogs(id string) (*GetReleasePlanLogsResponse, error) {
	logs, err := mongodb.NewReleasePlanLogColl().ListByOptions(&mongodb.ListReleasePlanLogOption{
		PlanID: id,
		IsSort: true,
	})
	if err != nil {
		return nil, errors.Wrap(err, "get release plan logs")
	}

	return &GetReleasePlanLogsResponse{
		List: logs,
		I18N: &ReleasePlanLogI18N{
			VerbI18Map:       VerbI18nMap,
			TargetTypeI18Map: TargetTypeI18nMap,
			DetailI18Map:     DetailI18nMap,
			UserNameI18Map:   UserNameI18nMap,
		},
	}, nil
}

func DeleteReleasePlan(c *gin.Context, username, id string) error {
	info, err := mongodb.NewReleasePlanColl().GetByID(context.Background(), id)
	if err != nil {
		return errors.Wrap(err, "get plan")
	}
	internalhandler.InsertOperationLog(c, username, "", "删除", "发布计划", info.Name, info.Name, "", types.RequestBodyTypeJSON, log.SugaredLogger())

	releasePlanCronName := util.GetReleasePlanCronName(id, info.Name, info.Index)
	releasePlanCron, err := commonrepo.NewCronjobColl().GetByName(releasePlanCronName, setting.ReleasePlanCronjob)
	if err != nil {
		if err != mongo.ErrNoDocuments && err != mongo.ErrNilDocument {
			return e.ErrUpsertCronjob.AddErr(fmt.Errorf("failed to get release plan cron job, err: %w", err))
		}
	} else {
		err = commonrepo.NewCronjobColl().Delete(&commonrepo.CronjobDeleteOption{
			IDList: []string{releasePlanCron.ID.Hex()},
		})
		if err != nil {
			fmtErr := fmt.Errorf("Failed to delete release plan schedule job %s, error: %w", releasePlanCron.ID.Hex(), err)
			log.Error(fmtErr)
		}
	}

	return mongodb.NewReleasePlanColl().DeleteByID(context.Background(), id)
}

type UpdateReleasePlanVerb string

const (
	ActionUpdateReleaseJob          = "update_release_job"
	ActionCreateReleaseJob          = "create_release_job"
	ActionDeleteReleaseJob          = "delete_release_job"
	ActionUpdateApproval            = "update_approval"
	ActionDeleteApproval            = "delete_approval"
	ActionUpdateName                = "update_name"
	ActionUpdateManager             = "update_manager"
	ActionUpdateTimeRange           = "update_time_range"
	ActionUpdateScheduleExecuteTime = "update_schedule_execute_time"
	ActionUpdateDescription         = "update_description"
	ActionUpdateJiraSprint          = "update_jira_sprint"
)

type UpdateReleasePlanArgs struct {
	Verb UpdateReleasePlanVerb `json:"verb"`
	Spec interface{}           `json:"spec"`
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

	if plan.Status != config.ReleasePlanStatusPlanning {
		return errors.Errorf("plan status is %s, can not update", plan.Status)
	}

	updater, err := NewPlanUpdater(args)
	if err != nil {
		return errors.Wrap(err, "new plan updater")
	}
	// note that for updating release job and creating new release job, the linting process is when we finish planning, not saving the draft.
	if err = updater.Lint(); err != nil {
		return errors.Wrap(err, "lint")
	}
	before, after, err := updater.Update(plan)
	if err != nil {
		return errors.Wrap(err, "update")
	}

	plan.UpdatedBy = c.UserName
	plan.UpdateTime = time.Now().Unix()

	hookSetting, err := mongodb.NewSystemSettingColl().GetReleasePlanHookSetting()
	if err != nil {
		fmtErr := fmt.Errorf("failed get release plan hook setting, err: %v", err)
		log.Error(fmtErr)
		return fmtErr
	}
	plan.HookSettings = hookSetting.ToHookSettings()

	instanceCode, err := generateInstanceCode(plan)
	if err != nil {
		fmtErr := fmt.Errorf("failed generate instance code, err: %v", err)
		log.Error(fmtErr)
		return fmtErr
	}
	plan.InstanceCode = instanceCode

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

func GetReleasePlanJobDetail(planID, jobID string) (*commonmodels.ReleaseJob, error) {
	releasePlan, err := mongodb.NewReleasePlanColl().GetByID(context.Background(), planID)
	if err != nil {
		return nil, errors.Wrap(err, "GetReleasePlan")
	}

	for _, releasePlanJob := range releasePlan.Jobs {
		if releasePlanJob.ID == jobID {
			if releasePlanJob.Type == config.JobWorkflow {
				spec := new(models.WorkflowReleaseJobSpec)
				if err := models.IToi(releasePlanJob.Spec, spec); err != nil {
					return nil, fmt.Errorf("invalid spec for job: %s. decode error: %s", releasePlanJob.Name, err)
				}
				if spec.Workflow == nil {
					return nil, fmt.Errorf("workflow is nil")
				}

				workflowController := controller.CreateWorkflowController(spec.Workflow)
				if err := workflowController.UpdateWithLatestWorkflow(nil); err != nil {
					log.Errorf("cannot merge workflow %s's input with the latest workflow settings, the error is: %v", spec.Workflow.Name, err)
					return nil, e.ErrPresetWorkflow.AddDesc(err.Error())
				}

				spec.Workflow = workflowController.WorkflowV4
				releasePlanJob.Spec = spec
			}

			return releasePlanJob, nil
		}
	}

	return nil, fmt.Errorf("failed to find release plan job with id: %s. Job does not exist", jobID)
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

	if plan.Status != config.ReleasePlanStatusExecuting {
		return errors.Errorf("plan status is %s, can not execute", plan.Status)
	}

	if !(plan.StartTime == 0 && plan.EndTime == 0) {
		now := time.Now().Unix()
		if now < plan.StartTime || now > plan.EndTime {
			if now > plan.EndTime {
				plan.Status = config.ReleasePlanStatusTimeoutForWindow
				if err = mongodb.NewReleasePlanColl().UpdateByID(ctx, planID, plan); err != nil {
					return errors.Wrap(err, "update plan")
				}
			}
			return errors.Errorf("plan is not in the release time range")
		}
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

	sendWebhook := false
	hookSetting, err := mongodb.NewSystemSettingColl().GetReleasePlanHookSetting()
	if err != nil {
		fmtErr := fmt.Errorf("failed get release plan hook setting, err: %v", err)
		log.Error(fmtErr)
	}

	if checkReleasePlanJobsAllDone(plan) {
		plan.Status = config.ReleasePlanStatusSuccess

		nextStatus, shouldWait := waitForExternalCheck(plan, hookSetting)
		if shouldWait {
			plan.Status = *nextStatus
		} else {
			plan.SuccessTime = time.Now().Unix()
		}

		sendWebhook = true
	}

	if err = mongodb.NewReleasePlanColl().UpdateByID(ctx, planID, plan); err != nil {
		return errors.Wrap(err, "update plan")
	}

	if sendWebhook {
		if err := sendReleasePlanHook(plan, hookSetting); err != nil {
			log.Errorf("send release plan hook error: %v", err)
		}
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

type RetryReleaseJobArgs struct {
	ID   string      `json:"id"`
	Name string      `json:"name"`
	Type string      `json:"type"`
	Spec interface{} `json:"spec"`
}

func RetryReleaseJob(c *handler.Context, planID string, args *RetryReleaseJobArgs, isSystemAdmin bool) error {
	approveLock := getLock(planID)
	approveLock.Lock()
	defer approveLock.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	plan, err := mongodb.NewReleasePlanColl().GetByID(ctx, planID)
	if err != nil {
		return errors.Wrap(err, "get plan")
	}

	if plan.Status != config.ReleasePlanStatusExecuting && plan.Status != config.ReleasePlanStatusSuccess {
		return errors.Errorf("plan status is %s, can not execute", plan.Status)
	}

	if !(plan.StartTime == 0 && plan.EndTime == 0) {
		now := time.Now().Unix()
		if now < plan.StartTime || now > plan.EndTime {
			if now > plan.EndTime {
				plan.Status = config.ReleasePlanStatusTimeoutForWindow
				if err = mongodb.NewReleasePlanColl().UpdateByID(ctx, planID, plan); err != nil {
					return errors.Wrap(err, "update plan")
				}
			}
			return errors.Errorf("plan is not in the release time range")
		}
	}

	retryer, err := NewReleaseJobRetryer(&RetryReleaseJobContext{
		AuthResources: c.Resources,
		UserID:        c.UserID,
		Account:       c.Account,
		UserName:      c.UserName,
	}, args)
	if err != nil {
		return errors.Wrap(err, "new release job executor")
	}
	if err = retryer.Retry(plan); err != nil {
		return errors.Wrap(err, "execute")
	}

	plan.UpdatedBy = c.UserName
	plan.UpdateTime = time.Now().Unix()

	sendWebhook := false
	hookSetting, err := mongodb.NewSystemSettingColl().GetReleasePlanHookSetting()
	if err != nil {
		fmtErr := fmt.Errorf("failed get release plan hook setting, err: %v", err)
		log.Error(fmtErr)
	}

	if checkReleasePlanJobsAllDone(plan) {
		plan.Status = config.ReleasePlanStatusSuccess

		nextStatus, shouldWait := waitForExternalCheck(plan, hookSetting)
		if shouldWait {
			plan.Status = *nextStatus
		} else {
			plan.SuccessTime = time.Now().Unix()
		}

		sendWebhook = true
	}

	if err = mongodb.NewReleasePlanColl().UpdateByID(ctx, planID, plan); err != nil {
		return errors.Wrap(err, "update plan")
	}

	if sendWebhook {
		if err := sendReleasePlanHook(plan, hookSetting); err != nil {
			log.Errorf("send release plan hook error: %v", err)
		}
	}

	go func() {
		if err := mongodb.NewReleasePlanLogColl().Create(&models.ReleasePlanLog{
			PlanID:     planID,
			Username:   c.UserName,
			Account:    c.Account,
			Verb:       VerbRetry,
			TargetName: args.Name,
			TargetType: TargetTypeReleaseJob,
			CreatedAt:  time.Now().Unix(),
		}); err != nil {
			log.Errorf("create release plan log error: %v", err)
		}
	}()

	return nil
}

func ScheduleExecuteReleasePlan(c *handler.Context, planID, jobID string) error {
	approveLock := getLock(planID)
	approveLock.Lock()
	defer approveLock.Unlock()

	// check if the job is already executed
	jobObjectID, err := primitive.ObjectIDFromHex(jobID)
	if err != nil {
		return errors.Wrap(err, "invalid job ID")
	}
	_, err = commonrepo.NewCronjobColl().GetByID(jobObjectID)
	if err != nil {
		if !mongodb.IsErrNoDocuments(err) {
			err = fmt.Errorf("Failed to get release job schedule job %s, error: %v", jobID, err)
			log.Error(err)
			return err
		} else {
			err = fmt.Errorf("Release job schedule job %s not found", jobID)
			log.Error(err)
			return err
		}
	}

	// delete the schedule job after executed
	err = commonrepo.NewCronjobColl().Delete(&commonrepo.CronjobDeleteOption{
		IDList: []string{jobID},
	})
	if err != nil {
		log.Errorf("Failed to delete release job schedule job %s, error: %v", jobID, err)
	}

	ctx := context.Background()
	plan, err := mongodb.NewReleasePlanColl().GetByID(ctx, planID)
	if err != nil {
		err = errors.Wrap(err, "get plan")
		log.Error(err)
		return err
	}

	if plan.Status != config.ReleasePlanStatusExecuting {
		err = errors.Errorf("plan ID is %s, name is %s, index is %d, status is %s, can not execute", plan.ID.Hex(), plan.Name, plan.Index, plan.Status)
		log.Error(err)
		return err
	}

	if !(plan.StartTime == 0 && plan.EndTime == 0) {
		now := time.Now().Unix()
		if now < plan.StartTime || now > plan.EndTime {
			err = errors.Errorf("plan ID is %s, name is %s, index is %d, it's not in the release time range", plan.ID.Hex(), plan.Name, plan.Index)
			log.Error(err)
			return err
		}
	}

	if plan.Approval != nil && plan.Approval.Enabled == true && plan.Approval.Status != config.StatusPassed {
		err = errors.Errorf("plan ID is %s, name is %s, index is %d, it's approval status is %s, can not execute", plan.ID.Hex(), plan.Name, plan.Index, plan.Approval.Status)
		log.Error(err)
		return err
	}

	log.Infof("schedule execute release plan, plan ID: %s, name: %s, index: %d", plan.ID.Hex(), plan.Name, plan.Index)

	sendWebhook := false
	hookSetting, err := mongodb.NewSystemSettingColl().GetReleasePlanHookSetting()
	if err != nil {
		fmtErr := fmt.Errorf("failed get release plan hook setting, err: %v", err)
		log.Error(fmtErr)
	}

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

			go func() {
				if err := mongodb.NewReleasePlanLogColl().Create(&models.ReleasePlanLog{
					PlanID:     planID,
					Username:   UserNameSystem,
					Account:    "",
					Verb:       VerbExecute,
					TargetName: args.Name,
					TargetType: TargetTypeReleaseJob,
					CreatedAt:  time.Now().Unix(),
				}); err != nil {
					log.Errorf("create release plan log error: %v", err)
				}
			}()

			executor, err := NewReleaseJobExecutor(&ExecuteReleaseJobContext{
				AuthResources: c.Resources,
				UserID:        c.UserID,
				Account:       "",
				UserName:      UserNameSystem,
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

			plan.UpdatedBy = UserNameSystem
			plan.UpdateTime = time.Now().Unix()

			if checkReleasePlanJobsAllDone(plan) {
				plan.SuccessTime = time.Now().Unix()
				plan.Status = config.ReleasePlanStatusSuccess

				nextStatus, shouldWait := waitForExternalCheck(plan, hookSetting)
				if shouldWait {
					plan.Status = *nextStatus
				} else {
					plan.SuccessTime = time.Now().Unix()
				}

				sendWebhook = true
			}

			log.Infof("schedule execute release job, plan ID: %s, name: %s, index: %d, job ID: %s, job name: %s", plan.ID.Hex(), plan.Name, plan.Index, job.ID, job.Name)

			if err = mongodb.NewReleasePlanColl().UpdateByID(ctx, planID, plan); err != nil {
				err = errors.Wrap(err, "update plan")
				log.Error(err)
				return err
			}
		}
	}

	if sendWebhook {
		if err := sendReleasePlanHook(plan, hookSetting); err != nil {
			log.Errorf("send release plan hook error: %v", err)
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

	if plan.Status != config.ReleasePlanStatusExecuting {
		return errors.Errorf("plan status is %s, can not skip", plan.Status)
	}

	if !(plan.StartTime == 0 && plan.EndTime == 0) {
		now := time.Now().Unix()
		if now < plan.StartTime || now > plan.EndTime {
			return errors.Errorf("plan is not in the release time range")
		}
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

	sendWebhook := false
	hookSetting, err := mongodb.NewSystemSettingColl().GetReleasePlanHookSetting()
	if err != nil {
		fmtErr := fmt.Errorf("failed get release plan hook setting, err: %v", err)
		log.Error(fmtErr)
	}

	if checkReleasePlanJobsAllDone(plan) {
		plan.Status = config.ReleasePlanStatusSuccess

		nextStatus, shouldWait := waitForExternalCheck(plan, hookSetting)
		if shouldWait {
			plan.Status = *nextStatus
		} else {
			plan.SuccessTime = time.Now().Unix()
		}
	}

	if err = mongodb.NewReleasePlanColl().UpdateByID(ctx, planID, plan); err != nil {
		return errors.Wrap(err, "update plan")
	}

	if sendWebhook {
		if err := sendReleasePlanHook(plan, hookSetting); err != nil {
			log.Errorf("send release plan hook error: %v", err)
		}
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

func UpdateReleasePlanStatus(c *handler.Context, planID, targetStatus string, isSystemAdmin bool) error {
	approveLock := getLock(planID)
	approveLock.Lock()
	defer approveLock.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	plan, err := mongodb.NewReleasePlanColl().GetByID(ctx, planID)
	if err != nil {
		return errors.Wrap(err, "get plan")
	}

	if plan.Status == config.ReleasePlanStatus(targetStatus) {
		return fmt.Errorf("current status can not equal to target status %s", targetStatus)
	}

	if c.UserID != plan.ManagerID && !isSystemAdmin {
		return errors.Errorf("only manager can update plan status")
	}

	if !lo.Contains(config.ReleasePlanStatusMap[plan.Status], config.ReleasePlanStatus(targetStatus)) {
		return errors.Errorf("can't convert plan status %s to %s", plan.Status, targetStatus)
	}

	userInfo, err := user.New().GetUserByID(c.UserID)
	if err != nil {
		return errors.Wrap(err, "get user")
	}

	detail := ""

	sendWebhook := false
	hookSetting, err := mongodb.NewSystemSettingColl().GetReleasePlanHookSetting()
	if err != nil {
		fmtErr := fmt.Errorf("failed get release plan hook setting, err: %v", err)
		log.Error(fmtErr)
		return fmtErr
	}

	// original status check and update
	switch plan.Status {
	// other status will not be done by this function
	case config.ReleasePlanStatusPlanning:
		if len(plan.Jobs) == 0 {
			return errors.Errorf("plan must not have no jobs")
		}
		plan.PlanningTime = time.Now().Unix()
	case config.ReleasePlanStatusWaitForFinishPlanningExternalCheck,
		config.ReleasePlanStatusWaitForFinishPlanningExternalCheckFailed,
		// config.ReleasePlanStatusWaitForApproveExternalCheck,
		// config.ReleasePlanStatusWaitForApproveExternalCheckFailed,
		config.ReleasePlanStatusWaitForExecuteExternalCheck,
		config.ReleasePlanStatusWaitForExecuteExternalCheckFailed,
		config.ReleasePlanStatusWaitForAllDoneExternalCheck,
		config.ReleasePlanStatusWaitForAllDoneExternalCheckFailed:
		if config.ReleasePlanStatus(targetStatus) != config.ReleasePlanStatusPlanning && config.ReleasePlanStatus(targetStatus) != config.ReleasePlanStatusCancel {
			return fmt.Errorf("can't update status, current status: %s", plan.Status)
		}
	}
	plan.Status = config.ReleasePlanStatus(targetStatus)

	// target status check and update
	switch config.ReleasePlanStatus(targetStatus) {
	case config.ReleasePlanStatusPlanning:
		for _, job := range plan.Jobs {
			job.LastStatus = job.Status
			job.Status = config.ReleasePlanJobStatusTodo
			job.Updated = false
		}

		plan.HookSettings = hookSetting.ToHookSettings()

		plan.PlanningTime = time.Now().Unix()
		plan.FinishPlanningTime = 0
		plan.ApprovalTime = 0
		plan.ExecutingTime = 0
		plan.SuccessTime = 0
		plan.WaitForFinishPlanningExternalCheckTime = 0
		plan.WaitForApproveExternalCheckTime = 0
		plan.WaitForExecuteExternalCheckTime = 0
		plan.WaitForAllDoneExternalCheckTime = 0
		plan.ExternalCheckFailedReason = ""

		for _, job := range plan.Jobs {
			if job.Type == config.JobWorkflow {
				spec := new(models.WorkflowReleaseJobSpec)
				if err := models.IToi(job.Spec, spec); err != nil {
					fmtErr := fmt.Errorf("failed convert job spec to workflow release job spec, job: %+v, err: %v", job, err)
					log.Error(fmtErr)
					return fmtErr
				}

				if spec.TaskID != 0 {
					err = runtimeWorkflowController.CancelWorkflowTask(c.UserName, spec.Workflow.Name, spec.TaskID, log.SugaredLogger())
					if err != nil {
						fmtErr := fmt.Errorf("failed cancel workflow task, workflow: %s, taskID: %d, err: %v", spec.Workflow.Name, spec.TaskID, err)
						log.Error(fmtErr)
						return fmtErr
					}
				}

				spec.TaskID = 0
				spec.Status = config.StatusPrepare
				job.Spec = spec
			}

			job.Status = config.ReleasePlanJobStatusTodo
			job.LastStatus = config.ReleasePlanJobStatusTodo
			job.Updated = false
			job.ExecutedBy = ""
			job.ExecutedTime = 0
		}

		plan.InstanceCode, err = generateInstanceCode(plan)
		if err != nil {
			fmtErr := fmt.Errorf("failed generate instance code, err: %v", err)
			log.Error(fmtErr)
			return fmtErr
		}

		cancelReleasePlanApproval(c, plan)
	case config.ReleasePlanStatusFinishPlanning:
		for _, job := range plan.Jobs {
			err := lintReleaseJob(job.Type, job.Spec)
			if err != nil {
				fmtErr := fmt.Errorf("failed to lint release job, err: %v", err)
				log.Error(fmtErr)
				return fmtErr
			}
		}
		
		nextStatus, shouldWait := waitForExternalCheck(plan, hookSetting)
		if shouldWait {
			plan.Status = *nextStatus
		} else {
			plan.FinishPlanningTime = time.Now().Unix()
		}

		sendWebhook = true

	case config.ReleasePlanStatusExecuting:
		if plan.Approval != nil && plan.Approval.Enabled == true && plan.Approval.Status != config.StatusPassed {
			return errors.Errorf("approval status is %s, can not execute", plan.Approval.Status)
		}
		nextStatus, shouldWait := waitForExternalCheck(plan, hookSetting)
		if shouldWait {
			plan.Status = *nextStatus
		} else {
			plan.ExecutingTime = time.Now().Unix()
		}

		setReleaseJobsForExecuting(plan)

		sendWebhook = true
	case config.ReleasePlanStatusWaitForApprove:
		// nextStatus, shouldWait := waitForExternalCheck(plan, hookSetting)
		// if shouldWait {
		// 	plan.Status = *nextStatus
		// 	plan.ApproverID = c.UserID
		// } else {
		if err := clearApprovalData(plan.Approval); err != nil {
			return errors.Wrap(err, "clear approval data")
		}
		if err := createApprovalInstance(plan, userInfo.Phone); err != nil {
			return errors.Wrap(err, "create approval instance")
		}
		// }

		// sendWebhook = true
	case config.ReleasePlanStatusCancel:
		// set executing status final time
		// plan.ExecutingTime = time.Now().Unix()
		break
	}

	if err = mongodb.NewReleasePlanColl().UpdateByID(ctx, planID, plan); err != nil {
		return errors.Wrap(err, "update plan")
	}

	if err := upsertReleasePlanCron(plan.ID.Hex(), plan.Name, plan.Index, plan.Status, plan.ScheduleExecuteTime); err != nil {
		return errors.Wrap(err, "upsert release plan cron")
	}

	if sendWebhook {
		if err := sendReleasePlanHook(plan, hookSetting); err != nil {
			log.Errorf("send release plan hook error: %v", err)
		}
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
			After:      targetStatus,
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

	if plan.Status != config.ReleasePlanStatusWaitForApprove {
		return errors.Errorf("plan status is %s, can not approve", plan.Status)
	}

	if plan.Approval == nil || plan.Approval.Type != config.NativeApproval || plan.Approval.NativeApproval == nil {
		return errors.Errorf("plan approval is nil or not native approval")
	}

	sendWebhook := false
	hookSetting, err := mongodb.NewSystemSettingColl().GetReleasePlanHookSetting()
	if err != nil {
		fmtErr := fmt.Errorf("failed get release plan hook setting, err: %v", err)
		log.Error(fmtErr)
		return fmtErr
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
			Username:   UserNameSystem,
			Verb:       VerbUpdate,
			TargetName: TargetTypeReleasePlanStatus,
			TargetType: TargetTypeReleasePlanStatus,
			Detail:     DetailApprovalPass,
			After:      config.ReleasePlanStatusExecuting,
			CreatedAt:  time.Now().Unix(),
		}
		plan.Status = config.ReleasePlanStatusExecuting
		plan.ApprovalTime = time.Now().Unix()
		plan.ExecutingTime = time.Now().Unix()

		if err := upsertReleasePlanCron(plan.ID.Hex(), plan.Name, plan.Index, plan.Status, plan.ScheduleExecuteTime); err != nil {
			err = errors.Wrap(err, "upsert release plan cron")
			log.Error(err)
		}

		nextStatus, shouldWait := waitForExternalCheck(plan, hookSetting)
		if shouldWait {
			plan.Status = *nextStatus
		} else {
			plan.ExecutingTime = time.Now().Unix()
		}

		sendWebhook = true

		setReleaseJobsForExecuting(plan)
	case config.StatusReject:
		planLog = &models.ReleasePlanLog{
			PlanID:    planID,
			Detail:    DetailApprovalReject,
			CreatedAt: time.Now().Unix(),
		}

		plan.Status = config.ReleasePlanStatusApprovalDenied
		plan.ApprovalTime = time.Now().Unix()
	}

	if err = mongodb.NewReleasePlanColl().UpdateByID(ctx, planID, plan); err != nil {
		return errors.Wrap(err, "update plan")
	}

	if sendWebhook {
		if err := sendReleasePlanHook(plan, hookSetting); err != nil {
			log.Errorf("send release plan hook error: %v", err)
		}
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
	case config.LarkApproval, config.LarkApprovalIntl:
		if approval.LarkApproval == nil {
			return errors.New("nil lark approval")
		}
		approval.LarkApproval.ApprovalInstance = nil
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
		if job.Status != config.ReleasePlanJobStatusDone && job.Status != config.ReleasePlanJobStatusSkipped && job.Status != config.ReleasePlanJobStatusFailed {
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
		UnixStamp:       input.UnixStamp,
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
			SortBy:           mongodb.SortReleasePlanByUpdateTime,
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
			SortBy:          mongodb.SortReleasePlanByUpdateTime,
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

func GetReleasePlanHookSetting(c *handler.Context) (*models.ReleasePlanHookSettings, error) {
	hookSetting, err := mongodb.NewSystemSettingColl().GetReleasePlanHookSetting()
	if err != nil {
		return nil, errors.Wrap(err, "get release plan hook setting")
	}

	return hookSetting, nil
}

func UpdateReleasePlanHookSetting(c *handler.Context, req *models.ReleasePlanHookSettings) error {
	if err := mongodb.NewSystemSettingColl().UpdateReleasePlanHookSetting(req); err != nil {
		return errors.Wrap(err, "update release plan hook setting")
	}

	return nil
}

type ReleasePlanCallBackBody struct {
	ReleasePlanID string                                `json:"release_plan_id"`
	InstanceCode  string                                `json:"instance_code"`
	HookEvent     models.ReleasePlanHookEvent           `json:"hook_event"`
	Result        setting.ReleasePlanCallBackResultType `json:"result"`
	FailedReason  string                                `json:"failed_reason"`
}

func ReleasePlanHookCallback(c *handler.Context, callback *ReleasePlanCallBackBody) error {
	log.Infof("release plan hook callback, id: %s, instance code: %s, hook event: %s, result: %s, failed reason: %s", callback.ReleasePlanID, callback.InstanceCode, callback.HookEvent, callback.Result, callback.FailedReason)

	hookSetting, err := mongodb.NewSystemSettingColl().GetReleasePlanHookSetting()
	if err != nil {
		fmtErr := fmt.Errorf("failed get release plan hook setting, err: %v", err)
		log.Error(fmtErr)
		return fmtErr
	}

	if !hookSetting.Enable || !hookSetting.EnableCallBack {
		return nil
	}

	hookEventStatusMap := map[config.ReleasePlanStatus]bool{}
	for _, event := range hookSetting.HookEvents {
		if event != callback.HookEvent {
			continue
		}

		switch event {
		case models.ReleasePlanHookEventFinishPlanning:
			hookEventStatusMap[config.ReleasePlanStatusWaitForFinishPlanningExternalCheck] = true
		// case models.ReleasePlanHookEventSubmitApproval:
		// 	hookEventStatusMap[config.ReleasePlanStatusWaitForApproveExternalCheck] = true
		case models.ReleasePlanHookEventStartExecute:
			hookEventStatusMap[config.ReleasePlanStatusWaitForExecuteExternalCheck] = true
		case models.ReleasePlanHookEventAllJobDone:
			hookEventStatusMap[config.ReleasePlanStatusWaitForAllDoneExternalCheck] = true
		}
	}

	releasePlan, err := mongodb.NewReleasePlanColl().GetByID(c, callback.ReleasePlanID)
	if err != nil {
		fmtErr := fmt.Errorf("failed get release plan, id: %s, err: %v", callback.ReleasePlanID, err)
		log.Error(fmtErr)
		return fmtErr
	}

	if releasePlan.InstanceCode != callback.InstanceCode {
		fmtErr := fmt.Errorf("release plan instance code is not correct, name: %s, instance code: %s, expected: %s", releasePlan.Name, callback.InstanceCode, releasePlan.InstanceCode)
		log.Error(fmtErr)
		return fmtErr
	}

	if !hookEventStatusMap[releasePlan.Status] {
		fmtErr := fmt.Errorf("release plan's status is not correct, status: %s", releasePlan.Status)
		log.Error(fmtErr)
		return fmtErr
	}

	if callback.Result == setting.ReleasePlanCallBackResultTypeSuccess {
		// source status process
		switch releasePlan.Status {
		case config.ReleasePlanStatusWaitForFinishPlanningExternalCheck:
			releasePlan.WaitForFinishPlanningExternalCheckTime = time.Now().Unix()
		// case config.ReleasePlanStatusWaitForApproveExternalCheck:
		// 	releasePlan.WaitForApproveExternalCheckTime = time.Now().Unix()
		case config.ReleasePlanStatusWaitForExecuteExternalCheck:
			releasePlan.WaitForExecuteExternalCheckTime = time.Now().Unix()
		case config.ReleasePlanStatusWaitForAllDoneExternalCheck:
			releasePlan.WaitForAllDoneExternalCheckTime = time.Now().Unix()
		}

		nextStatus, ok := config.ReleasePlanExternalCheckNextStatusMap[releasePlan.Status]
		if !ok {
			fmtErr := fmt.Errorf("release plan's status is not correct, cannot get next status, status: %s", releasePlan.Status)
			log.Error(fmtErr)
			return fmtErr
		}
		releasePlan.Status = nextStatus

		// target status process
		if releasePlan.Status == config.ReleasePlanStatusFinishPlanning {
			releasePlan.FinishPlanningTime = time.Now().Unix()
			if releasePlan.Approval == nil || releasePlan.Approval.Enabled == false {
				releasePlan.Status = config.ReleasePlanStatusExecuting
				releasePlan.ExecutingTime = time.Now().Unix()
				setReleaseJobsForExecuting(releasePlan)
			}

			// if releasePlan.Status == config.ReleasePlanStatusWaitForApprove {
			// userInfo, err := user.New().GetUserByID(releasePlan.ApproverID)
			// if err != nil {
			// 	fmtErr := fmt.Errorf("failed get user, id: %s, err: %v", releasePlan.ApproverID, err)
			// 	log.Error(fmtErr)
			// 	return fmtErr
			// }

			// if err := clearApprovalData(releasePlan.Approval); err != nil {
			// 	fmtErr := fmt.Errorf("failed clear approval data, err: %v", err)
			// 	log.Error(fmtErr)
			// 	return fmtErr
			// }
			// if err := createApprovalInstance(releasePlan, userInfo.Phone); err != nil {
			// 	fmtErr := fmt.Errorf("failed create approval instance, err: %v", err)
			// 	log.Error(fmtErr)
			// 	return fmtErr
			// }
		} else if releasePlan.Status == config.ReleasePlanStatusExecuting {
			if releasePlan.Approval != nil && releasePlan.Approval.Enabled == true && releasePlan.Approval.Status != config.StatusPassed {
				fmtErr := fmt.Errorf("approval status is %s, can not execute", releasePlan.Approval.Status)
				log.Error(fmtErr)
				return fmtErr
			}

			releasePlan.ExecutingTime = time.Now().Unix()
			setReleaseJobsForExecuting(releasePlan)
		} else if releasePlan.Status == config.ReleasePlanStatusSuccess {
			releasePlan.SuccessTime = time.Now().Unix()
		}

		if err := mongodb.NewReleasePlanColl().UpdateByID(c, releasePlan.ID.Hex(), releasePlan); err != nil {
			fmtErr := fmt.Errorf("failed update release plan, id: %s, err: %v", releasePlan.ID.Hex(), err)
			log.Error(fmtErr)
			return fmtErr
		}
	} else if callback.Result == setting.ReleasePlanCallBackResultTypeFailed {
		switch releasePlan.Status {
		case config.ReleasePlanStatusWaitForFinishPlanningExternalCheck:
			releasePlan.Status = config.ReleasePlanStatusWaitForFinishPlanningExternalCheckFailed
			releasePlan.WaitForFinishPlanningExternalCheckTime = time.Now().Unix()
		// case config.ReleasePlanStatusWaitForApproveExternalCheck:
		// 	releasePlan.Status = config.ReleasePlanStatusWaitForApproveExternalCheckFailed
		// 	releasePlan.WaitForApproveExternalCheckTime = time.Now().Unix()
		case config.ReleasePlanStatusWaitForExecuteExternalCheck:
			releasePlan.Status = config.ReleasePlanStatusWaitForExecuteExternalCheckFailed
			releasePlan.WaitForExecuteExternalCheckTime = time.Now().Unix()
		case config.ReleasePlanStatusWaitForAllDoneExternalCheck:
			releasePlan.Status = config.ReleasePlanStatusWaitForAllDoneExternalCheckFailed
			releasePlan.WaitForAllDoneExternalCheckTime = time.Now().Unix()
		}
		releasePlan.ExternalCheckFailedReason = callback.FailedReason

		if err := mongodb.NewReleasePlanColl().UpdateByID(c, releasePlan.ID.Hex(), releasePlan); err != nil {
			fmtErr := fmt.Errorf("failed update release plan, id: %s, err: %v", releasePlan.ID.Hex(), err)
			log.Error(fmtErr)
			return fmtErr
		}
	} else {
		err = fmt.Errorf("release plan callback result is not correct, result: %s", callback.Result)
		log.Error(err)
		return err
	}

	if err := upsertReleasePlanCron(releasePlan.ID.Hex(), releasePlan.Name, releasePlan.Index, releasePlan.Status, releasePlan.ScheduleExecuteTime); err != nil {
		return errors.Wrap(err, "upsert release plan cron")
	}

	return nil
}

func sendReleasePlanHook(plan *models.ReleasePlan, systemHookSetting *commonmodels.ReleasePlanHookSettings) error {
	hookSetting := plan.HookSettings
	if plan.HookSettings == nil {
		return nil
	}

	if !hookSetting.Enable {
		return nil
	}

	type EventStatus struct {
		Event  commonmodels.ReleasePlanHookEvent
		Status bool
	}

	hookEventStatusMap := map[config.ReleasePlanStatus]*EventStatus{}
	for _, event := range hookSetting.HookEvents {
		switch event {
		case models.ReleasePlanHookEventFinishPlanning:
			if hookSetting.EnableCallBack {
				hookEventStatusMap[config.ReleasePlanStatusWaitForFinishPlanningExternalCheck] = &EventStatus{
					Event:  models.ReleasePlanHookEventFinishPlanning,
					Status: true,
				}
			} else {
				hookEventStatusMap[config.ReleasePlanStatusFinishPlanning] = &EventStatus{
					Event:  models.ReleasePlanHookEventFinishPlanning,
					Status: true,
				}
			}
		// case models.ReleasePlanHookEventSubmitApproval:
		// 	if hookSetting.EnableCallBack {
		// 		hookEventStatusMap[config.ReleasePlanStatusWaitForApproveExternalCheck] = &EventStatus{
		// 			Event:  models.ReleasePlanHookEventSubmitApproval,
		// 			Status: true,
		// 		}
		// 	} else {
		// 		hookEventStatusMap[config.ReleasePlanStatusWaitForApprove] = &EventStatus{
		// 			Event:  models.ReleasePlanHookEventSubmitApproval,
		// 			Status: true,
		// 		}
		// 	}
		case models.ReleasePlanHookEventStartExecute:
			if hookSetting.EnableCallBack {
				hookEventStatusMap[config.ReleasePlanStatusWaitForExecuteExternalCheck] = &EventStatus{
					Event:  models.ReleasePlanHookEventStartExecute,
					Status: true,
				}
			} else {
				hookEventStatusMap[config.ReleasePlanStatusExecuting] = &EventStatus{
					Event:  models.ReleasePlanHookEventStartExecute,
					Status: true,
				}
			}
		case models.ReleasePlanHookEventAllJobDone:
			if hookSetting.EnableCallBack {
				hookEventStatusMap[config.ReleasePlanStatusWaitForAllDoneExternalCheck] = &EventStatus{
					Event:  models.ReleasePlanHookEventAllJobDone,
					Status: true,
				}
			} else {
				hookEventStatusMap[config.ReleasePlanStatusSuccess] = &EventStatus{
					Event:  models.ReleasePlanHookEventAllJobDone,
					Status: true,
				}
			}
		}
	}

	if hookEventStatusMap[plan.Status] != nil && hookEventStatusMap[plan.Status].Status {
		hookBody, err := convertReleasePlanToHookBody(plan, hookEventStatusMap[plan.Status].Event)
		if err != nil {
			log.Errorf("failed convert release plan to hook body, plan: %+v, err: %v", plan, err)
			return err
		}

		err = webhooknotify.NewClient(systemHookSetting.HookAddress, systemHookSetting.HookSecret).SendReleasePlanWebhook(hookBody)
		if err != nil {
			err = errors.Wrap(err, "send release plan hook")
			log.Error(err)
			return err
		}
	}

	return nil
}

func convertReleasePlanToHookBody(plan *models.ReleasePlan, hookEvent commonmodels.ReleasePlanHookEvent) (*webhooknotify.ReleasePlanHookBody, error) {
	hookBody := &webhooknotify.ReleasePlanHookBody{
		ID:                  plan.ID,
		Index:               plan.Index,
		EventName:           hookEvent,
		Name:                plan.Name,
		Manager:             plan.Manager,
		ManagerID:           plan.ManagerID,
		InstanceCode:        plan.InstanceCode,
		StartTime:           plan.StartTime,
		EndTime:             plan.EndTime,
		ScheduleExecuteTime: plan.ScheduleExecuteTime,
		Description:         plan.Description,
		CreatedBy:           plan.CreatedBy,
		CreateTime:          plan.CreateTime,
		UpdatedBy:           plan.UpdatedBy,
		UpdateTime:          plan.UpdateTime,
		Status:              plan.Status,
		PlanningTime:        plan.PlanningTime,
		ApprovalTime:        plan.ApprovalTime,
		ExecutingTime:       plan.ExecutingTime,
		SuccessTime:         plan.SuccessTime,
	}

	jobs := []*webhooknotify.ReleasePlanHookJob{}
	for _, job := range plan.Jobs {
		hookJob := &webhooknotify.ReleasePlanHookJob{
			ID:   job.ID,
			Name: job.Name,
			Type: job.Type,
			ReleasePlanHookJobRuntime: webhooknotify.ReleasePlanHookJobRuntime{
				Status:       job.Status,
				ExecutedBy:   job.ExecutedBy,
				ExecutedTime: job.ExecutedTime,
			},
		}

		if job.Type == config.JobText {
			spec := new(models.TextReleaseJobSpec)
			err := models.IToi(job.Spec, spec)
			if err != nil {
				fmtErr := fmt.Errorf("failed convert job spec to text release job spec, job: %+v, err: %v", job, err)
				log.Error(fmtErr)
				return nil, fmtErr
			}

			hookJob.Spec = &webhooknotify.ReleasePlanHookTextJobSpec{
				Content: spec.Content,
				Remark:  spec.Remark,
			}
		} else if job.Type == config.JobWorkflow {
			spec := new(models.WorkflowReleaseJobSpec)
			err := models.IToi(job.Spec, spec)
			if err != nil {
				fmtErr := fmt.Errorf("failed convert job spec to workflow release job spec, job: %+v, err: %v", job, err)
				log.Error(fmtErr)
				return nil, fmtErr
			}

			hookWorkflow, err := convertWorkflowV4ToOpenAPIWorkflowV4(spec.Workflow)
			if err != nil {
				fmtErr := fmt.Errorf("failed convert workflow to openapi workflow, job: %+v, err: %v", job, err)
				log.Error(fmtErr)
				return nil, fmtErr
			}

			hookJob.Spec = &webhooknotify.ReleasePlanHookWorkflowJobSpec{
				Workflow: hookWorkflow,
				Status:   spec.Status,
				TaskID:   spec.TaskID,
			}
		} else {
			fmtErr := fmt.Errorf("job type is not text or workflow, job: %+v", job)
			log.Error(fmtErr)
			return nil, fmtErr
		}

		jobs = append(jobs, hookJob)
	}

	hookBody.Jobs = jobs

	return hookBody, nil
}

func convertWorkflowV4ToOpenAPIWorkflowV4(workflow *commonmodels.WorkflowV4) (*webhooknotify.OpenAPIWorkflowV4, error) {
	params := []*webhooknotify.OpenAPIWorkflowParam{}
	for _, param := range workflow.Params {
		hookParam := &webhooknotify.OpenAPIWorkflowParam{
			Name:         param.Name,
			Description:  param.Description,
			ParamsType:   param.ParamsType,
			Value:        param.Value,
			ChoiceOption: param.ChoiceOption,
			ChoiceValue:  param.ChoiceValue,
			Default:      param.Default,
			IsCredential: param.IsCredential,
			Source:       param.Source,
			Repo:         convertRepoToOpenAPIWorkflowRepository(param.Repo),
		}

		params = append(params, hookParam)
	}

	hookStages := []*webhooknotify.OpenAPIWorkflowStage{}
	for _, stage := range workflow.Stages {
		hookStage := &webhooknotify.OpenAPIWorkflowStage{
			Name: stage.Name,
		}

		hookSpec := interface{}(nil)
		for _, job := range stage.Jobs {
			if job.Skipped {
				continue
			}

			switch job.JobType {
			case config.JobZadigBuild:
				spec := new(commonmodels.ZadigBuildJobSpec)
				err := models.IToi(job.Spec, spec)
				if err != nil {
					fmtErr := fmt.Errorf("failed convert job spec to zadig build job spec, job: %+v, err: %v", job, err)
					log.Error(fmtErr)
					return nil, fmtErr
				}

				serviceAndBuilds := []*webhooknotify.OpenAPIWorkflowServiceAndBuild{}
				for _, serviceAndBuild := range spec.ServiceAndBuilds {
					serviceAndBuilds = append(serviceAndBuilds, &webhooknotify.OpenAPIWorkflowServiceAndBuild{
						ServiceName:   serviceAndBuild.ServiceName,
						ServiceModule: serviceAndBuild.ServiceModule,
						BuildName:     serviceAndBuild.BuildName,
						Image:         serviceAndBuild.Image,
						Package:       serviceAndBuild.Package,
						ImageName:     serviceAndBuild.ImageName,
						KeyVals:       serviceAndBuild.KeyVals,
						Repos:         convertReposToOpenAPIWorkflowRepository(serviceAndBuild.Repos),
					})
				}

				hookSpec = &webhooknotify.OpenAPIWorkflowBuildJobSpec{
					Source:           spec.Source,
					JobName:          spec.JobName,
					RefRepos:         spec.RefRepos,
					ServiceAndBuilds: serviceAndBuilds,
				}
			case config.JobZadigDeploy:
				spec := new(commonmodels.ZadigDeployJobSpec)
				err := models.IToi(job.Spec, spec)
				if err != nil {
					fmtErr := fmt.Errorf("failed convert job spec to zadig deploy job spec, job: %+v, err: %v", job, err)
					log.Error(fmtErr)
					return nil, fmtErr
				}

				services := []*webhooknotify.OpenAPIWorkflowDeployServiceInfo{}
				for _, service := range spec.Services {
					modules := []*webhooknotify.OpenAPIWorkflowDeployModuleInfo{}
					for _, module := range service.Modules {
						modules = append(modules, &webhooknotify.OpenAPIWorkflowDeployModuleInfo{
							ServiceModule: module.ServiceModule,
							Image:         module.Image,
							ImageName:     module.ImageName,
						})
					}

					services = append(services, &webhooknotify.OpenAPIWorkflowDeployServiceInfo{
						OpenAPIWorkflowDeployBasicInfo: webhooknotify.OpenAPIWorkflowDeployBasicInfo{
							ServiceName:  service.ServiceName,
							Modules:      modules,
							Deployed:     service.Deployed,
							AutoSync:     service.AutoSync,
							UpdateConfig: service.UpdateConfig,
							Updatable:    service.Updatable,
						},
						OpenAPIWorkflowDeployVariableInfo: webhooknotify.OpenAPIWorkflowDeployVariableInfo{
							ValueMergeStrategy: service.ValueMergeStrategy,
							VariableKVs:        service.VariableKVs,
							OverrideKVs:        service.OverrideKVs,
							VariableYaml:       service.VariableYaml,
						},
					})
				}

				hookSpec = &webhooknotify.OpenAPIWorkflowDeployJobSpec{
					Source:         spec.Source,
					JobName:        spec.JobName,
					Env:            spec.Env,
					EnvSource:      spec.EnvSource,
					Production:     spec.Production,
					DeployType:     spec.DeployType,
					DeployContents: spec.DeployContents,
					VersionName:    spec.VersionName,
					Services:       services,
				}
			case config.JobZadigVMDeploy:
				spec := new(commonmodels.ZadigVMDeployJobSpec)
				err := models.IToi(job.Spec, spec)
				if err != nil {
					fmtErr := fmt.Errorf("failed convert job spec to zadig vm deploy job spec, job: %+v, err: %v", job, err)
					log.Error(fmtErr)
					return nil, fmtErr
				}

				serviceAndVMDeploys := []*webhooknotify.OpenAPIWorkflowServiceAndVMDeploy{}
				for _, serviceAndVMDeploy := range spec.ServiceAndVMDeploys {
					serviceAndVMDeploy := &webhooknotify.OpenAPIWorkflowServiceAndVMDeploy{
						ServiceName:        serviceAndVMDeploy.ServiceName,
						ServiceModule:      serviceAndVMDeploy.ServiceModule,
						DeployName:         serviceAndVMDeploy.DeployName,
						DeployArtifactType: serviceAndVMDeploy.DeployArtifactType,
						ArtifactURL:        serviceAndVMDeploy.ArtifactURL,
						FileName:           serviceAndVMDeploy.FileName,
						Image:              serviceAndVMDeploy.Image,
						KeyVals:            serviceAndVMDeploy.KeyVals,
						Repos:              convertReposToOpenAPIWorkflowRepository(serviceAndVMDeploy.Repos),
					}

					serviceAndVMDeploys = append(serviceAndVMDeploys, serviceAndVMDeploy)
				}

				hookSpec = &webhooknotify.OpenAPIWorkflowVMDeployJobSpec{
					Source:              spec.Source,
					JobName:             spec.JobName,
					Env:                 spec.Env,
					EnvSource:           spec.EnvSource,
					Production:          spec.Production,
					EnvAlias:            spec.EnvAlias,
					RefRepos:            spec.RefRepos,
					ServiceAndVMDeploys: serviceAndVMDeploys,
				}
			case config.JobFreestyle:
				spec := new(commonmodels.FreestyleJobSpec)
				err := models.IToi(job.Spec, spec)
				if err != nil {
					fmtErr := fmt.Errorf("failed convert job spec to freestyle job spec, job: %+v, err: %v", job, err)
					log.Error(fmtErr)
					return nil, fmtErr
				}

				services := []*webhooknotify.OpenAPIWorkflowFreeStyleServiceInfo{}
				for _, service := range spec.Services {
					services = append(services, &webhooknotify.OpenAPIWorkflowFreeStyleServiceInfo{
						OpenAPIWorkflowServiceWithModule: webhooknotify.OpenAPIWorkflowServiceWithModule{
							ServiceName:   service.ServiceName,
							ServiceModule: service.ServiceModule,
						},
						Repos:   convertReposToOpenAPIWorkflowRepository(service.Repos),
						KeyVals: service.KeyVals,
					})
				}

				hookSpec = &webhooknotify.OpenAPIWorkflowFreestyleJobSpec{
					FreestyleJobType: spec.FreestyleJobType,
					ServiceSource:    spec.ServiceSource,
					JobName:          spec.JobName,
					RefRepos:         spec.RefRepos,
					Repos:            convertReposToOpenAPIWorkflowRepository(spec.Repos),
					Services:         services,
					Envs:             spec.Envs,
				}
			case config.JobZadigTesting:
				spec := new(commonmodels.ZadigTestingJobSpec)
				err := models.IToi(job.Spec, spec)
				if err != nil {
					fmtErr := fmt.Errorf("failed convert job spec to testing job spec, job: %+v, err: %v", job, err)
					log.Error(fmtErr)
					return nil, fmtErr
				}

				testModules := []*webhooknotify.OpenAPIWorkflowTestModule{}
				for _, testModule := range spec.TestModules {
					testModules = append(testModules, &webhooknotify.OpenAPIWorkflowTestModule{
						Name:    testModule.Name,
						KeyVals: testModule.KeyVals,
						Repos:   convertReposToOpenAPIWorkflowRepository(testModule.Repos),
					})
				}

				serviceAndTests := []*webhooknotify.OpenAPIWorkflowServiceAndTest{}
				for _, serviceAndTest := range spec.ServiceAndTests {
					serviceAndTests = append(serviceAndTests, &webhooknotify.OpenAPIWorkflowServiceAndTest{
						ServiceName:   serviceAndTest.ServiceName,
						ServiceModule: serviceAndTest.ServiceModule,
						OpenAPIWorkflowTestModule: &webhooknotify.OpenAPIWorkflowTestModule{
							Name:    serviceAndTest.Name,
							KeyVals: serviceAndTest.KeyVals,
							Repos:   convertReposToOpenAPIWorkflowRepository(serviceAndTest.Repos),
						},
					})
				}

				hookSpec = &webhooknotify.OpenAPIWorkflowTestingJobSpec{
					TestType:        spec.TestType,
					Source:          spec.Source,
					JobName:         spec.JobName,
					RefRepos:        spec.RefRepos,
					TestModules:     testModules,
					ServiceAndTests: serviceAndTests,
				}
			case config.JobZadigScanning:
				spec := new(commonmodels.ZadigScanningJobSpec)
				err := models.IToi(job.Spec, spec)
				if err != nil {
					fmtErr := fmt.Errorf("failed convert job spec to scanning job spec, job: %+v, err: %v", job, err)
					log.Error(fmtErr)
					return nil, fmtErr
				}

				scannings := []*webhooknotify.OpenAPIWorkflowScanningModule{}
				for _, scanning := range spec.Scannings {
					scannings = append(scannings, &webhooknotify.OpenAPIWorkflowScanningModule{
						Name:    scanning.Name,
						Repos:   convertReposToOpenAPIWorkflowRepository(scanning.Repos),
						KeyVals: scanning.KeyVals,
					})
				}

				serviceAndScannings := []*webhooknotify.OpenAPIWorkflowServiceAndScannings{}
				for _, serviceAndScanning := range spec.ServiceAndScannings {
					serviceAndScannings = append(serviceAndScannings, &webhooknotify.OpenAPIWorkflowServiceAndScannings{
						ServiceName:   serviceAndScanning.ServiceName,
						ServiceModule: serviceAndScanning.ServiceModule,
						OpenAPIWorkflowScanningModule: &webhooknotify.OpenAPIWorkflowScanningModule{
							Name:    serviceAndScanning.Name,
							Repos:   convertReposToOpenAPIWorkflowRepository(serviceAndScanning.Repos),
							KeyVals: serviceAndScanning.KeyVals,
						},
					})
				}

				hookSpec = &webhooknotify.OpenAPIWorkflowScanningJobSpec{
					ScanningType:        spec.ScanningType,
					Source:              spec.Source,
					JobName:             spec.JobName,
					RefRepos:            spec.RefRepos,
					Scannings:           scannings,
					ServiceAndScannings: serviceAndScannings,
				}
			case config.JobSQL:
				spec := new(commonmodels.SQLJobSpec)
				err := models.IToi(job.Spec, spec)
				if err != nil {
					fmtErr := fmt.Errorf("failed convert job spec to sql job spec, job: %+v, err: %v", job, err)
					log.Error(fmtErr)
					return nil, fmtErr
				}

				hookSpec = &webhooknotify.OpenAPIWorkflowSQLJobSpec{
					ID:     spec.ID,
					Type:   spec.Type,
					SQL:    spec.SQL,
					Source: spec.Source,
				}
			case config.JobApollo:
				spec := new(commonmodels.ApolloJobSpec)
				err := models.IToi(job.Spec, spec)
				if err != nil {
					fmtErr := fmt.Errorf("failed convert job spec to apollo job spec, job: %+v, err: %v", job, err)
					log.Error(fmtErr)
					return nil, fmtErr
				}

				namespaceList := []*webhooknotify.OpenAPIWorkflowApolloNamespace{}
				for _, namespace := range spec.NamespaceList {
					keyValList := []*webhooknotify.OpenAPIWorkflowApolloKV{}
					for _, kv := range namespace.KeyValList {
						keyValList = append(keyValList, &webhooknotify.OpenAPIWorkflowApolloKV{
							Key: kv.Key,
							Val: kv.Val,
						})
					}

					originalConfig := []*webhooknotify.OpenAPIWorkflowApolloKV{}
					for _, kv := range namespace.OriginalConfig {
						originalConfig = append(originalConfig, &webhooknotify.OpenAPIWorkflowApolloKV{
							Key: kv.Key,
							Val: kv.Val,
						})
					}

					namespaceList = append(namespaceList, &webhooknotify.OpenAPIWorkflowApolloNamespace{
						AppID:          namespace.AppID,
						ClusterID:      namespace.ClusterID,
						Env:            namespace.Env,
						Namespace:      namespace.Namespace,
						Type:           namespace.Type,
						OriginalConfig: originalConfig,
						KeyValList:     keyValList,
					})
				}

				hookSpec = &webhooknotify.OpenAPIWorkflowApolloJobSpec{
					ApolloID:      spec.ApolloID,
					NamespaceList: namespaceList,
				}
			case config.JobNacos:
				spec := new(commonmodels.NacosJobSpec)
				err := models.IToi(job.Spec, spec)
				if err != nil {
					fmtErr := fmt.Errorf("failed convert job spec to nacos job spec, job: %+v, err: %v", job, err)
					log.Error(fmtErr)
					return nil, fmtErr
				}

				hookSpec = &webhooknotify.OpenAPIWorkflowNacosJobSpec{
					NacosID:     spec.NacosID,
					NamespaceID: spec.NamespaceID,
					Source:      spec.Source,
					NacosDatas:  spec.NacosDatas,
				}
			case config.JobZadigDistributeImage:
				spec := new(commonmodels.ZadigDistributeImageJobSpec)
				err := models.IToi(job.Spec, spec)
				if err != nil {
					fmtErr := fmt.Errorf("failed convert job spec to distribute image job spec, job: %+v, err: %v", job, err)
					log.Error(fmtErr)
					return nil, fmtErr
				}

				targets := []*webhooknotify.OpenAPIWorkflowDistributeTarget{}
				for _, target := range spec.Targets {
					targets = append(targets, &webhooknotify.OpenAPIWorkflowDistributeTarget{
						ServiceName:   target.ServiceName,
						ServiceModule: target.ServiceModule,
						SourceTag:     target.SourceTag,
						TargetTag:     target.TargetTag,
						ImageName:     target.ImageName,
						SourceImage:   target.SourceImage,
						TargetImage:   target.TargetImage,
						UpdateTag:     target.UpdateTag,
					})
				}

				hookSpec = &webhooknotify.OpenAPIWorkflowDistributeImageJobSpec{
					Source:                   spec.Source,
					JobName:                  spec.JobName,
					DistributeMethod:         spec.DistributeMethod,
					Targets:                  targets,
					EnableTargetImageTagRule: spec.EnableTargetImageTagRule,
					TargetImageTagRule:       spec.TargetImageTagRule,
				}
			}

			hookStage.Jobs = append(hookStage.Jobs, &webhooknotify.OpenAPIWorkflowJob{
				Name:      job.Name,
				JobType:   job.JobType,
				Spec:      hookSpec,
				RunPolicy: job.RunPolicy,
			})
		}

		if len(hookStage.Jobs) != 0 {
			hookStages = append(hookStages, hookStage)
		}
	}

	return &webhooknotify.OpenAPIWorkflowV4{
		Name:                 workflow.Name,
		DisplayName:          workflow.DisplayName,
		Disabled:             workflow.Disabled,
		Params:               params,
		Stages:               hookStages,
		Project:              workflow.Project,
		Description:          workflow.Description,
		CreatedBy:            workflow.CreatedBy,
		CreateTime:           workflow.CreateTime,
		UpdatedBy:            workflow.UpdatedBy,
		UpdateTime:           workflow.UpdateTime,
		Remark:               workflow.Remark,
		EnableApprovalTicket: workflow.EnableApprovalTicket,
		ApprovalTicketID:     workflow.ApprovalTicketID,
	}, nil
}

func convertReposToOpenAPIWorkflowRepository(repos []*types.Repository) []*webhooknotify.OpenAPIWorkflowRepository {
	hookRepos := []*webhooknotify.OpenAPIWorkflowRepository{}
	for _, repo := range repos {
		hookRepos = append(hookRepos, convertRepoToOpenAPIWorkflowRepository(repo))
	}

	return hookRepos
}

func convertRepoToOpenAPIWorkflowRepository(repo *types.Repository) *webhooknotify.OpenAPIWorkflowRepository {
	if repo == nil {
		return nil
	}

	hookRepo := &webhooknotify.OpenAPIWorkflowRepository{
		Source:        repo.Source,
		RepoOwner:     repo.RepoOwner,
		RepoNamespace: repo.RepoNamespace,
		RepoName:      repo.RepoName,
		RemoteName:    repo.RemoteName,
		Branch:        repo.Branch,
		PRs:           repo.PRs,
		Tag:           repo.Tag,
		CommitID:      repo.CommitID,
		CommitMessage: repo.CommitMessage,
		CheckoutPath:  repo.CheckoutPath,
		CodehostID:    repo.CodehostID,
		Address:       repo.Address,
	}

	return hookRepo
}

func waitForExternalCheck(plan *models.ReleasePlan, systemHookSetting *commonmodels.ReleasePlanHookSettings) (*config.ReleasePlanStatus, bool) {
	if plan.HookSettings == nil {
		return nil, false
	}

	hookSetting := plan.HookSettings
	shouldWait := false
	if !hookSetting.Enable {
		return nil, shouldWait
	}

	nextStatus := plan.Status
	hookEventStatusMap := map[config.ReleasePlanStatus]bool{}
	for _, event := range hookSetting.HookEvents {
		switch event {
		case models.ReleasePlanHookEventFinishPlanning:
			hookEventStatusMap[config.ReleasePlanStatusFinishPlanning] = true

			if plan.Status == config.ReleasePlanStatusFinishPlanning {
				nextStatus = config.ReleasePlanStatusWaitForFinishPlanningExternalCheck
			}
		// case models.ReleasePlanHookEventSubmitApproval:
		// 	hookEventStatusMap[config.ReleasePlanStatusWaitForApprove] = true

		// 	if plan.Status == config.ReleasePlanStatusWaitForApprove {
		// 		nextStatus = config.ReleasePlanStatusWaitForApproveExternalCheck
		// 	}
		case models.ReleasePlanHookEventStartExecute:
			hookEventStatusMap[config.ReleasePlanStatusExecuting] = true

			if plan.Status == config.ReleasePlanStatusExecuting {
				nextStatus = config.ReleasePlanStatusWaitForExecuteExternalCheck
			}
		case models.ReleasePlanHookEventAllJobDone:
			hookEventStatusMap[config.ReleasePlanStatusSuccess] = true

			if plan.Status == config.ReleasePlanStatusSuccess {
				nextStatus = config.ReleasePlanStatusWaitForAllDoneExternalCheck
			}
		default:
			log.Errorf("release plan hook event is not correct, event: %s", event)
		}
	}

	if hookEventStatusMap[plan.Status] {
		if hookSetting.EnableCallBack {
			shouldWait = true
		}
	}

	return &nextStatus, shouldWait
}

func generateInstanceCode(plan *models.ReleasePlan) (string, error) {
	newPlan := new(models.ReleasePlan)
	err := util.DeepCopy(newPlan, plan)
	if err != nil {
		err := fmt.Errorf("failed deep copy plan, err: %v", err)
		log.Error(err)
		return "", err
	}
	newPlan.UpdateTime = 0
	newPlan.UpdatedBy = ""
	newPlan.InstanceCode = ""
	newPlan.Status = ""
	newPlan.PlanningTime = 0
	newPlan.ApprovalTime = 0
	newPlan.ExecutingTime = 0
	newPlan.SuccessTime = 0
	newPlan.WaitForFinishPlanningExternalCheckTime = 0
	// newPlan.WaitForApproveExternalCheckTime = 0
	newPlan.WaitForExecuteExternalCheckTime = 0
	newPlan.WaitForAllDoneExternalCheckTime = 0
	newPlan.ExternalCheckFailedReason = ""

	newPlanBytes, err := bson.Marshal(newPlan)
	if err != nil {
		err := fmt.Errorf("failed marshal plan, err: %v", err)
		log.Error(err)
		return "", err
	}

	sum := sha256.Sum256([]byte(newPlanBytes))
	return hex.EncodeToString(sum[:])[:16], nil
}

func cancelReleasePlanApproval(ctx *handler.Context, plan *models.ReleasePlan) error {
	if plan == nil {
		log.Warnf("cancelReleasePlanApproval: release plan is nil")
		return nil
	}

	if plan.Approval == nil {
		return nil
	}

	if !plan.Approval.Enabled {
		return nil
	}

	if plan.Approval.Status == config.StatusPassed || plan.Approval.Status == config.StatusReject || plan.Approval.Status == config.StatusCancelled {
		return nil
	}

	if plan.Approval.LarkApproval != nil {
		data, err := mongodb.NewIMAppColl().GetByID(context.Background(), plan.Approval.LarkApproval.ID)
		if err != nil {
			return fmt.Errorf("get lark im app data error: %s", err)
		}

		approvalCode := data.LarkApprovalCodeListCommon[plan.Approval.LarkApproval.GetNodeTypeKey()]
		if approvalCode == "" {
			log.Warnf("failed to find approval code for node type %s", plan.Approval.LarkApproval.GetNodeTypeKey())
			return nil
		}

		initiatorUserID := ""
		if plan.Approval.LarkApproval.ApprovalInitiator != nil {
			initiatorUserID = plan.Approval.LarkApproval.ApprovalInitiator.ID
		} else if plan.Approval.LarkApproval.DefaultApprovalInitiator != nil {
			initiatorUserID = plan.Approval.LarkApproval.DefaultApprovalInitiator.ID
		} else {
			log.Warnf("cancel approval %s: initiator user id is not set", plan.Approval.LarkApproval.InstanceCode)
			return nil
		}

		client := lark.NewClient(data.AppID, data.AppSecret, data.Type)
		err = client.CancelApprovalInstance(&lark.CancelApprovalInstanceArgs{
			ApprovalID: approvalCode,
			InstanceID: plan.Approval.LarkApproval.InstanceCode,
			UserID:     initiatorUserID,
		})
		if err != nil {
			log.Errorf("cancel approval %s error: %v", plan.Approval.LarkApproval.InstanceCode, err)
		}
	}

	if plan.Approval.DingTalkApproval != nil {
		dingservice.RemoveDingTalkApprovalManager(plan.Approval.DingTalkApproval.InstanceCode)
	}

	if plan.Approval.WorkWXApproval != nil {
		workwxservice.RemoveWorkWXApprovalManager(plan.Approval.WorkWXApproval.InstanceID)
	}

	return nil
}
