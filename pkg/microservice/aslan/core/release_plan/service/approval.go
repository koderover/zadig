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

	configbase "github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	dingservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/dingtalk"
	larkservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/lark"
	"github.com/koderover/zadig/pkg/tool/dingtalk"
	"github.com/koderover/zadig/pkg/tool/lark"
	"github.com/koderover/zadig/pkg/tool/log"
)

func createApprovalInstance(plan *models.ReleasePlan, phone string) error {
	if plan.Approval == nil {
		return errors.New("createApprovalInstance: approval data not found")
	}

	// todo
	detailURL := fmt.Sprintf("%s/v1/projects/detail/%s/pipelines/custom/%s/%d?display_name=%s",
		configbase.SystemAddress(),
		//workflowCtx.ProjectName,
		//workflowCtx.WorkflowName,
		//workflowCtx.TaskID,
		//url.QueryEscape(workflowCtx.WorkflowDisplayName),
	)

	formContent := fmt.Sprintf("发布计划名称: %s\n发布负责人: %s\n发布窗口期: %s\n\n更多详见: %s",
		plan.Name, plan.Manager,
		time.Unix(plan.StartTime, 0).Format("2006-01-02 15:04:05")+"-"+time.Unix(plan.EndTime, 0).Format("2006-01-02 15:04:05"),
		detailURL)

	switch plan.Approval.Type {
	//case config.NativeApproval:
	//	return createNativeApproval(ctx, stage, workflowCtx, logger, ack)
	case config.LarkApproval:
		return createLarkApproval(plan.Approval.LarkApproval, plan.Manager, phone, formContent)
	case config.DingTalkApproval:
		return createDingTalkApproval(plan.Approval.DingTalkApproval, plan.Manager, phone, formContent)
	default:
		return errors.New("invalid approval type")
	}
}

func createDingTalkApproval(approval *models.DingTalkApproval, manager, phone, content string) error {
	if approval == nil {
		return errors.New("waitForApprove: dingtalk approval data not found")
	}

	data, err := mongodb.NewIMAppColl().GetByID(context.Background(), approval.ID)
	if err != nil {
		return errors.Wrap(err, "get dingtalk im data")
	}

	client := dingtalk.NewClient(data.DingTalkAppKey, data.DingTalkAppSecret)

	var userID string
	if approval.DefaultApprovalInitiator == nil {
		userIDResp, err := client.GetUserIDByMobile(phone)
		if err != nil {
			return errors.Wrapf(err, "get user dingtalk id by mobile-%s", phone)
		}
		userID = userIDResp.UserID
	} else {
		userID = approval.DefaultApprovalInitiator.ID
		content = fmt.Sprintf("审批发起人: %s\n%s", manager, content)
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
		FormContent: content,
	})
	if err != nil {
		return errors.Wrap(err, "create approval instance")
	}

	approval.InstanceCode = instanceResp.InstanceID
	return nil
}

func updateDingTalkApproval(ctx context.Context, approvalInfo *models.Approval) error {
	if approvalInfo == nil || approvalInfo.DingTalkApproval == nil {
		return errors.New("updateDingTalkApproval: approval data not found")
	}
	approval := approvalInfo.DingTalkApproval
	instanceID := approval.InstanceCode
	if instanceID == "" {
		return errors.New("updateDingTalkApproval: instance id not found")
	}

	data, err := mongodb.NewIMAppColl().GetByID(context.Background(), approval.ID)
	if err != nil {
		return errors.Wrap(err, "get dingtalk im data")
	}
	client := dingtalk.NewClient(data.DingTalkAppKey, data.DingTalkAppSecret)

	resultMap := map[string]config.ApproveOrReject{
		"agree":  config.Approve,
		"refuse": config.Reject,
	}

	checkNodeStatus := func(node *models.DingTalkApprovalNode) (config.ApproveOrReject, error) {
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
		node.RejectOrApprove, err = checkNodeStatus(node)
		if err != nil {
			return errors.Wrap(err, "check node")
		}
		switch node.RejectOrApprove {
		case config.Approve:
		case config.Reject:
			approvalInfo.Status = config.StatusReject
			return nil
		}
		break
	}
	if approval.ApprovalNodes[len(approval.ApprovalNodes)-1].RejectOrApprove == config.Approve {
		instanceInfo, err := client.GetApprovalInstance(instanceID)
		if err != nil {
			return errors.Wrap(err, "get instance final info")
		}
		if instanceInfo.Status == "COMPLETED" && instanceInfo.Result == "agree" {
			approvalInfo.Status = config.StatusPassed
			return nil
		} else {
			log.Errorf("Unexpect instance final status is %s, result is %s", instanceInfo.Status, instanceInfo.Result)
			return errors.Wrap(err, "get unexpected instance final info")
		}
	}
	return nil
}

//func createNativeApproval(approval *models.LarkApproval) error {
//	approval := stage.Approval.NativeApproval
//	if approval == nil {
//		return errors.New("waitForApprove: native approval data not found")
//	}
//
//	if approval.Timeout == 0 {
//		approval.Timeout = 60
//	}
//	approveKey := fmt.Sprintf("%s-%d-%s", workflowCtx.WorkflowName, workflowCtx.TaskID, stage.Name)
//	approveWithL := &approveWithLock{approval: approval}
//	globalApproveMap.setApproval(approveKey, approveWithL)
//	defer func() {
//		globalApproveMap.deleteApproval(approveKey)
//		ack()
//	}()
//}

