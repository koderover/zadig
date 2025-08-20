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

	"github.com/google/uuid"
	"github.com/pkg/errors"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	larkservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/lark"
	"github.com/koderover/zadig/v2/pkg/shared/client/user"
	"github.com/koderover/zadig/v2/pkg/tool/lark"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

const (
	VerbUpdateName                = "update_name"
	VerbUpdateDesc                = "update_description"
	VerbUpdateTimeRange           = "update_time_range"
	VerbUpdateScheduleExecuteTime = "update_schedule_execute_time"
	VerbUpdateManager             = "update_manager"
	VerbUpdateJiraSprint          = "update_jira_sprint"

	VerbCreateReleaseJob = "create_release_job"
	VerbUpdateReleaseJob = "update_release_job"
	VerbDeleteReleaseJob = "delete_release_job"

	VerbUpdateApproval = "update_approval"
	VerbDeleteApproval = "delete_approval"

	TargetTypeReleasePlan       = "发布计划"
	TargetTypeReleasePlanStatus = "发布计划状态"
	TargetTypeMetadata          = "元数据"
	TargetTypeReleaseJob        = "发布内容"
	TargetTypeApproval          = "审批"
	TargetTypeDescription       = "需求关联"

	VerbCreate  = "新建"
	VerbUpdate  = "更新"
	VerbDelete  = "删除"
	VerbExecute = "执行"
	VerbSkip    = "跳过"
)

type PlanUpdater interface {
	// Update returns the old data and the updated data
	Update(plan *models.ReleasePlan) (before interface{}, after interface{}, err error)
	Verb() string
	TargetName() string
	TargetType() string
	Lint() error
}

func NewPlanUpdater(args *UpdateReleasePlanArgs) (PlanUpdater, error) {
	switch args.Verb {
	case VerbUpdateName:
		return NewNameUpdater(args)
	case VerbUpdateDesc:
		return NewDescUpdater(args)
	case VerbUpdateTimeRange:
		return NewTimeRangeUpdater(args)
	case VerbUpdateScheduleExecuteTime:
		return NewScheduleExecuteTimeUpdater(args)
	case VerbUpdateManager:
		return NewManagerUpdater(args)
	case VerbCreateReleaseJob:
		return NewCreateReleaseJobUpdater(args)
	case VerbUpdateReleaseJob:
		return NewUpdateReleaseJobUpdater(args)
	case VerbDeleteReleaseJob:
		return NewDeleteReleaseJobUpdater(args)
	case VerbUpdateApproval:
		return NewUpdateApprovalUpdater(args)
	case VerbDeleteApproval:
		return NewDeleteApprovalUpdater(args)
	case VerbUpdateJiraSprint:
		return NewJiraSprintUpdater(args)
	default:
		return nil, fmt.Errorf("invalid verb: %s", args.Verb)
	}
}

type NameUpdater struct {
	Name string `json:"name"`
}

func NewNameUpdater(args *UpdateReleasePlanArgs) (*NameUpdater, error) {
	var updater NameUpdater
	if err := models.IToi(args.Spec, &updater); err != nil {
		return nil, errors.Wrap(err, "invalid spec")
	}
	return &updater, nil
}

func (u *NameUpdater) Update(plan *models.ReleasePlan) (before interface{}, after interface{}, err error) {
	before, after = plan.Name, u.Name
	plan.Name = u.Name
	return
}

func (u *NameUpdater) Lint() error {
	if u.Name == "" {
		return fmt.Errorf("name cannot be empty")
	}
	return nil
}

func (u *NameUpdater) TargetName() string {
	return "发布计划名称"
}

func (u *NameUpdater) TargetType() string {
	return TargetTypeMetadata
}

func (u *NameUpdater) Verb() string {
	return VerbUpdate
}

type DescUpdater struct {
	Description string `json:"description"`
}

