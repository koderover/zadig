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

package workflowcontroller

import (
	"context"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	configbase "github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	dingservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/dingtalk"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/instantmessage"
	larkservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/lark"
	"github.com/koderover/zadig/pkg/tool/dingtalk"
	"github.com/koderover/zadig/pkg/tool/lark"
	"github.com/koderover/zadig/pkg/tool/log"
)

type approveMap struct {
	m map[string]*approveWithLock
	sync.RWMutex
}

type approveWithLock struct {
	approval *commonmodels.NativeApproval
	sync.RWMutex
}

var globalApproveMap approveMap

func init() {
	globalApproveMap.m = make(map[string]*approveWithLock, 0)
}

type StageCtl interface {
	Run(ctx context.Context, concurrency int)
}

func runStage(ctx context.Context, stage *commonmodels.StageTask, workflowCtx *commonmodels.WorkflowTaskCtx, concurrency int, logger *zap.SugaredLogger, ack func()) {
	stage.Status = config.StatusRunning
	ack()
	logger.Infof("start stage: %s,status: %s", stage.Name, stage.Status)
	if err := waitForApprove(ctx, stage, workflowCtx, logger, ack); err != nil {
		stage.Error = err.Error()
		logger.Errorf("finish stage: %s,status: %s", stage.Name, stage.Status)
		ack()
		return
	}
	defer func() {
		updateStageStatus(stage)
		stage.EndTime = time.Now().Unix()
		logger.Infof("finish stage: %s,status: %s", stage.Name, stage.Status)
		ack()
	}()
	stage.StartTime = time.Now().Unix()
	ack()
	stageCtl := NewCustomStageCtl(stage, workflowCtx, logger, ack)

	stageCtl.Run(ctx, concurrency)
}

func RunStages(ctx context.Context, stages []*commonmodels.StageTask, workflowCtx *commonmodels.WorkflowTaskCtx, concurrency int, logger *zap.SugaredLogger, ack func()) {
	for _, stage := range stages {
		// should skip passed stage when workflow task be restarted
		if stage.Status == config.StatusPassed {
			continue
		}
		runStage(ctx, stage, workflowCtx, concurrency, logger, ack)
		if statusFailed(stage.Status) {
			return
		}
	}
}

func ApproveStage(workflowName, stageName, userName, userID, comment string, taskID int64, approve bool) error {
	approveKey := fmt.Sprintf("%s-%d-%s", workflowName, taskID, stageName)
	approveWithL, ok := globalApproveMap.getApproval(approveKey)
	if !ok {
		return fmt.Errorf("workflow %s ID %d stage %s do not need approve", workflowName, taskID, stageName)
	}
	return approveWithL.doApproval(userName, userID, comment, approve)
}

func waitForApprove(ctx context.Context, stage *commonmodels.StageTask, workflowCtx *commonmodels.WorkflowTaskCtx, logger *zap.SugaredLogger, ack func()) (err error) {
	if stage.Approval == nil {
		return nil
	}
	if !stage.Approval.Enabled {
		return nil
	}
	// should skip passed approval when workflow task be restarted
	if stage.Approval.Status == config.StatusPassed {
		return nil
	}
	stage.Approval.StartTime = time.Now().Unix()
	defer func() {
		stage.Approval.EndTime = time.Now().Unix()

		if err == nil {
			stage.Status = config.StatusRunning
			stage.Approval.Status = config.StatusPassed
		} else {
			stage.Approval.Status = stage.Status
		}
	}()
	// workflowCtx.SetStatus contain ack() function, so we don't need to call ack() here
	stage.Status = config.StatusWaitingApprove
	workflowCtx.SetStatus(config.StatusWaitingApprove)
	// if approval result is not passed, workflow status will be set correctly in outer function
	defer workflowCtx.SetStatus(config.StatusRunning)

	switch stage.Approval.Type {
	case config.NativeApproval:
		err = waitForNativeApprove(ctx, stage, workflowCtx, logger, ack)
	case config.LarkApproval:
		err = waitForLarkApprove(ctx, stage, workflowCtx, logger, ack)
	case config.DingTalkApproval:
		err = waitForDingTalkApprove(ctx, stage, workflowCtx, logger, ack)
	default:
		err = errors.New("invalid approval type")
	}
	return err
}

