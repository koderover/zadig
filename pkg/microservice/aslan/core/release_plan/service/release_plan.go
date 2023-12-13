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

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/samber/lo"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	approvalservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/approval"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/shared/client/user"
	"github.com/koderover/zadig/v2/pkg/shared/handler"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

var (
	defaultTimeout = time.Second * 3
)

func CreateReleasePlan(c *handler.Context, args *models.ReleasePlan) error {
	if args.Name == "" || args.ManagerID == "" {
		return errors.New("Required parameters are missing")
	}
	if err := lintReleaseTimeRange(args.StartTime, args.EndTime); err != nil {
		return errors.Wrap(err, "lint release time range error")
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

type ListReleasePlanResp struct {
	List  []*models.ReleasePlan `json:"list"`
	Total int64                 `json:"total"`
}

func ListReleasePlans(pageNum, pageSize int64) (*ListReleasePlanResp, error) {
	list, total, err := mongodb.NewReleasePlanColl().ListByOptions(&mongodb.ListReleasePlanOption{
		PageNum:        pageNum,
		PageSize:       pageSize,
		IsSort:         true,
		ExcludedFields: []string{"jobs", "logs"},
	})
	if err != nil {
		return nil, errors.Wrap(err, "ListReleasePlans")
	}
	return &ListReleasePlanResp{
		List:  list,
		Total: total,
	}, nil
}

func GetReleasePlan(id string) (*models.ReleasePlan, error) {
	return mongodb.NewReleasePlanColl().GetByID(context.Background(), id)
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
	getLock(planID).Lock()
	defer getLock(planID).Unlock()

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

func ExecuteReleaseJob(c *handler.Context, planID string, args *ExecuteReleaseJobArgs) error {
	getLock(planID).Lock()
	defer getLock(planID).Unlock()

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

	if plan.ManagerID != c.UserID {
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
		plan.ExecutingTime = time.Now().Unix()
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

func UpdateReleasePlanStatus(c *handler.Context, planID, status string) error {
	getLock(planID).Lock()
	defer getLock(planID).Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	plan, err := mongodb.NewReleasePlanColl().GetByID(ctx, planID)
	if err != nil {
		return errors.Wrap(err, "get plan")
	}

	if c.UserID != plan.ManagerID {
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
		if plan.Approval != nil && plan.Approval.Status != config.StatusPassed {
			detail = "跳过审批"
		}
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
		plan.ExecutingTime = time.Now().Unix()
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
	getLock(planID).Lock()
	defer getLock(planID).Unlock()

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

	approveWithL, ok := approvalservice.GlobalApproveMap.GetApproval(plan.Approval.NativeApproval.InstanceCode)
	if !ok {
		// restore data after restart aslan
		log.Infof("updateNativeApproval: approval instance code %s not found, set it", plan.Approval.NativeApproval.InstanceCode)
		approveWithL = &approvalservice.ApproveWithLock{Approval: plan.Approval.NativeApproval}
		approvalservice.GlobalApproveMap.SetApproval(plan.Approval.NativeApproval.InstanceCode, approveWithL)
	}
	if err = approveWithL.DoApproval(c.UserName, c.UserID, req.Comment, req.Approve); err != nil {
		return errors.Wrap(err, "do approval")
	}

	plan.Approval.NativeApproval = approveWithL.Approval
	approved, _, err := approveWithL.IsApproval()
	if err != nil {
		plan.Approval.Status = config.StatusReject
	}
	if approved {
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
		if job.Status != config.ReleasePlanJobStatusDone {
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