func NewDescUpdater(args *UpdateReleasePlanArgs) (*DescUpdater, error) {
	var updater DescUpdater
	if err := models.IToi(args.Spec, &updater); err != nil {
		return nil, errors.Wrap(err, "invalid spec")
	}
	return &updater, nil
}

func (u *DescUpdater) Update(plan *models.ReleasePlan) (before interface{}, after interface{}, err error) {
	before, after = plan.Description, u.Description
	plan.Description = u.Description
	return
}

func (u *DescUpdater) Lint() error {
	return nil
}

func (u *DescUpdater) TargetName() string {
	return "需求关联"
}

func (u *DescUpdater) TargetType() string {
	return TargetTypeDescription
}

func (u *DescUpdater) Verb() string {
	return VerbUpdate
}

type TimeRangeUpdater struct {
	StartTime int64 `json:"start_time"`
	EndTime   int64 `json:"end_time"`
}

func NewTimeRangeUpdater(args *UpdateReleasePlanArgs) (*TimeRangeUpdater, error) {
	var updater TimeRangeUpdater
	if err := models.IToi(args.Spec, &updater); err != nil {
		return nil, errors.Wrap(err, "invalid spec")
	}
	return &updater, nil
}

func (u *TimeRangeUpdater) Update(plan *models.ReleasePlan) (before interface{}, after interface{}, err error) {
	format := "2006-01-02 15:04:05"
	before = fmt.Sprintf("%s-%s", time.Unix(plan.StartTime, 0).Format(format),
		time.Unix(plan.EndTime, 0).Format(format))
	after = fmt.Sprintf("%s-%s", time.Unix(u.StartTime, 0).Format(format),
		time.Unix(u.EndTime, 0).Format(format))
	plan.StartTime = u.StartTime
	plan.EndTime = u.EndTime
	return
}

func (u *TimeRangeUpdater) Lint() error {
	return lintReleaseTimeRange(u.StartTime, u.EndTime)
}

func (u *TimeRangeUpdater) TargetName() string {
	return "发布窗口期"
}

func (u *TimeRangeUpdater) TargetType() string {
	return TargetTypeMetadata
}

func (u *TimeRangeUpdater) Verb() string {
	return VerbUpdate
}

type ManagerUpdater struct {
	ManagerID string `json:"manager_id"`
	Manager   string `json:"manager"`
}

func NewManagerUpdater(args *UpdateReleasePlanArgs) (*ManagerUpdater, error) {
	var updater ManagerUpdater
	if err := models.IToi(args.Spec, &updater); err != nil {
		return nil, errors.Wrap(err, "invalid spec")
	}
	return &updater, nil
}

func (u *ManagerUpdater) Update(plan *models.ReleasePlan) (before interface{}, after interface{}, err error) {
	before, after = plan.Manager, u.Manager
	plan.ManagerID = u.ManagerID
	plan.Manager = u.Manager
	return
}

func (u *ManagerUpdater) Lint() error {
	if u.ManagerID == "" {
		return fmt.Errorf("manager_id cannot be empty")
	}
	userInfo, err := user.New().GetUserByID(u.ManagerID)
	if err != nil {
		return fmt.Errorf("user not found")
	}
	if u.Manager != userInfo.Name {
		return fmt.Errorf("name not match")
	}
	return nil
}

func (u *ManagerUpdater) TargetName() string {
	return "发布负责人"
}

func (u *ManagerUpdater) TargetType() string {
	return TargetTypeMetadata
}

func (u *ManagerUpdater) Verb() string {
	return VerbUpdate
}

type CreateReleaseJobUpdater struct {
	Name string                    `json:"name"`
	Type config.ReleasePlanJobType `json:"type"`
	Spec interface{}               `json:"spec"`
}

func NewCreateReleaseJobUpdater(args *UpdateReleasePlanArgs) (*CreateReleaseJobUpdater, error) {
	var updater CreateReleaseJobUpdater
	if err := models.IToi(args.Spec, &updater); err != nil {
		return nil, errors.Wrap(err, "invalid spec")
	}
	return &updater, nil
}