func waitForNativeApprove(ctx context.Context, stage *commonmodels.StageTask, workflowCtx *commonmodels.WorkflowTaskCtx, logger *zap.SugaredLogger, ack func()) error {
	approval := stage.Approval.NativeApproval
	if approval == nil {
		return errors.New("waitForApprove: native approval data not found")
	}

	if approval.Timeout == 0 {
		approval.Timeout = 60
	}
	approveKey := fmt.Sprintf("%s-%d-%s", workflowCtx.WorkflowName, workflowCtx.TaskID, stage.Name)
	approveWithL := &approveWithLock{approval: approval}
	globalApproveMap.setApproval(approveKey, approveWithL)
	defer func() {
		globalApproveMap.deleteApproval(approveKey)
		ack()
	}()
	if err := instantmessage.NewWeChatClient().SendWorkflowTaskAproveNotifications(workflowCtx.WorkflowName, workflowCtx.TaskID); err != nil {
		logger.Errorf("send approve notification failed, error: %v", err)
	}

	timeout := time.After(time.Duration(approval.Timeout) * time.Minute)
	latestApproveCount := 0
	for {
		time.Sleep(1 * time.Second)
		select {
		case <-ctx.Done():
			stage.Status = config.StatusCancelled
			return fmt.Errorf("workflow was canceled")

		case <-timeout:
			stage.Status = config.StatusTimeout
			return fmt.Errorf("workflow timeout")
		default:
			approved, approveCount, err := approveWithL.isApproval()
			if err != nil {
				stage.Status = config.StatusReject
				return err
			}
			if approved {
				return nil
			}
			if approveCount > latestApproveCount {
				ack()
				latestApproveCount = approveCount
			}
		}
	}
}

