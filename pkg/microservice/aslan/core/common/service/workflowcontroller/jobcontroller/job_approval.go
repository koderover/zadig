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

package jobcontroller

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/koderover/zadig/v2/pkg/util"
	"go.uber.org/zap"

	configbase "github.com/koderover/zadig/v2/pkg/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	approvalservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/approval"
	dingservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/dingtalk"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/instantmessage"
	larkservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/lark"
	workwxservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/workwx"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/dingtalk"
	"github.com/koderover/zadig/v2/pkg/tool/lark"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/tool/workwx"
)

type ApprovalJobCtl struct {
	job         *commonmodels.JobTask
	workflowCtx *commonmodels.WorkflowTaskCtx
	logger      *zap.SugaredLogger
	jobTaskSpec *commonmodels.JobTaskApprovalSpec
	ack         func()
}

func NewApprovalJobCtl(job *commonmodels.JobTask, workflowCtx *commonmodels.WorkflowTaskCtx, ack func(), logger *zap.SugaredLogger) *ApprovalJobCtl {
	jobTaskSpec := &commonmodels.JobTaskApprovalSpec{}
	if err := commonmodels.IToi(job.Spec, jobTaskSpec); err != nil {
		logger.Error(err)
	}
	job.Spec = jobTaskSpec
	return &ApprovalJobCtl{
		job:         job,
		workflowCtx: workflowCtx,
		logger:      logger,
		ack:         ack,
		jobTaskSpec: jobTaskSpec,
	}
}

func (c *ApprovalJobCtl) Clean(ctx context.Context) {}

func (c *ApprovalJobCtl) Run(ctx context.Context) {
	c.job.Status = config.StatusWaitingApprove
	c.ack()

	var err error
	var status config.Status

	switch c.jobTaskSpec.Type {
	case config.NativeApproval:
		status, err = waitForNativeApprove(ctx, c.jobTaskSpec, c.workflowCtx.WorkflowName, c.job.Name, c.workflowCtx.TaskID, c.ack)
	case config.LarkApproval:
		status, err = waitForLarkApprove(ctx, c.jobTaskSpec, c.workflowCtx, c.job.DisplayName, c.ack)
	case config.DingTalkApproval:
		status, err = waitForDingTalkApprove(ctx, c.jobTaskSpec, c.workflowCtx, c.job.DisplayName, c.ack)
	case config.WorkWXApproval:
		status, err = waitForWorkWXApprove(ctx, c.jobTaskSpec, c.workflowCtx, c.job.DisplayName, c.ack)
	default:
		err = errors.New("invalid approval type")
	}

	c.job.Status = status
	if err != nil {
		c.job.Error = err.Error()
	}

	return
}

func waitForNativeApprove(ctx context.Context, spec *commonmodels.JobTaskApprovalSpec, workflowName, jobName string, taskID int64, ack func()) (config.Status, error) {
	log.Infof("waitForNativeApprove start")

	approval := spec.NativeApproval

	if approval == nil {
		return config.StatusFailed, fmt.Errorf("waitForApprove: native approval data not found")
	}

	timeout := spec.Timeout
	if timeout == 0 {
		timeout = 60
	}

	approveKey := fmt.Sprintf("%s-%s-%d", workflowName, jobName, taskID)
	approvalservice.GlobalApproveMap.SetApproval(approveKey, approval)
	defer func() {
		approvalservice.GlobalApproveMap.DeleteApproval(approveKey)
	}()
	if err := instantmessage.NewWeChatClient().SendWorkflowTaskApproveNotifications(workflowName, taskID); err != nil {
		log.Errorf("send approve notification failed, error: %v", err)
	}

	timeoutChan := time.After(time.Duration(timeout) * time.Minute)

	for {
		time.Sleep(1 * time.Second)
		select {
		case <-ctx.Done():
			return config.StatusCancelled, fmt.Errorf("workflow was canceled")
		case <-timeoutChan:
			return config.StatusTimeout, fmt.Errorf("workflow timeout")
		default:
			approved, rejected, navtiveApproval, err := approvalservice.GlobalApproveMap.IsApproval(approveKey)
			if err != nil {
				return config.StatusFailed, fmt.Errorf("get approval status error: %s", err)
			}
			if navtiveApproval != nil {
				for _, nativeUser := range navtiveApproval.ApproveUsers {
					for _, user := range approval.ApproveUsers {
						if nativeUser.UserID == user.UserID {
							user.RejectOrApprove = nativeUser.RejectOrApprove
							user.Comment = nativeUser.Comment
							user.OperationTime = nativeUser.OperationTime
						}
					}
				}
			}

			// update the approval user information
			ack()

			if rejected {
				return config.StatusReject, nil
			} else if approved {
				return config.StatusPassed, nil
			}
		}
	}
}