func (u *CreateReleaseJobUpdater) Update(plan *models.ReleasePlan) (before interface{}, after interface{}, err error) {
	before, after = nil, u
	job := &models.ReleaseJob{
		ID:   uuid.New().String(),
		Name: u.Name,
		Type: u.Type,
		Spec: u.Spec,
	}
	plan.Jobs = append(plan.Jobs, job)
	return
}

func (u *CreateReleaseJobUpdater) Lint() error {
	if u.Name == "" {
		return fmt.Errorf("name cannot be empty")
	}

	return lintReleaseJob(u.Type, u.Spec)
}

func (u *CreateReleaseJobUpdater) TargetName() string {
	return u.Name
}

func (u *CreateReleaseJobUpdater) TargetType() string {
	return TargetTypeReleaseJob
}

func (u *CreateReleaseJobUpdater) Verb() string {
	return VerbCreate
}

type UpdateReleaseJobUpdater struct {
	ID   string                    `json:"id"`
	Name string                    `json:"name"`
	Type config.ReleasePlanJobType `json:"type"`
	Spec interface{}               `json:"spec"`
}

func NewUpdateReleaseJobUpdater(args *UpdateReleasePlanArgs) (*UpdateReleaseJobUpdater, error) {
	var updater UpdateReleaseJobUpdater
	if err := models.IToi(args.Spec, &updater); err != nil {
		return nil, errors.Wrap(err, "invalid spec")
	}
	return &updater, nil
}

func (u *UpdateReleaseJobUpdater) Update(plan *models.ReleasePlan) (before interface{}, after interface{}, err error) {
	for _, job := range plan.Jobs {
		if job.ID == u.ID {
			if job.Type != u.Type {
				return nil, nil, fmt.Errorf("job type cannot be changed")
			}
			before, after = job, u
			job.Name = u.Name
			job.Spec = u.Spec
			job.Updated = true
			return
		}
	}
	return nil, nil, fmt.Errorf("job %s-%s not found", u.Name, u.ID)
}

func (u *UpdateReleaseJobUpdater) Lint() error {
	if u.ID == "" {
		return fmt.Errorf("id cannot be empty")
	}
	if u.Name == "" {
		return fmt.Errorf("name cannot be empty")
	}
	return lintReleaseJob(u.Type, u.Spec)
}

func (u *UpdateReleaseJobUpdater) TargetName() string {
	return u.Name
}

func (u *UpdateReleaseJobUpdater) TargetType() string {
	return TargetTypeReleaseJob
}

func (u *UpdateReleaseJobUpdater) Verb() string {
	return VerbUpdate
}

type DeleteReleaseJobUpdater struct {
	ID   string `json:"id"`
	name string
}

func NewDeleteReleaseJobUpdater(args *UpdateReleasePlanArgs) (*DeleteReleaseJobUpdater, error) {
	var updater DeleteReleaseJobUpdater
	if err := models.IToi(args.Spec, &updater); err != nil {
		return nil, errors.Wrap(err, "invalid spec")
	}
	return &updater, nil
}

func (u *DeleteReleaseJobUpdater) Update(plan *models.ReleasePlan) (before interface{}, after interface{}, err error) {
	for i, job := range plan.Jobs {
		if job.ID == u.ID {
			u.name = job.Name
			plan.Jobs = append(plan.Jobs[:i], plan.Jobs[i+1:]...)
			return
		}
	}
	return nil, nil, fmt.Errorf("job %s not found", u.ID)
}

func (u *DeleteReleaseJobUpdater) Lint() error {
	if u.ID == "" {
		return fmt.Errorf("id cannot be empty")
	}
	return nil
}

func (u *DeleteReleaseJobUpdater) TargetName() string {
	return u.name
}