func waitForLarkApprove(ctx context.Context, stage *commonmodels.StageTask, workflowCtx *commonmodels.WorkflowTaskCtx, logger *zap.SugaredLogger, ack func()) error {
	log.Infof("waitForLarkApprove start")
	approval := stage.Approval.LarkApproval
	if approval == nil {
		stage.Status = config.StatusFailed
		return errors.New("waitForApprove: lark approval data not found")
	}
	if approval.Timeout == 0 {
		approval.Timeout = 60
	}

	data, err := mongodb.NewIMAppColl().GetByID(context.Background(), approval.ApprovalID)
	if err != nil {
		stage.Status = config.StatusFailed
		return errors.Wrap(err, "get external approval data")
	}

	client := lark.NewClient(data.AppID, data.AppSecret)

	userID, err := client.GetUserOpenIDByEmailOrMobile(lark.QueryTypeMobile, workflowCtx.WorkflowTaskCreatorMobile)
	if err != nil {
		stage.Status = config.StatusFailed
		return errors.Wrapf(err, "get user lark id by mobile-%s", workflowCtx.WorkflowTaskCreatorMobile)
	}

	log.Infof("waitForLarkApprove: ApproveUsers num %d", len(approval.ApproveUsers))
	detailURL := fmt.Sprintf("%s/v1/projects/detail/%s/pipelines/custom/%s/%d?display_name=%s",
		configbase.SystemAddress(),
		workflowCtx.ProjectName,
		workflowCtx.WorkflowName,
		workflowCtx.TaskID,
		url.QueryEscape(workflowCtx.WorkflowDisplayName),
	)
	descForm := ""
	if stage.Approval.Description != "" {
		descForm = fmt.Sprintf("\n描述: %s", stage.Approval.Description)
	}
	instance, err := client.CreateApprovalInstance(&lark.CreateApprovalInstanceArgs{
		ApprovalCode: data.LarkDefaultApprovalCode,
		UserOpenID:   userID,
		ApproverIDList: func() (list []string) {
			for _, user := range approval.ApproveUsers {
				list = append(list, user.ID)
			}
			return list
		}(),
		FormContent: fmt.Sprintf("项目名称: %s\n工作流名称: %s\n阶段名称: %s%s\n\n更多详见: %s",
			workflowCtx.ProjectName, workflowCtx.WorkflowDisplayName, stage.Name, descForm, detailURL),
	})
	if err != nil {
		log.Errorf("waitForLarkApprove: create instance failed: %v", err)
		stage.Status = config.StatusFailed
		return errors.Wrap(err, "create approval instance")
	}
	log.Infof("waitForLarkApprove: create instance success, id %s", instance)

	if err := instantmessage.NewWeChatClient().SendWorkflowTaskAproveNotifications(workflowCtx.WorkflowName, workflowCtx.TaskID); err != nil {
		logger.Errorf("send approve notification failed, error: %v", err)
	}

	cancelApproval := func() {
		err := client.CancelApprovalInstance(&lark.CancelApprovalInstanceArgs{
			ApprovalID: data.LarkDefaultApprovalCode,
			InstanceID: instance,
			UserID:     userID,
		})
		if err != nil {
			log.Errorf("cancel approval %s error: %v", instance, err)
		}
	}
	userUpdate := func(list []*commonmodels.LarkApprovalUser) []*commonmodels.LarkApprovalUser {
		info, err := client.GetApprovalInstance(&lark.GetApprovalInstanceArgs{InstanceID: instance})
		if err != nil {
			log.Errorf("waitForLarkApprove: GetApprovalInstance error: %v", err)
			return list
		}
		for _, user := range list {
			if user.ID == info.ApproverInfo.ID {
				user.RejectOrApprove = info.ApproveOrReject
				user.Comment = info.Comment
				user.OperationTime = info.Time
			}
		}
		return list
	}

	defer func() {
		larkservice.GetLarkApprovalManager(approval.ApprovalID).RemoveInstance(instance)
	}()
	timeout := time.After(time.Duration(approval.Timeout) * time.Minute)
	for {
		time.Sleep(1 * time.Second)
		select {
		case <-ctx.Done():
			stage.Status = config.StatusCancelled
			cancelApproval()
			return fmt.Errorf("workflow was canceled")
		case <-timeout:
			stage.Status = config.StatusCancelled
			cancelApproval()
			return fmt.Errorf("workflow timeout")
		default:
			status := larkservice.GetLarkApprovalManager(approval.ApprovalID).GetInstanceStatus(instance)
			switch status {
			case larkservice.ApprovalStatusApproved:
				stage.Approval.LarkApproval.ApproveUsers = userUpdate(approval.ApproveUsers)
				return nil
			case larkservice.ApprovalStatusRejected:
				stage.Status = config.StatusReject
				stage.Approval.LarkApproval.ApproveUsers = userUpdate(approval.ApproveUsers)
				return errors.New("Approval has been rejected")
			case larkservice.ApprovalStatusCanceled:
				stage.Status = config.StatusFailed
				return errors.New("Approval has been canceled")
			case larkservice.ApprovalStatusDeleted:
				return errors.New("Approval has been deleted")
			}
		}
	}
}