//func updateNativeApproval() error {
//	approval := stage.Approval.NativeApproval
//	if approval == nil {
//		return errors.New("waitForApprove: native approval data not found")
//	}
//
//	if approval.Timeout == 0 {
//		approval.Timeout = 60
//	}
//	approveKey := fmt.Sprintf("%s-%d-%s", workflowCtx.WorkflowName, workflowCtx.TaskID, stage.Name)
//	approveWithL := &approveWithLock{approval: approval}
//	globalApproveMap.setApproval(approveKey, approveWithL)
//	defer func() {
//		globalApproveMap.deleteApproval(approveKey)
//		ack()
//	}()
//	if err := instantmessage.NewWeChatClient().SendWorkflowTaskAproveNotifications(workflowCtx.WorkflowName, workflowCtx.TaskID); err != nil {
//		logger.Errorf("send approve notification failed, error: %v", err)
//	}
//
//	timeout := time.After(time.Duration(approval.Timeout) * time.Minute)
//	latestApproveCount := 0
//	for {
//		time.Sleep(1 * time.Second)
//		select {
//		case <-ctx.Done():
//			stage.Status = config.StatusCancelled
//			return fmt.Errorf("workflow was canceled")
//
//		case <-timeout:
//			stage.Status = config.StatusTimeout
//			return fmt.Errorf("workflow timeout")
//		default:
//			approved, approveCount, err := approveWithL.isApproval()
//			if err != nil {
//				stage.Status = config.StatusReject
//				return err
//			}
//			if approved {
//				return nil
//			}
//			if approveCount > latestApproveCount {
//				ack()
//				latestApproveCount = approveCount
//			}
//		}
//	}
//}

func createLarkApproval(approval *models.LarkApproval, manager, phone, content string) error {
	if approval == nil {
		return errors.New("waitForApprove: lark approval data not found")
	}

	data, err := mongodb.NewIMAppColl().GetByID(context.Background(), approval.ID)
	if err != nil {
		return errors.Wrap(err, "get lark im app data")
	}
	approvalCode := data.LarkApprovalCodeListCommon[approval.GetNodeTypeKey()]
	if approvalCode == "" {
		return errors.Errorf("failed to find approval code for node type %s", approval.GetNodeTypeKey())
	}

	client := lark.NewClient(data.AppID, data.AppSecret)

	var userID string
	if approval.DefaultApprovalInitiator == nil {
		userID, err = client.GetUserOpenIDByEmailOrMobile(lark.QueryTypeMobile, phone)
		if err != nil {
			return errors.Wrapf(err, "get user lark id by mobile-%s", phone)
		}
	} else {
		userID = approval.DefaultApprovalInitiator.ID
		content = fmt.Sprintf("审批发起人: %s\n%s", manager, content)
	}

	instance, err := client.CreateApprovalInstance(&lark.CreateApprovalInstanceArgs{
		ApprovalCode: approvalCode,
		UserOpenID:   userID,
		Nodes:        approval.GetLarkApprovalNode(),
		FormContent:  content,
	})
	if err != nil {
		return errors.Wrap(err, "create approval instance")
	}
	approval.InstanceCode = instance
	return nil
}

func updateLarkApproval(ctx context.Context, approval *models.Approval) error {
	if approval == nil || approval.LarkApproval == nil {
		return errors.New("updateLarkApproval: lark approval data not found")
	}
	larkApproval := approval.LarkApproval
	instance := larkApproval.InstanceCode
	if instance == "" {
		return errors.New("updateLarkApproval: lark approval instance code not found")
	}

	data, err := mongodb.NewIMAppColl().GetByID(ctx, larkApproval.ID)
	if err != nil {
		return errors.Wrap(err, "get lark im app data")
	}
	client := lark.NewClient(data.AppID, data.AppSecret)

	checkNodeStatus := func(node *models.LarkApprovalNode) (config.ApproveOrReject, error) {
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

	// approvalUpdate is used to update the larkApproval status
	approvalUpdate := func(larkApproval *models.LarkApproval) (done, isApprove bool, err error) {
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
						return false, false, errors.Wrap(err, "get larkApproval instance")
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
				break
			}
			if node.RejectOrApprove == config.Reject {
				return true, false, nil
			}
			if userUpdated {
				break
			}
		}

		finalResult := larkApproval.ApprovalNodes[len(larkApproval.ApprovalNodes)-1].RejectOrApprove
		return finalResult != "", finalResult == config.Approve, nil
	}

	done, isApprove, err := approvalUpdate(larkApproval)
	if err != nil {
		return errors.Wrap(err, "check larkApproval status")
	}
	if done {
		finalInstance, err := client.GetApprovalInstance(&lark.GetApprovalInstanceArgs{InstanceID: instance})
		if err != nil {
			return errors.Wrap(err, "get larkApproval final instance")
		}
		if finalInstance.ApproveOrReject == config.Approve && isApprove {
			approval.Status = config.StatusPassed
			return nil
		}
		if finalInstance.ApproveOrReject == config.Reject && !isApprove {
			approval.Status = config.StatusReject
			return errors.New("Approval has been rejected")
		}
		return errors.New("check final larkApproval status failed")
	}
	return nil
}