func waitForLarkApprove(ctx context.Context, spec *commonmodels.JobTaskApprovalSpec, workflowCtx *commonmodels.WorkflowTaskCtx, jobDisplayName string, ack func()) (config.Status, error) {
	log.Infof("waitForLarkApprove start")
	approval := spec.LarkApproval
	if approval == nil {
		return config.StatusFailed, fmt.Errorf("waitForApprove: lark approval data not found")
	}

	timeout := spec.Timeout
	if timeout == 0 {
		timeout = 60
	}

	data, err := mongodb.NewIMAppColl().GetByID(context.Background(), approval.ID)
	if err != nil {
		return config.StatusFailed, fmt.Errorf("get lark im app data error: %s", err)
	}

	approvalCode := data.LarkApprovalCodeList[approval.GetNodeTypeKey()]
	if approvalCode == "" {
		log.Errorf("failed to find approval code for node type %s", approval.GetNodeTypeKey())
		return config.StatusFailed, fmt.Errorf("failed to find approval code for node type %s", approval.GetNodeTypeKey())
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
	if spec.Description != "" {
		descForm = fmt.Sprintf("\n描述: %s", spec.Description)
	}

	formContent := fmt.Sprintf("项目名称: %s\n工作流名称: %s\n任务名称: %s%s\n备注: %s\n\n更多详见: %s",
		workflowCtx.ProjectName, workflowCtx.WorkflowDisplayName, jobDisplayName, descForm, workflowCtx.Remark, detailURL)

	var userID string
	if approval.DefaultApprovalInitiator == nil {
		if workflowCtx.WorkflowTaskCreatorMobile == "" {
			return config.StatusFailed, errors.New("审批发起人手机号码未找到，请正确配置您的手机号码")
		}
		userInfo, err := client.GetUserIDByEmailOrMobile(lark.QueryTypeMobile, workflowCtx.WorkflowTaskCreatorMobile, setting.LarkUserOpenID)
		if err != nil {
			return config.StatusFailed, fmt.Errorf("get user lark id by mobile-%s, error: %s", workflowCtx.WorkflowTaskCreatorMobile, err)
		}
		userID = util.GetStringFromPointer(userInfo.UserId)
	} else {
		userID = approval.DefaultApprovalInitiator.ID
		formContent = fmt.Sprintf("工作流执行人: %s\n%s", workflowCtx.WorkflowTaskCreatorUsername, formContent)
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
		return config.StatusFailed, fmt.Errorf("create approval instance error: %s", err)
	}

	log.Infof("waitForLarkApprove: create instance success, id %s", instance)

	if err := instantmessage.NewWeChatClient().SendWorkflowTaskApproveNotifications(workflowCtx.WorkflowName, workflowCtx.TaskID); err != nil {
		log.Errorf("send approve notification failed, error: %v", err)
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

	checkNodeStatus := func(node *commonmodels.LarkApprovalNode) (config.ApprovalStatus, error) {
		switch node.Type {
		case "AND":
			totalCount := 0
			doneCount := 0
			redirectCount := 0
			approveCount := 0
			for _, user := range node.ApproveUsers {
				totalCount++
				if user.RejectOrApprove == config.ApprovalStatusDone {
					doneCount++
				}
				if user.RejectOrApprove == config.ApprovalStatusApprove {
					approveCount++
				}
				if user.RejectOrApprove == config.ApprovalStatusRedirect {
					redirectCount++
				}
				if user.RejectOrApprove == config.ApprovalStatusReject {
					return config.ApprovalStatusReject, nil
				}
			}

			if doneCount == totalCount {
				return config.ApprovalStatusDone, nil
			}
			if approveCount+doneCount+redirectCount == totalCount {
				return config.ApprovalStatusApprove, nil
			}
			return config.ApprovalStatusPending, nil
		case "OR":
			for _, user := range node.ApproveUsers {
				if user.RejectOrApprove == config.ApprovalStatusApprove {
					return config.ApprovalStatusApprove, nil
				}
				if user.RejectOrApprove == config.ApprovalStatusReject {
					return config.ApprovalStatusReject, nil
				}
			}
			return "", nil
		default:
			return "", fmt.Errorf("unknown node type %s", node.Type)
		}
	}

	checkApprovalFinished := func(nodes []*commonmodels.LarkApprovalNode) bool {
		count := len(nodes)
		finishedNode := 0
		for _, node := range nodes {
			if node.RejectOrApprove == config.ApprovalStatusReject {
				return true
			}

			if node.RejectOrApprove == config.ApprovalStatusApprove || node.RejectOrApprove == config.ApprovalStatusReject || node.RejectOrApprove == config.ApprovalStatusRedirect || node.RejectOrApprove == config.ApprovalStatusDone {
				finishedNode++
			}
		}

		if finishedNode == count {
			return true
		}

		return false
	}

	// approvalUpdate is used to update the approval status
	approvalUpdate := func(larkApproval *commonmodels.LarkApproval) (done bool, err error) {
		approvalUserMap := map[string]*commonmodels.LarkApprovalUser{}
		for _, node := range approval.ApprovalNodes {
			for _, approveUser := range node.ApproveUsers {
				approvalUserMap[approveUser.ID] = approveUser
			}
		}

		// userUpdated represents whether the user status has been updated
		userUpdated := false
		allResultMap := larkservice.GetUserApprovalResults(instance)
		nodeKeyMap := larkservice.GetLarkApprovalInstanceManager(instance).GetNodeKeyMap()
		for i, node := range larkApproval.ApprovalNodes {
			resultMap, ok := allResultMap[lark.ApprovalNodeIDKey(i)]
			if !ok {
				continue
			}

			if node.RejectOrApprove == config.ApprovalStatusReject || node.RejectOrApprove == config.ApprovalStatusApprove {
				for _, user := range node.ApproveUsers {
					delete(resultMap, user.ID)
				}
				continue
			}

			for _, user := range node.ApproveUsers {
				if result, ok := resultMap[user.ID]; ok && (user.RejectOrApprove == "" || user.RejectOrApprove == config.ApprovalStatusDone) {
					instanceData, err := client.GetApprovalInstance(&lark.GetApprovalInstanceArgs{InstanceID: instance})
					if err != nil {
						return false, fmt.Errorf("failed to get approval instance, error: %s", err)
					}

					comment := ""
					// nodeKeyMap is used to get the node key from the custom node key
					if nodeData, ok := instanceData.ApproverInfoWithNode[nodeKeyMap[lark.ApprovalNodeIDKey(i)]]; ok {
						if userData, ok := nodeData[user.ID]; ok {
							comment = userData.Comment
						}
					}
					user.Comment = comment
					user.RejectOrApprove = result.ApproveOrReject
					user.OperationTime = result.OperationTime
					userUpdated = true

					if user.RejectOrApprove == config.ApprovalStatusRedirect {
						for customNodeKey, taskMap := range instanceData.ApproverTaskWithNode {
							if customNodeKey == lark.ApprovalNodeIDKey(i) {
								for userID, task := range taskMap {
									if approvalUserMap[userID] == nil {
										userInfo, err := client.GetUserInfoByID(userID, setting.LarkUserOpenID)
										if err != nil {
											return false, fmt.Errorf("get user info %s failed, error: %s", userID, err)
										}

										redirectedUser := &commonmodels.LarkApprovalUser{
											UserInfo: lark.UserInfo{
												ID:     task.UserID,
												Name:   userInfo.Name,
												Avatar: userInfo.Avatar,
											},
											RejectOrApprove: task.Status,
										}
										node.ApproveUsers = append(node.ApproveUsers, redirectedUser)

										userUpdated = true
									}
								}
							}
						}
					}
				}

				delete(resultMap, user.ID)
			}

			node.RejectOrApprove, err = checkNodeStatus(node)
			if err != nil {
				return false, err
			}
			if node.RejectOrApprove == config.ApprovalStatusApprove {
				ack()
				break
			}
			if node.RejectOrApprove == config.ApprovalStatusReject {
				return true, nil
			}
			if userUpdated {
				ack()
				break
			}
		}

		newNodeKeyUserMap := map[string]map[string]*commonmodels.LarkApprovalUser{}
		for nodeKey, resultMap := range allResultMap {
			for userID, result := range resultMap {
				if newNodeKeyUserMap[nodeKey] == nil {
					newNodeKeyUserMap[nodeKey] = map[string]*commonmodels.LarkApprovalUser{}
				}
				user := &commonmodels.LarkApprovalUser{}
				newNodeKeyUserMap[nodeKey][userID] = user

				userInfo, err := client.GetUserInfoByID(userID, setting.LarkUserOpenID)
				if err != nil {
					return false, fmt.Errorf("get user info %s failed, error: %s", userID, err)
				}

				user.UserInfo = lark.UserInfo{
					ID:     userInfo.ID,
					Name:   userInfo.Name,
					Avatar: userInfo.Avatar,
				}

				user.RejectOrApprove = result.ApproveOrReject
				user.OperationTime = result.OperationTime

				instanceData, err := client.GetApprovalInstance(&lark.GetApprovalInstanceArgs{InstanceID: instance})
				if err != nil {
					return false, fmt.Errorf("failed to get approval instance, error: %s", err)
				}
				comment := ""
				// nodeKeyMap is used to get the node key from the custom node key
				if nodeData, ok := instanceData.ApproverInfoWithNode[nodeKeyMap[nodeKey]]; ok {
					if userData, ok := nodeData[user.ID]; ok {
						comment = userData.Comment
					}
				}
				user.Comment = comment
			}
		}
		if len(newNodeKeyUserMap) > 0 {
			for nodeKey, userMap := range newNodeKeyUserMap {
				foundNode := false
				for i, node := range larkApproval.ApprovalNodes {
					if nodeKey == lark.ApprovalNodeIDKey(i) {
						foundNode = true
						for _, user := range userMap {
							node.ApproveUsers = append(node.ApproveUsers, user)
						}
					}
				}

				if !foundNode {
					newNode := &commonmodels.LarkApprovalNode{
						ApproveUsers: []*commonmodels.LarkApprovalUser{},
					}
					for _, user := range userMap {
						newNode.ApproveUsers = append(newNode.ApproveUsers, user)
					}
					larkApproval.ApprovalNodes = append(larkApproval.ApprovalNodes, newNode)
				}
			}
			ack()
		}

		finished := checkApprovalFinished(larkApproval.ApprovalNodes)
		return finished, nil
	}

	defer func() {
		larkservice.RemoveLarkApprovalInstanceManager(instance)
	}()

	timeoutChan := time.After(time.Duration(timeout) * time.Minute)
	for {
		time.Sleep(1 * time.Second)
		select {
		case <-ctx.Done():
			cancelApproval()
			return config.StatusCancelled, fmt.Errorf("workflow was canceled")
		case <-timeoutChan:
			cancelApproval()
			return config.StatusTimeout, fmt.Errorf("workflow timeout")
		default:
			done, err := approvalUpdate(approval)
			if err != nil {
				cancelApproval()
				return config.StatusFailed, fmt.Errorf("failed to check approval status, error: %s", err)
			}

			if done {
				finalInstance, err := client.GetApprovalInstance(&lark.GetApprovalInstanceArgs{InstanceID: instance})
				if err != nil {
					return config.StatusFailed, fmt.Errorf("get approval final instance, error: %s", err)
				}

				if finalInstance.ApproveOrReject == config.ApprovalStatusApprove {
					return config.StatusPassed, nil
				}
				if finalInstance.ApproveOrReject == config.ApprovalStatusReject {
					return config.StatusReject, nil
				}
			}
		}
	}
}

func waitForDingTalkApprove(ctx context.Context, spec *commonmodels.JobTaskApprovalSpec, workflowCtx *commonmodels.WorkflowTaskCtx, jobDisplayName string, ack func()) (config.Status, error) {
	log.Infof("waitForDingTalkApprove start")
	approval := spec.DingTalkApproval
	if approval == nil {
		return config.StatusFailed, fmt.Errorf("waitForApprove: dingtalk approval data not found")
	}

	timeout := spec.Timeout
	if timeout == 0 {
		timeout = 60
	}

	data, err := mongodb.NewIMAppColl().GetByID(context.Background(), approval.ID)
	if err != nil {
		return config.StatusFailed, fmt.Errorf("get dingtalk im data error: %s", err)
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
	if spec.Description != "" {
		descForm = fmt.Sprintf("\n描述: %s", spec.Description)
	}
	formContent := fmt.Sprintf("项目名称: %s\n工作流名称: %s\n任务名称: %s%s\n\n备注: %s\n\n更多详见: %s",
		workflowCtx.ProjectName, workflowCtx.WorkflowDisplayName, jobDisplayName, descForm, workflowCtx.Remark, detailURL)

	var userID string
	if approval.DefaultApprovalInitiator == nil {
		if workflowCtx.WorkflowTaskCreatorMobile == "" {
			return config.StatusFailed, errors.New("审批发起人手机号码未找到，请正确配置您的手机号码")
		}
		userIDResp, err := client.GetUserIDByMobile(workflowCtx.WorkflowTaskCreatorMobile)
		if err != nil {
			return config.StatusFailed, fmt.Errorf("get user dingtalk id by mobile-%s error: %s", workflowCtx.WorkflowTaskCreatorMobile, err)
		}
		userID = userIDResp.UserID
	} else {
		userID = approval.DefaultApprovalInitiator.ID
		formContent = fmt.Sprintf("工作流执行人: %s\n%s", workflowCtx.WorkflowTaskCreatorUsername, formContent)
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
		return config.StatusFailed, fmt.Errorf("create approval instance failed, error: %s", err)
	}
	instanceID := instanceResp.InstanceID
	log.Infof("waitForDingTalkApprove: create instance success, id %s", instanceID)

	if err := instantmessage.NewWeChatClient().SendWorkflowTaskApproveNotifications(workflowCtx.WorkflowName, workflowCtx.TaskID); err != nil {
		log.Errorf("send approve notification failed, error: %v", err)
	}
	defer func() {
		dingservice.RemoveDingTalkApprovalManager(instanceID)
	}()

	resultMap := map[string]config.ApprovalStatus{
		"agree":    config.ApprovalStatusApprove,
		"refuse":   config.ApprovalStatusReject,
		"redirect": config.ApprovalStatusRedirect,
	}

	checkNodeStatus := func(node *commonmodels.DingTalkApprovalNode) (config.ApprovalStatus, error) {
		users := node.ApproveUsers
		switch node.Type {
		case "AND":
			result := config.ApprovalStatusApprove
			for _, user := range users {
				if user.RejectOrApprove == "" {
					result = ""
				}
				if user.RejectOrApprove == config.ApprovalStatusReject {
					return config.ApprovalStatusReject, nil
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
			return "", fmt.Errorf("unknown node type %s", node.Type)
		}
	}

	checkApprovalFinished := func(nodes []*commonmodels.DingTalkApprovalNode) bool {
		count := len(nodes)
		finishedNode := 0
		for _, node := range nodes {
			if node.RejectOrApprove == config.ApprovalStatusApprove || node.RejectOrApprove == config.ApprovalStatusRedirect {
				finishedNode++
			}
		}

		if finishedNode == count {
			return true
		}

		return false
	}

	timeoutChan := time.After(time.Duration(timeout) * time.Minute)
	for {
		time.Sleep(1 * time.Second)
		select {
		case <-ctx.Done():
			return config.StatusCancelled, fmt.Errorf("workflow was canceled")
		case <-timeoutChan:
			return config.StatusTimeout, fmt.Errorf("workflow timeout")
		default:
			userUpdated := false
			userApprovalResult := dingservice.GetAllUserApprovalResults(instanceID)

			approvalUserMap := map[string]*commonmodels.DingTalkApprovalUser{}
			for _, node := range approval.ApprovalNodes {
				for _, approveUser := range node.ApproveUsers {
					approvalUserMap[approveUser.ID] = approveUser
				}
			}

			for _, node := range approval.ApprovalNodes {
				if node.RejectOrApprove != "" {
					continue
				}

				isRedirected := false
				for _, user := range node.ApproveUsers {
					if result := userApprovalResult[user.ID]; result != nil {
						if user.RejectOrApprove == resultMap[result.Result] &&
							user.Comment == result.Remark &&
							user.OperationTime == result.OperationTime {
							continue
						}

						user.RejectOrApprove = resultMap[result.Result]
						user.Comment = result.Remark
						user.OperationTime = result.OperationTime
						userUpdated = true

						if user.RejectOrApprove == config.ApprovalStatusRedirect {
							isRedirected = true
						}
					}
				}

				if isRedirected {
					instanceInfo, err := client.GetApprovalInstance(instanceID)
					if err != nil {
						log.Errorf("get instance final info failed: %v", err)
						return config.StatusFailed, fmt.Errorf("get instance final info error: %s", err)
					}

					timeLayout := "2006-01-02T15:04Z"
					operationRecordMap := map[string]*dingtalk.OperationRecord{}
					for _, record := range instanceInfo.OperationRecords {
						oldRecord, ok := operationRecordMap[record.UserID]
						if !ok {
							operationRecordMap[record.UserID] = record
						} else {
							oldTime, err := time.Parse(timeLayout, oldRecord.Date)
							if err != nil {
								return config.StatusFailed, fmt.Errorf("parse operation time failed: %s", err)
							}
							newTime, err := time.Parse(timeLayout, record.Date)
							if err != nil {
								return config.StatusFailed, fmt.Errorf("parse operation time failed: %s", err)
							}

							if newTime.After(oldTime) {
								operationRecordMap[record.UserID] = record
							}
						}
					}

					for _, task := range instanceInfo.Tasks {
						if _, ok := approvalUserMap[task.UserID]; !ok {
							operationTime := int64(0)
							record, ok := operationRecordMap[task.UserID]
							if !ok {
								record = &dingtalk.OperationRecord{
									UserID: task.UserID,
									Result: task.Result,
								}
							} else {
								t, err := time.Parse(timeLayout, record.Date)
								if err != nil {
									return config.StatusFailed, fmt.Errorf("parse operation time failed: %s", err)
								}
								operationTime = t.Unix()
							}

							userInfo, err := client.GetUserInfo(task.UserID)
							if err != nil {
								err = fmt.Errorf("get user info %s failed: %s", task.UserID, err)
								log.Error(err)
								return config.StatusFailed, err
							}

							redirectedUser := &commonmodels.DingTalkApprovalUser{
								ID:              task.UserID,
								Name:            userInfo.Name,
								RejectOrApprove: resultMap[task.Result],
								Comment:         record.Remark,
								OperationTime:   operationTime,
							}
							node.ApproveUsers = append(node.ApproveUsers, redirectedUser)

							userUpdated = true
						}
					}
				}

				node.RejectOrApprove, err = checkNodeStatus(node)
				if err != nil {
					log.Errorf("check node failed: %v", err)
					return config.StatusFailed, fmt.Errorf("check node failed, error: %s", err)
				}

				switch node.RejectOrApprove {
				case config.ApprovalStatusApprove:
					ack()
				case config.ApprovalStatusRedirect:
					ack()
				case config.ApprovalStatusReject:
					return config.StatusReject, fmt.Errorf("Approval has been rejected")
				default:
					if userUpdated {
						ack()
					}
				}
				break
			}

			if checkApprovalFinished(approval.ApprovalNodes) {
				instanceInfo, err := client.GetApprovalInstance(instanceID)
				if err != nil {
					log.Errorf("get instance final info failed: %v", err)
					return config.StatusFailed, fmt.Errorf("get instance final info error: %s", err)
				}
				if instanceInfo.Status == "COMPLETED" && instanceInfo.Result == "agree" {
					return config.StatusPassed, nil
				} else {
					err = fmt.Errorf("Unexpect instance final status is %s, result is %s", instanceInfo.Status, instanceInfo.Result)
					log.Error(err)
					return config.StatusFailed, err
				}
			}
		}
	}
}

func waitForWorkWXApprove(ctx context.Context, spec *commonmodels.JobTaskApprovalSpec, workflowCtx *commonmodels.WorkflowTaskCtx, jobDisplayName string, ack func()) (config.Status, error) {
	log.Infof("waitForWorkWXApprove start...")
	approval := spec.WorkWXApproval
	if approval == nil {
		return config.StatusFailed, fmt.Errorf("waitForApprove: workwx approval data not found")
	}

	timeout := spec.Timeout
	if timeout == 0 {
		timeout = 60
	}

	data, err := mongodb.NewIMAppColl().GetByID(context.Background(), approval.ID)
	if err != nil {
		return config.StatusFailed, fmt.Errorf("get workwx im data, error: %s", err)
	}

	client := workwx.NewClient(data.Host, data.CorpID, data.AgentID, data.AgentSecret)

	detailURL := fmt.Sprintf("%s/v1/projects/detail/%s/pipelines/custom/%s/%d?display_name=%s",
		configbase.SystemAddress(),
		workflowCtx.ProjectName,
		workflowCtx.WorkflowName,
		workflowCtx.TaskID,
		url.QueryEscape(workflowCtx.WorkflowDisplayName),
	)

	descForm := ""
	if spec.Description != "" {
		descForm = fmt.Sprintf("\n描述: %s", spec.Description)
	}

	formContent := fmt.Sprintf("项目名称: %s\n\n工作流名称: %s\n\n任务名称: %s%s\n备注: %s\n\n更多详见: %s",
		workflowCtx.ProjectName, workflowCtx.WorkflowDisplayName, jobDisplayName, descForm, workflowCtx.Remark, detailURL)

	var applicant string
	if approval.CreatorUser != nil {
		applicant = approval.CreatorUser.ID
	} else {
		phoneInt, err := strconv.Atoi(workflowCtx.WorkflowTaskCreatorMobile)
		if err != nil {
			return config.StatusFailed, fmt.Errorf("get task creator phone failed, error: %s", err)
		}
		resp, err := client.FindUserByPhone(phoneInt)
		if err != nil {
			return config.StatusFailed, fmt.Errorf("find approval applicant by task creator phone error: %s", err)
		}

		applicant = resp.UserID
	}

	log.Infof("waitforWorkWXApprove: ApproveNode num %d", len(approval.ApprovalNodes))

	applydata := make([]*workwx.ApplyDataContent, 0)
	applydata = append(applydata, &workwx.ApplyDataContent{
		Control: config.DefaultWorkWXApprovalControlType,
		Id:      config.DefaultWorkWXApprovalControlID,
		Value:   &workwx.TextApplyData{Text: formContent},
	})

	for _, node := range approval.ApprovalNodes {
		userIDList := make([]string, 0)
		for _, user := range node.Users {
			userIDList = append(userIDList, user.ID)
		}
		node.UserID = userIDList
	}

	instanceID, err := client.CreateApprovalInstance(
		data.WorkWXApprovalTemplateID,
		applicant,
		false,
		applydata,
		approval.ApprovalNodes,
		make([]*workwx.ApprovalSummary, 0),
	)
	if err != nil {
		log.Errorf("waitForWorkWXApprove: create instance failed: %v", err)
		return config.StatusFailed, fmt.Errorf("create approval instance failed, error: %s", err)
	}
	spec.WorkWXApproval.InstanceID = instanceID
	ack()
	log.Infof("waitForWorkWXApprove: create instance success, id %s", instanceID)

	defer func() {
		workwxservice.RemoveWorkWXApprovalManager(instanceID)
	}()

	timeoutChan := time.After(time.Duration(timeout) * time.Minute)
	for {
		time.Sleep(1 * time.Second)
		select {
		case <-ctx.Done():
			return config.StatusCancelled, fmt.Errorf("workflow was canceled")
		case <-timeoutChan:
			return config.StatusTimeout, fmt.Errorf("workflow timeout")
		default:
			userApprovalResult, err := workwxservice.GetWorkWXApprovalEvent(instanceID)
			if err != nil {
				log.Warnf("failed to handle workwx approval event, error: %s", err)
				continue
			}

			spec.WorkWXApproval.ApprovalNodeDetails = userApprovalResult.ProcessList.NodeList
			ack()

			switch userApprovalResult.Status {
			case workwx.ApprovalStatusApproved:
				return config.StatusPassed, nil
			case workwx.ApprovalStatusRejected:
				return config.StatusReject, errors.New("Approval has been rejected")
			case workwx.ApprovalStatusDeleted:
				return config.StatusCancelled, errors.New("Approval has been deleted")
			default:
				continue
			}
		}
	}
}

func (c *ApprovalJobCtl) SaveInfo(ctx context.Context) error {
	return mongodb.NewJobInfoColl().Create(ctx, &commonmodels.JobInfo{
		Type:                c.job.JobType,
		WorkflowName:        c.workflowCtx.WorkflowName,
		WorkflowDisplayName: c.workflowCtx.WorkflowDisplayName,
		TaskID:              c.workflowCtx.TaskID,
		ProductName:         c.workflowCtx.ProjectName,
		StartTime:           c.job.StartTime,
		EndTime:             c.job.EndTime,
		Duration:            c.job.EndTime - c.job.StartTime,
		Status:              string(c.job.Status),
	})
}