func waitForDingTalkApprove(ctx context.Context, stage *commonmodels.StageTask, workflowCtx *commonmodels.WorkflowTaskCtx, logger *zap.SugaredLogger, ack func()) error {
	log.Infof("waitForDingTalkApprove start")
	approval := stage.Approval.DingTalkApproval
	if approval == nil {
		stage.Status = config.StatusFailed
		return errors.New("waitForApprove: dingtalk approval data not found")
	}
	if approval.Timeout == 0 {
		approval.Timeout = 60
	}

	data, err := mongodb.NewIMAppColl().GetByID(context.Background(), approval.ApprovalID)
	if err != nil {
		stage.Status = config.StatusFailed
		return errors.Wrap(err, "get dingtalk im data")
	}

	client := dingtalk.NewClient(data.DingTalkAppKey, data.DingTalkAppSecret)

	mobile := workflowCtx.WorkflowTaskCreatorMobile
	if mobile == "" {
		if workflowCtx.DefaultApprovalInitiatorMobile == "" {
			return errors.Errorf("failed to get approval initiator phone number")
		}
		mobile = workflowCtx.DefaultApprovalInitiatorMobile
	}
	userIDResp, err := client.GetUserIDByMobile(mobile)
	if err != nil {
		stage.Status = config.StatusFailed
		return errors.Wrapf(err, "get user dingtalk id by mobile-%s", mobile)
	}
	userID := userIDResp.UserID

	log.Infof("waitForDingTalkApprove: ApproveNode num %d", len(approval.ApprovalNodes))
	detailURL := fmt.Sprintf("%s/v1/projects/detail/%s/pipelines/custom/%s/%d?display_name=%s",
		configbase.SystemAddress(),
		workflowCtx.ProjectName,
		workflowCtx.WorkflowName,
		workflowCtx.TaskID,
		url.QueryEscape(workflowCtx.WorkflowDisplayName),
	)
	descForm := ""
	if stage.Approval.Description != "" {
		descForm = fmt.Sprintf("\n描述: %s", stage.Approval.Description)
	}
	instanceResp, err := client.CreateApprovalInstance(&dingtalk.CreateApprovalInstanceArgs{
		ProcessCode:      data.DingTalkDefaultApprovalFormCode,
		OriginatorUserID: userID,
		ApproverNodeList: func() (nodeList []*dingtalk.ApprovalNode) {
			for _, node := range approval.ApprovalNodes {
				var userIDList []string
				for _, user := range node.ApproveUsers {
					userIDList = append(userIDList, user.ID)
				}
				nodeList = append(nodeList, &dingtalk.ApprovalNode{
					UserIDs:    userIDList,
					ActionType: node.Type,
				})
			}
			return
		}(),
		FormContent: fmt.Sprintf("项目名称: %s\n工作流名称: %s\n阶段名称: %s%s\n\n更多详见: %s",
			workflowCtx.ProjectName, workflowCtx.WorkflowDisplayName, stage.Name, descForm, detailURL),
	})
	if err != nil {
		log.Errorf("waitForDingTalkApprove: create instance failed: %v", err)
		stage.Status = config.StatusFailed
		return errors.Wrap(err, "create approval instance")
	}
	instanceID := instanceResp.InstanceID
	log.Infof("waitForDingTalkApprove: create instance success, id %s", instanceID)

	if err := instantmessage.NewWeChatClient().SendWorkflowTaskAproveNotifications(workflowCtx.WorkflowName, workflowCtx.TaskID); err != nil {
		logger.Errorf("send approve notification failed, error: %v", err)
	}
	defer func() {
		dingservice.RemoveDingTalkApprovalManager(instanceID)
	}()

	resultMap := map[string]config.ApproveOrReject{
		"agree":  config.Approve,
		"refuse": config.Reject,
	}

	checkNodeStatus := func(_type string, users []*commonmodels.DingTalkApprovalUser) (config.ApproveOrReject, error) {
		switch _type {
		case "AND":
			result := config.Approve
			for _, user := range users {
				if user.RejectOrApprove == "" {
					result = ""
				}
				if user.RejectOrApprove == config.Reject {
					return config.Reject, nil
				}
			}
			return result, nil
		case "OR":
			for _, user := range users {
				if user.RejectOrApprove != "" {
					return user.RejectOrApprove, nil
				}
			}
			return "", nil
		default:
			return "", errors.Errorf("unknown node type %s", _type)
		}
	}

	timeout := time.After(time.Duration(approval.Timeout) * time.Minute)
	for {
		time.Sleep(1 * time.Second)
		select {
		case <-ctx.Done():
			stage.Status = config.StatusCancelled
			return fmt.Errorf("workflow was canceled")
		case <-timeout:
			stage.Status = config.StatusCancelled
			return fmt.Errorf("workflow timeout")
		default:
			userApprovalResult := dingservice.GetDingTalkApprovalManager(instanceID).GetAllUserApprovalResults()
			for _, node := range approval.ApprovalNodes {
				if node.RejectOrApprove != "" {
					continue
				}
				for _, user := range node.ApproveUsers {
					if result := userApprovalResult[user.ID]; result != nil && user.RejectOrApprove == "" {
						user.RejectOrApprove = resultMap[result.Result]
						user.Comment = result.Remark
						user.OperationTime = result.OperationTime
					}
				}
				node.RejectOrApprove, err = checkNodeStatus(string(node.Type), node.ApproveUsers)
				if err != nil {
					stage.Status = config.StatusFailed
					log.Errorf("check node failed: %v", err)
					return errors.Wrap(err, "check node")
				}
				switch node.RejectOrApprove {
				case config.Approve:
					ack()
				case config.Reject:
					stage.Status = config.StatusReject
					return errors.New("Approval has been rejected")
				}
				break
			}
			if approval.ApprovalNodes[len(approval.ApprovalNodes)-1].RejectOrApprove == config.Approve {
				return nil
			}
		}
	}
}

