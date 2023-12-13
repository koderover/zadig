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
	"time"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	configbase "github.com/koderover/zadig/v2/pkg/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	approvalservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/approval"
	dingservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/dingtalk"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/instantmessage"
	larkservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/lark"
	"github.com/koderover/zadig/v2/pkg/tool/dingtalk"
	"github.com/koderover/zadig/v2/pkg/tool/lark"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

type StageCtl interface {
	Run(ctx context.Context, concurrency int)
}

func runStage(ctx context.Context, stage *commonmodels.StageTask, workflowCtx *commonmodels.WorkflowTaskCtx, concurrency int, logger *zap.SugaredLogger, ack func()) {
	stage.Status = config.StatusRunning
	ack()
	logger.Infof("start stage: %s,status: %s", stage.Name, stage.Status)
	if err := waitForApprove(ctx, stage, workflowCtx, logger, ack); err != nil {
		stage.Error = err.Error()
		logger.Errorf("finish stage: %s,status: %s error: %s", stage.Name, stage.Status, stage.Error)
		ack()
		return
	}
	defer func() {
		updateStageStatus(ctx, stage)
		stage.EndTime = time.Now().Unix()
		logger.Infof("finish stage: %s,status: %s", stage.Name, stage.Status)
		ack()
	}()
	stage.StartTime = time.Now().Unix()
	ack()
	stageCtl := NewCustomStageCtl(stage, workflowCtx, logger, ack)

	stageCtl.Run(ctx, concurrency)
	stageCtl.AfterRun()
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
	approveWithL, ok := approvalservice.GlobalApproveMap.GetApproval(approveKey)
	if !ok {
		return fmt.Errorf("workflow %s ID %d stage %s do not need approve", workflowName, taskID, stageName)
	}
	return approveWithL.DoApproval(userName, userID, comment, approve)
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
	approveWithL := &approvalservice.ApproveWithLock{Approval: approval}
	approvalservice.GlobalApproveMap.SetApproval(approveKey, approveWithL)
	defer func() {
		approvalservice.GlobalApproveMap.DeleteApproval(approveKey)
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
			approved, approveCount, err := approveWithL.IsApproval()
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

	data, err := mongodb.NewIMAppColl().GetByID(context.Background(), approval.ID)
	if err != nil {
		stage.Status = config.StatusFailed
		return errors.Wrap(err, "get lark im app data")
	}
	approvalCode := data.LarkApprovalCodeList[approval.GetNodeTypeKey()]
	if approvalCode == "" {
		stage.Status = config.StatusFailed
		log.Errorf("failed to find approval code for node type %s", approval.GetNodeTypeKey())
		return errors.Errorf("failed to find approval code for node type %s", approval.GetNodeTypeKey())
	}

	client := lark.NewClient(data.AppID, data.AppSecret)

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
	formContent := fmt.Sprintf("项目名称: %s\n工作流名称: %s\n阶段名称: %s%s\n\n更多详见: %s",
		workflowCtx.ProjectName, workflowCtx.WorkflowDisplayName, stage.Name, descForm, detailURL)

	var userID string
	if approval.DefaultApprovalInitiator == nil {
		userID, err = client.GetUserOpenIDByEmailOrMobile(lark.QueryTypeMobile, workflowCtx.WorkflowTaskCreatorMobile)
		if err != nil {
			stage.Status = config.StatusFailed
			return errors.Wrapf(err, "get user lark id by mobile-%s", workflowCtx.WorkflowTaskCreatorMobile)
		}
	} else {
		userID = approval.DefaultApprovalInitiator.ID
		formContent = fmt.Sprintf("审批发起人: %s\n%s", workflowCtx.WorkflowTaskCreatorUsername, formContent)
	}
	log.Infof("waitForLarkApprove: ApproveNodes num %d", len(approval.ApprovalNodes))
	instance, err := client.CreateApprovalInstance(&lark.CreateApprovalInstanceArgs{
		ApprovalCode: approvalCode,
		UserOpenID:   userID,
		Nodes:        approval.GetLarkApprovalNode(),
		FormContent:  formContent,
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
			ApprovalID: approvalCode,
			InstanceID: instance,
			UserID:     userID,
		})
		if err != nil {
			log.Errorf("cancel approval %s error: %v", instance, err)
		}
	}

	checkNodeStatus := func(node *commonmodels.LarkApprovalNode) (config.ApproveOrReject, error) {
		switch node.Type {
		case "AND":
			result := config.Approve
			for _, user := range node.ApproveUsers {
				if user.RejectOrApprove == "" {
					result = ""
				}
				if user.RejectOrApprove == config.Reject {
					return config.Reject, nil
				}
			}
			return result, nil
		case "OR":
			for _, user := range node.ApproveUsers {
				if user.RejectOrApprove != "" {
					return user.RejectOrApprove, nil
				}
			}
			return "", nil
		default:
			return "", errors.Errorf("unknown node type %s", node.Type)
		}
	}

	// approvalUpdate is used to update the approval status
	approvalUpdate := func(larkApproval *commonmodels.LarkApproval) (done, isApprove bool, err error) {
		// userUpdated represents whether the user status has been updated
		userUpdated := false
		for i, node := range larkApproval.ApprovalNodes {
			if node.RejectOrApprove != "" {
				continue
			}
			resultMap := larkservice.GetLarkApprovalInstanceManager(instance).GetNodeUserApprovalResults(lark.ApprovalNodeIDKey(i))
			for _, user := range node.ApproveUsers {
				if result, ok := resultMap[user.ID]; ok && user.RejectOrApprove == "" {
					instanceData, err := client.GetApprovalInstance(&lark.GetApprovalInstanceArgs{InstanceID: instance})
					if err != nil {
						return false, false, errors.Wrap(err, "get approval instance")
					}

					comment := ""
					// nodeKeyMap is used to get the node key from the custom node key
					nodeKeyMap := larkservice.GetLarkApprovalInstanceManager(instance).GetNodeKeyMap()
					if nodeData, ok := instanceData.ApproverInfoWithNode[nodeKeyMap[lark.ApprovalNodeIDKey(i)]]; ok {
						if userData, ok := nodeData[user.ID]; ok {
							comment = userData.Comment
						}
					}
					user.Comment = comment
					user.RejectOrApprove = result.ApproveOrReject
					user.OperationTime = result.OperationTime
					userUpdated = true
				}
			}
			node.RejectOrApprove, err = checkNodeStatus(node)
			if err != nil {
				return false, false, err
			}
			if node.RejectOrApprove == config.Approve {
				ack()
				break
			}
			if node.RejectOrApprove == config.Reject {
				return true, false, nil
			}
			if userUpdated {
				ack()
				break
			}
		}

		finalResult := larkApproval.ApprovalNodes[len(larkApproval.ApprovalNodes)-1].RejectOrApprove
		return finalResult != "", finalResult == config.Approve, nil
	}

	defer func() {
		larkservice.RemoveLarkApprovalInstanceManager(instance)
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
			done, isApprove, err := approvalUpdate(approval)
			if err != nil {
				stage.Status = config.StatusFailed
				cancelApproval()
				return errors.Wrap(err, "check approval status")
			}
			if done {
				finalInstance, err := client.GetApprovalInstance(&lark.GetApprovalInstanceArgs{InstanceID: instance})
				if err != nil {
					stage.Status = config.StatusFailed
					return errors.Wrap(err, "get approval final instance")
				}
				if finalInstance.ApproveOrReject == config.Approve && isApprove {
					return nil
				}
				if finalInstance.ApproveOrReject == config.Reject && !isApprove {
					stage.Status = config.StatusReject
					return errors.New("Approval has been rejected")
				}
				stage.Status = config.StatusFailed
				return errors.New("check final approval status failed")
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

	data, err := mongodb.NewIMAppColl().GetByID(context.Background(), approval.ID)
	if err != nil {
		stage.Status = config.StatusFailed
		return errors.Wrap(err, "get dingtalk im data")
	}

	client := dingtalk.NewClient(data.DingTalkAppKey, data.DingTalkAppSecret)

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
	formContent := fmt.Sprintf("项目名称: %s\n工作流名称: %s\n阶段名称: %s%s\n\n更多详见: %s",
		workflowCtx.ProjectName, workflowCtx.WorkflowDisplayName, stage.Name, descForm, detailURL)

	var userID string
	if approval.DefaultApprovalInitiator == nil {
		userIDResp, err := client.GetUserIDByMobile(workflowCtx.WorkflowTaskCreatorMobile)
		if err != nil {
			stage.Status = config.StatusFailed
			return errors.Wrapf(err, "get user dingtalk id by mobile-%s", workflowCtx.WorkflowTaskCreatorMobile)
		}
		userID = userIDResp.UserID
	} else {
		userID = approval.DefaultApprovalInitiator.ID
		formContent = fmt.Sprintf("审批发起人: %s\n%s", workflowCtx.WorkflowTaskCreatorUsername, formContent)
	}

	log.Infof("waitForDingTalkApprove: ApproveNode num %d", len(approval.ApprovalNodes))
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
		FormContent: formContent,
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

	checkNodeStatus := func(node *commonmodels.DingTalkApprovalNode) (config.ApproveOrReject, error) {
		users := node.ApproveUsers
		switch node.Type {
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
			return "", errors.Errorf("unknown node type %s", node.Type)
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
			userUpdated := false
			for _, node := range approval.ApprovalNodes {
				if node.RejectOrApprove != "" {
					continue
				}
				for _, user := range node.ApproveUsers {
					if result := userApprovalResult[user.ID]; result != nil && user.RejectOrApprove == "" {
						user.RejectOrApprove = resultMap[result.Result]
						user.Comment = result.Remark
						user.OperationTime = result.OperationTime
						userUpdated = true
					}
				}
				node.RejectOrApprove, err = checkNodeStatus(node)
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
				default:
					if userUpdated {
						ack()
					}
				}
				break
			}
			if approval.ApprovalNodes[len(approval.ApprovalNodes)-1].RejectOrApprove == config.Approve {
				instanceInfo, err := client.GetApprovalInstance(instanceID)
				if err != nil {
					stage.Status = config.StatusFailed
					log.Errorf("get instance final info failed: %v", err)
					return errors.Wrap(err, "get instance final info")
				}
				if instanceInfo.Status == "COMPLETED" && instanceInfo.Result == "agree" {
					return nil
				} else {
					log.Errorf("Unexpect instance final status is %s, result is %s", instanceInfo.Status, instanceInfo.Result)
					stage.Status = config.StatusFailed
					return errors.Wrap(err, "get unexpected instance final info")
				}
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

func updateStageStatus(ctx context.Context, stage *commonmodels.StageTask) {
	select {
	case <-ctx.Done():
		stage.Status = config.StatusCancelled
		return
	default:
	}
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