func (u *DeleteReleaseJobUpdater) TargetType() string {
	return TargetTypeReleaseJob
}

func (u *DeleteReleaseJobUpdater) Verb() string {
	return VerbDelete
}

type UpdateApprovalUpdater struct {
	Approval *models.Approval `json:"approval"`
}

func NewUpdateApprovalUpdater(args *UpdateReleasePlanArgs) (*UpdateApprovalUpdater, error) {
	var updater UpdateApprovalUpdater
	if err := models.IToi(args.Spec, &updater); err != nil {
		return nil, errors.Wrap(err, "invalid spec")
	}
	return &updater, nil
}

func (u *UpdateApprovalUpdater) Update(plan *models.ReleasePlan) (before interface{}, after interface{}, err error) {
	if err := clearApprovalData(u.Approval); err != nil {
		return nil, nil, errors.Wrap(err, "clear approval data")
	}
	before, after = plan.Approval, u.Approval
	plan.Approval = u.Approval
	return
}

func (u *UpdateApprovalUpdater) Lint() error {
	if u.Approval == nil {
		return fmt.Errorf("approval cannot be empty")
	}
	if err := lintApproval(u.Approval); err != nil {
		return errors.Wrap(err, "lint")
	}
	if u.Approval.Type == config.LarkApproval || u.Approval.Type == config.LarkApprovalIntl {
		if err := createLarkApprovalDefinition(u.Approval.LarkApproval); err != nil {
			return errors.Wrap(err, "create lark approval definition")
		}
	}
	return nil
}

func (u *UpdateApprovalUpdater) TargetName() string {
	return "审批"
}

func (u *UpdateApprovalUpdater) TargetType() string {
	return TargetTypeApproval
}

func (u *UpdateApprovalUpdater) Verb() string {
	return VerbUpdate
}

type DeleteApprovalUpdater struct {
}

func NewDeleteApprovalUpdater(args *UpdateReleasePlanArgs) (*DeleteApprovalUpdater, error) {
	return &DeleteApprovalUpdater{}, nil
}

func (u *DeleteApprovalUpdater) Update(plan *models.ReleasePlan) (before interface{}, after interface{}, err error) {
	before, after = plan.Approval, nil
	plan.Approval = nil
	return
}

func (u *DeleteApprovalUpdater) Lint() error {
	return nil
}

func (u *DeleteApprovalUpdater) TargetName() string {
	return "审批"
}

func (u *DeleteApprovalUpdater) TargetType() string {
	return TargetTypeApproval
}

func (u *DeleteApprovalUpdater) Verb() string {
	return VerbDelete
}

func createLarkApprovalDefinition(approval *models.LarkApproval) error {
	if approval == nil {
		return errors.Errorf("lark approval is nil")
	}
	larkInfo, err := commonrepo.NewIMAppColl().GetByID(context.Background(), approval.ID)
	if err != nil {
		return errors.Wrapf(err, "get lark app %s", approval.ID)
	}
	if larkInfo.Type != string(config.LarkApproval) && larkInfo.Type != string(config.LarkApprovalIntl) {
		return errors.Errorf("lark app %s is not lark approval", approval.ID)
	}

	if larkInfo.LarkApprovalCodeListCommon == nil {
		larkInfo.LarkApprovalCodeListCommon = make(map[string]string)
	}
	// skip if this node type approval definition already created
	if approvalNodeTypeID := larkInfo.LarkApprovalCodeListCommon[approval.GetNodeTypeKey()]; approvalNodeTypeID != "" {
		return nil
	}

	// create this node type approval definition and save to db
	client, err := larkservice.GetLarkClientByIMAppID(approval.ID)
	if err != nil {
		return errors.Wrapf(err, "get lark client by im app id %s", approval.ID)
	}
	nodesArgs := make([]*lark.ApprovalNode, 0)
	for _, node := range approval.ApprovalNodes {
		nodesArgs = append(nodesArgs, &lark.ApprovalNode{
			Type: node.Type,
			ApproverIDList: func() (re []string) {
				for _, user := range node.ApproveUsers {
					re = append(re, user.ID)
				}
				return
			}(),
		})
	}

	approvalCode, err := client.CreateApprovalDefinition(&lark.CreateApprovalDefinitionArgs{
		Name:        "Zadig 审批",
		Description: "Zadig 审批-" + approval.GetNodeTypeKey(),
		Nodes:       nodesArgs,
	})
	if err != nil {
		return errors.Wrap(err, "create lark approval definition")
	}
	err = client.SubscribeApprovalDefinition(&lark.SubscribeApprovalDefinitionArgs{
		ApprovalID: approvalCode,
	})
	if err != nil {
		return errors.Wrap(err, "subscribe lark approval definition")
	}
	larkInfo.LarkApprovalCodeListCommon[approval.GetNodeTypeKey()] = approvalCode
	if err := commonrepo.NewIMAppColl().Update(context.Background(), approval.ID, larkInfo); err != nil {
		return errors.Wrap(err, "update lark approval data")
	}
	log.Infof("create lark approval common definition %s, key: %s", approvalCode, approval.GetNodeTypeKey())

	return nil
}