func statusFailed(status config.Status) bool {
	if status == config.StatusCancelled || status == config.StatusFailed || status == config.StatusTimeout || status == config.StatusReject {
		return true
	}
	return false
}

func updateStageStatus(stage *commonmodels.StageTask) {
	statusMap := map[config.Status]int{
		config.StatusCancelled: 4,
		config.StatusTimeout:   3,
		config.StatusFailed:    2,
		config.StatusPassed:    1,
		config.StatusSkipped:   0,
	}

	// 初始化stageStatus为创建状态
	stageStatus := config.StatusRunning

	jobStatus := make([]int, len(stage.Jobs))

	for i, j := range stage.Jobs {
		statusCode, ok := statusMap[j.Status]
		if !ok {
			statusCode = -1
		}
		jobStatus[i] = statusCode
	}
	var stageStatusCode int
	for i, code := range jobStatus {
		if i == 0 || code > stageStatusCode {
			stageStatusCode = code
		}
	}

	for taskstatus, code := range statusMap {
		if stageStatusCode == code {
			stageStatus = taskstatus
			break
		}
	}
	stage.Status = stageStatus
}

func (c *approveMap) setApproval(key string, value *approveWithLock) {
	c.Lock()
	defer c.Unlock()
	c.m[key] = value
}

func (c *approveMap) getApproval(key string) (*approveWithLock, bool) {
	c.RLock()
	defer c.RUnlock()
	v, existed := c.m[key]
	return v, existed
}
func (c *approveMap) deleteApproval(key string) {
	c.Lock()
	defer c.Unlock()
	delete(c.m, key)
}

func (c *approveWithLock) isApproval() (bool, int, error) {
	c.Lock()
	defer c.Unlock()
	approveCount := 0
	for _, user := range c.approval.ApproveUsers {
		if user.RejectOrApprove == config.Reject {
			c.approval.RejectOrApprove = config.Reject
			return false, approveCount, fmt.Errorf("%s reject this task", user.UserName)
		}
		if user.RejectOrApprove == config.Approve {
			approveCount++
		}
	}
	if approveCount >= c.approval.NeededApprovers {
		c.approval.RejectOrApprove = config.Approve
		return true, approveCount, nil
	}
	return false, approveCount, nil
}

func (c *approveWithLock) doApproval(userName, userID, comment string, appvove bool) error {
	c.Lock()
	defer c.Unlock()
	for _, user := range c.approval.ApproveUsers {
		if user.UserID != userID {
			continue
		}
		if user.RejectOrApprove != "" {
			return fmt.Errorf("%s have %s already", userName, user.RejectOrApprove)
		}
		user.Comment = comment
		user.OperationTime = time.Now().Unix()
		if appvove {
			user.RejectOrApprove = config.Approve
			return nil
		} else {
			user.RejectOrApprove = config.Reject
			return nil
		}
	}
	return fmt.Errorf("user %s has no authority to approve", userName)
}
