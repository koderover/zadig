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
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"text/template"
	"time"

	html2md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/google/uuid"
	workwxservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/workwx"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/setting"
	util2 "github.com/koderover/zadig/v2/pkg/util"
	"github.com/pkg/errors"

	configbase "github.com/koderover/zadig/v2/pkg/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	approvalservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/approval"
	dingservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/dingtalk"
	larkservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/lark"
	"github.com/koderover/zadig/v2/pkg/shared/client/systemconfig"
	"github.com/koderover/zadig/v2/pkg/shared/client/user"
	"github.com/koderover/zadig/v2/pkg/tool/dingtalk"
	"github.com/koderover/zadig/v2/pkg/tool/lark"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/tool/mail"
	"github.com/koderover/zadig/v2/pkg/tool/workwx"
	"github.com/koderover/zadig/v2/pkg/types"
)

//go:embed approval.html
var approvalHTML []byte

func createApprovalInstance(plan *models.ReleasePlan, phone string) error {
	if plan.Approval == nil {
		return errors.New("createApprovalInstance: approval data not found")
	}

	detailURL := fmt.Sprintf("%s/v1/releasePlan/detail?id=%s",
		configbase.SystemAddress(),
		url.QueryEscape(plan.ID.Hex()),
	)

	formContent := fmt.Sprintf("发布计划名称: %s\n发布负责人: %s\n", plan.Name, plan.Manager)

	if plan.StartTime != 0 && plan.EndTime != 0 {
		formContent += fmt.Sprintf("发布窗口期: %s\n", time.Unix(plan.StartTime, 0).Format("2006-01-02 15:04:05")+"-"+time.Unix(plan.EndTime, 0).Format("2006-01-02 15:04:05"))
	}
	if plan.ScheduleExecuteTime != 0 {
		formContent += fmt.Sprintf("定时执行: %s\n", time.Unix(plan.ScheduleExecuteTime, 0).Format("2006-01-02 15:04"))
	}
	if plan.Description != "" {
		if plan.Approval.Type != config.NativeApproval {
			converter := html2md.NewConverter("", true, nil)
			markdownDescription, err := converter.ConvertString(plan.Description)
			if err != nil {
				log.Error("Error convert %s HTML to Markdown: %v", plan.Description, err)
			} else {
				formContent += fmt.Sprintf("需求关联: \n")
				descArr := strings.Split(markdownDescription, "\n")
				for _, desc := range descArr {
					formContent += fmt.Sprintf("	%s\n", desc)
				}
			}
		} else {
			formContent += fmt.Sprintf("需求关联: %s\n", plan.Description)
		}
	}

	formContent += fmt.Sprintf("\n更多详见: %s", detailURL)

	switch plan.Approval.Type {
	case config.NativeApproval:
		return createNativeApproval(plan, detailURL)
	case config.LarkApproval:
		return createLarkApproval(plan.Approval.LarkApproval, plan.Manager, phone, formContent)
	case config.DingTalkApproval:
		return createDingTalkApproval(plan.Approval.DingTalkApproval, plan.Manager, phone, formContent)
	case config.WorkWXApproval:
		return createWorkWXApproval(plan.Approval.WorkWXApproval, plan.Manager, phone, formContent)
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
		if phone == "" {
			return errors.New("审批发起人手机号码未找到，请正确配置您的手机号码")
		}
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

func updateWorkWXApproval(ctx context.Context, approvalInfo *models.Approval) error {
	if approvalInfo == nil || approvalInfo.WorkWXApproval == nil {
		return errors.New("updateWorkWXApproval: approval data not found")
	}

	approval := approvalInfo.WorkWXApproval
	instanceID := approval.InstanceID
	if instanceID == "" {
		return errors.New("updateWorkWXApproval: instance id not found")
	}

	userApprovalResult, err := workwxservice.GetWorkWXApprovalEvent(instanceID)
	if err != nil {
		return fmt.Errorf("updateWorkWXApproval: failed to handle workwx approval event, error: %s", err)
	}

	approvalInfo.WorkWXApproval.ApprovalNodeDetails = userApprovalResult.ProcessList.NodeList
	switch userApprovalResult.Status {
	case workwx.ApprovalStatusApproved:
		approvalInfo.Status = config.StatusPassed
		return nil
	case workwx.ApprovalStatusRejected:
		approvalInfo.Status = config.StatusReject
		return nil
	case workwx.ApprovalStatusDeleted:
		approvalInfo.Status = config.StatusCancelled
		return nil
	default:
		return nil
	}
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

	resultMap := map[string]config.ApprovalStatus{
		"agree":  config.ApprovalStatusApprove,
		"refuse": config.ApprovalStatusReject,
	}

	checkNodeStatus := func(node *models.DingTalkApprovalNode) (config.ApprovalStatus, error) {
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
			return "", errors.Errorf("unknown node type %s", node.Type)
		}
	}

	userApprovalResult := dingservice.GetAllUserApprovalResults(instanceID)
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
		case config.ApprovalStatusApprove:
		case config.ApprovalStatusReject:
			approvalInfo.Status = config.StatusReject
			return nil
		}
		break
	}
	if approval.ApprovalNodes[len(approval.ApprovalNodes)-1].RejectOrApprove == config.ApprovalStatusApprove {
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

func geneFlatNativeApprovalUsers(approval *models.NativeApproval) ([]*models.User, map[string]*types.UserInfo) {
	// change [group + user] approvals to user approvals
	return util.GeneFlatUsers(approval.ApproveUsers)
}

func createNativeApproval(plan *models.ReleasePlan, url string) error {
	if plan == nil || plan.Approval == nil || plan.Approval.NativeApproval == nil {
		return errors.New("createNativeApproval: native approval data not found")
	}
	approval := plan.Approval.NativeApproval

	approvalUsers, userMap := geneFlatNativeApprovalUsers(approval)

	// send email to all approval users if necessary
	go func() {
		var err error
		mailNotifyInfo := ""
		var email *systemconfig.Email
		var emailService *systemconfig.EmailService

		for {
			email, err = systemconfig.New().GetEmailHost()
			if err != nil {
				log.Errorf("CreateNativeApproval GetEmailHost error, error msg:%s", err)
				break
			}

			emailService, err = systemconfig.New().GetEmailService()
			if err != nil {
				log.Errorf("CreateNativeApproval GetEmailService error, error msg:%s", err)
				break
			}

			t, err := template.New("approval").Parse(string(approvalHTML))
			if err != nil {
				log.Errorf("CreateNativeApproval template parse error, error msg:%s", err)
				break
			}

			timeRange := ""
			if plan.StartTime != 0 && plan.EndTime != 0 {
				timeRange = time.Unix(plan.StartTime, 0).Format("2006-01-02 15:04:05") + "-" + time.Unix(plan.EndTime, 0).Format("2006-01-02 15:04:05")
			}
			scheduleExecuteTime := ""
			if plan.ScheduleExecuteTime != 0 {
				scheduleExecuteTime = time.Unix(plan.ScheduleExecuteTime, 0).Format("2006-01-02 15:04:05")
			}

			planDescription := ""
			if plan.JiraSprintAssociation != nil {
				if len(plan.JiraSprintAssociation.Sprints) > 0 {
					for _, sprint := range plan.JiraSprintAssociation.Sprints {
						planDescription += fmt.Sprintf("%s #%s, ", sprint.ProjectName, sprint.SprintName)
					}
					planDescription = planDescription[:len(planDescription)-2]
					planDescription += "\n" + plan.Description
				}
			} else {
				planDescription = "\n" + plan.Description
			}
			var buf bytes.Buffer
			err = t.Execute(&buf, struct {
				PlanName            string
				Manager             string
				Description         string
				TimeRange           string
				ScheduleExecuteTime string
				Url                 string
			}{
				PlanName:            plan.Name,
				Manager:             plan.Manager,
				Description:         planDescription,
				TimeRange:           timeRange,
				ScheduleExecuteTime: scheduleExecuteTime,
				Url:                 url,
			})
			if err != nil {
				log.Errorf("CreateNativeApproval template execute error, error msg:%s", err)
				break
			}
			mailNotifyInfo = buf.String()
			break
		}

		if email == nil {
			return
		}

		for _, u := range approvalUsers {
			info, ok := userMap[u.UserID]
			if !ok {
				info, err = user.New().GetUserByID(u.UserID)
				if err != nil {
					log.Warnf("CreateNativeApproval GetUserByUid error, error msg:%s", err)
					continue
				}
			}

			if info.Email == "" {
				log.Warnf("CreateNativeApproval user %s email is empty", info.Name)
				continue
			}
			err = mail.SendEmail(&mail.EmailParams{
				From:     emailService.Address,
				To:       info.Email,
				Subject:  fmt.Sprintf("发布计划 %s 待审批", plan.Name),
				Host:     email.Name,
				UserName: email.UserName,
				Password: email.Password,
				Port:     email.Port,
				Body:     mailNotifyInfo,
			})
			if err != nil {
				log.Errorf("CreateNativeApproval SendEmail error, error msg:%s", err)
				continue
			}
		}
	}()

	originApprovalUser := approval.ApproveUsers
	approval.ApproveUsers = approvalUsers

	approveKey := uuid.New().String()
	approval.InstanceCode = approveKey

	approvalservice.GlobalApproveMap.SetApproval(approveKey, approval)
	approval.ApproveUsers = originApprovalUser
	return nil
}

func createWorkWXApproval(approval *models.WorkWXApproval, manager, phone, content string) error {
	if approval == nil {
		return errors.New("waitForApprove: workwx approval data not found")
	}

	data, err := mongodb.NewIMAppColl().GetByID(context.Background(), approval.ID)
	if err != nil {
		return errors.Wrap(err, "get workwx im app data")
	}

	client := workwx.NewClient(data.Host, data.CorpID, data.AgentID, data.AgentSecret)
	var applicant string
	if approval.CreatorUser != nil {
		applicant = approval.CreatorUser.ID
	} else {
		if phone == "" {
			return errors.New("审批发起人手机号码未找到，请正确配置您的手机号码")
		}

		content = fmt.Sprintf("审批发起人: %s\n%s", manager, content)
		phoneInt, err := strconv.Atoi(phone)
		if err != nil {
			return errors.Wrap(err, "get applicant phone")
		}
		resp, err := client.FindUserByPhone(phoneInt)
		if err != nil {
			return errors.Wrap(err, "find approval applicant by applicant phone")
		}

		applicant = resp.UserID
	}

	applydata := make([]*workwx.ApplyDataContent, 0)
	applydata = append(applydata, &workwx.ApplyDataContent{
		Control: config.DefaultWorkWXApprovalControlType,
		Id:      config.DefaultWorkWXApprovalControlID,
		Value:   &workwx.TextApplyData{Text: content},
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
		log.Errorf("create workwx approval instance failed: %v", err)
		return errors.Wrap(err, "create approval instance")
	}

	approval.InstanceID = instanceID
	return nil
}

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
		if phone == "" {
			return errors.New("审批发起人手机号码未找到，请正确配置您的手机号码")
		}
		userInfo, err := client.GetUserIDByEmailOrMobile(lark.QueryTypeMobile, phone, setting.LarkUserOpenID)
		if err != nil {
			return errors.Wrapf(err, "get user lark id by mobile-%s", phone)
		}
		userID = util2.GetStringFromPointer(userInfo.UserId)
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

	checkNodeStatus := func(node *models.LarkApprovalNode) (config.ApprovalStatus, error) {
		switch node.Type {
		case "AND":
			result := config.ApprovalStatusApprove
			for _, user := range node.ApproveUsers {
				if user.RejectOrApprove == "" {
					result = ""
				}
				if user.RejectOrApprove == config.ApprovalStatusReject {
					return config.ApprovalStatusReject, nil
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
			resultMap := larkservice.GetNodeUserApprovalResults(instance, lark.ApprovalNodeIDKey(i))
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
			if node.RejectOrApprove == config.ApprovalStatusApprove {
				break
			}
			if node.RejectOrApprove == config.ApprovalStatusReject {
				return true, false, nil
			}
			if userUpdated {
				break
			}
		}

		finalResult := larkApproval.ApprovalNodes[len(larkApproval.ApprovalNodes)-1].RejectOrApprove
		return finalResult != "", finalResult == config.ApprovalStatusApprove, nil
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
		if finalInstance.ApproveOrReject == config.ApprovalStatusApprove && isApprove {
			approval.Status = config.StatusPassed
			return nil
		}
		if finalInstance.ApproveOrReject == config.ApprovalStatusReject && !isApprove {
			approval.Status = config.StatusReject
			return nil
		}
		return errors.New("check final larkApproval status failed")
	}
	return nil
}
