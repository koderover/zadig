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
		status, err = waitForLarkApprove(ctx, c.jobTaskSpec, c.workflowCtx, c.job.Name, c.ack)
	case config.DingTalkApproval:
		status, err = waitForDingTalkApprove(ctx, c.jobTaskSpec, c.workflowCtx, c.job.Name, c.ack)
	case config.WorkWXApproval:
		status, err = waitForWorkWXApprove(ctx, c.jobTaskSpec, c.workflowCtx, c.job.Name, c.ack)
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
			approved, _, navtiveApproval, err := approvalservice.GlobalApproveMap.IsApproval(approveKey)
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

			if err != nil {
				return config.StatusReject, err
			}

			if approved {
				return config.StatusPassed, nil
			}
		}
	}
}

func waitForLarkApprove(ctx context.Context, spec *commonmodels.JobTaskApprovalSpec, workflowCtx *commonmodels.WorkflowTaskCtx, jobName string, ack func()) (config.Status, error) {
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
		workflowCtx.ProjectName, workflowCtx.WorkflowDisplayName, jobName, descForm, workflowCtx.Remark, detailURL)

	var userID string
	if approval.DefaultApprovalInitiator == nil {
		userID, err = client.GetUserIDByEmailOrMobile(lark.QueryTypeMobile, workflowCtx.WorkflowTaskCreatorMobile, setting.LarkUserOpenID)
		if err != nil {
			return config.StatusFailed, fmt.Errorf("get user lark id by mobile-%s, error: %s", workflowCtx.WorkflowTaskCreatorMobile, err)
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
			return "", fmt.Errorf("unknown node type %s", node.Type)
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
			resultMap := larkservice.GetNodeUserApprovalResults(instance, lark.ApprovalNodeIDKey(i))
			for _, user := range node.ApproveUsers {
				if result, ok := resultMap[user.ID]; ok && user.RejectOrApprove == "" {
					instanceData, err := client.GetApprovalInstance(&lark.GetApprovalInstanceArgs{InstanceID: instance})
					if err != nil {
						return false, false, fmt.Errorf("failed to get approval instance, error: %s", err)
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

	timeoutChan := time.After(time.Duration(timeout) * time.Minute)
	for {
		time.Sleep(1 * time.Second)
		select {
		case <-ctx.Done():
			cancelApproval()
			return config.StatusCancelled, fmt.Errorf("workflow was canceled")
		case <-timeoutChan:
			return config.StatusTimeout, fmt.Errorf("workflow timeout")
		default:
			done, isApprove, err := approvalUpdate(approval)
			if err != nil {
				cancelApproval()
				return config.StatusFailed, fmt.Errorf("failed to check approval status, error: %s", err)
			}

			if done {
				finalInstance, err := client.GetApprovalInstance(&lark.GetApprovalInstanceArgs{InstanceID: instance})
				if err != nil {
					return config.StatusFailed, fmt.Errorf("get approval final instance, error: %s", err)
				}
				if finalInstance.ApproveOrReject == config.Approve && isApprove {
					return config.StatusPassed, nil
				}
				if finalInstance.ApproveOrReject == config.Reject && !isApprove {
					return config.StatusReject, fmt.Errorf("Approval has been rejected")
				}
				return config.StatusFailed, errors.New("check final approval status failed")
			}
		}
	}
}

func waitForDingTalkApprove(ctx context.Context, spec *commonmodels.JobTaskApprovalSpec, workflowCtx *commonmodels.WorkflowTaskCtx, jobName string, ack func()) (config.Status, error) {
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
		workflowCtx.ProjectName, workflowCtx.WorkflowDisplayName, jobName, descForm, workflowCtx.Remark, detailURL)

	var userID string
	if approval.DefaultApprovalInitiator == nil {
		userIDResp, err := client.GetUserIDByMobile(workflowCtx.WorkflowTaskCreatorMobile)
		if err != nil {
			return config.StatusFailed, fmt.Errorf("get user dingtalk id by mobile-%s error: %s", workflowCtx.WorkflowTaskCreatorMobile, err)
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
			return "", fmt.Errorf("unknown node type %s", node.Type)
		}
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
			userApprovalResult := dingservice.GetAllUserApprovalResults(instanceID)
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
					log.Errorf("check node failed: %v", err)
					return config.StatusFailed, fmt.Errorf("check node failed, error: %s", err)
				}
				switch node.RejectOrApprove {
				case config.Approve:
					ack()
				case config.Reject:
					return config.StatusReject, fmt.Errorf("Approval has been rejected")
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
					log.Errorf("get instance final info failed: %v", err)
					return config.StatusFailed, fmt.Errorf("get instance final info error: %s", err)
				}
				if instanceInfo.Status == "COMPLETED" && instanceInfo.Result == "agree" {
					return config.StatusPassed, nil
				} else {
					log.Errorf("Unexpect instance final status is %s, result is %s", instanceInfo.Status, instanceInfo.Result)
					return config.StatusFailed, fmt.Errorf("get unexpected instance final info, error: %s", err)
				}
			}
		}
	}
}

func waitForWorkWXApprove(ctx context.Context, spec *commonmodels.JobTaskApprovalSpec, workflowCtx *commonmodels.WorkflowTaskCtx, jobName string, ack func()) (config.Status, error) {
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
		workflowCtx.ProjectName, workflowCtx.WorkflowDisplayName, jobName, descForm, workflowCtx.Remark, detailURL)

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