type ScheduleExecuteTimeUpdater struct {
	StartTime           int64 `json:"start_time"`
	EndTime             int64 `json:"end_time"`
	ScheduleExecuteTime int64 `json:"schedule_execute_time"`
}

func NewScheduleExecuteTimeUpdater(args *UpdateReleasePlanArgs) (*ScheduleExecuteTimeUpdater, error) {
	var updater ScheduleExecuteTimeUpdater
	if err := models.IToi(args.Spec, &updater); err != nil {
		return nil, errors.Wrap(err, "invalid spec")
	}
	return &updater, nil
}

func (u *ScheduleExecuteTimeUpdater) Update(plan *models.ReleasePlan) (before interface{}, after interface{}, err error) {
	format := "2006-01-02 15:04:05"
	before = time.Unix(plan.ScheduleExecuteTime, 0).Format(format)
	after = time.Unix(u.ScheduleExecuteTime, 0).Format(format)
	plan.ScheduleExecuteTime = u.ScheduleExecuteTime
	return
}

func (u *ScheduleExecuteTimeUpdater) Lint() error {
	return lintScheduleExecuteTime(u.ScheduleExecuteTime, u.StartTime, u.EndTime)
}

func (u *ScheduleExecuteTimeUpdater) TargetName() string {
	return "定时执行"
}

func (u *ScheduleExecuteTimeUpdater) TargetType() string {
	return TargetTypeMetadata
}

func (u *ScheduleExecuteTimeUpdater) Verb() string {
	return VerbUpdate
}

type JiraSprintUpdater struct {
	JiraSprintAssociation *models.ReleasePlanJiraSprintAssociation `json:"jira_sprint_association"`
}

func NewJiraSprintUpdater(args *UpdateReleasePlanArgs) (*JiraSprintUpdater, error) {
	var updater JiraSprintUpdater
	if err := models.IToi(args.Spec, &updater); err != nil {
		return nil, errors.Wrap(err, "invalid spec")
	}
	return &updater, nil
}

func (u *JiraSprintUpdater) Update(plan *models.ReleasePlan) (before interface{}, after interface{}, err error) {
	before, after = plan.JiraSprintAssociation, u.JiraSprintAssociation
	plan.JiraSprintAssociation = u.JiraSprintAssociation
	return
}

func (u *JiraSprintUpdater) Lint() error {
	return nil
}

func (u *JiraSprintUpdater) TargetName() string {
	return "需求关联"
}

func (u *JiraSprintUpdater) TargetType() string {
	return TargetTypeDescription
}

func (u *JiraSprintUpdater) Verb() string {
	return VerbUpdate
}
