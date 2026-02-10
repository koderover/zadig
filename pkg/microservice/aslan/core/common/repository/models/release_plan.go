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

package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
)

type ReleasePlan struct {
	ID      primitive.ObjectID `bson:"_id,omitempty"       yaml:"-"                   json:"id"`
	Index   int64              `bson:"index"       yaml:"index"                   json:"index"`
	Name    string             `bson:"name"       yaml:"name"                   json:"name"`
	Manager string             `bson:"manager"       yaml:"manager"                   json:"manager"`
	// ManagerID is the user id of the manager
	ManagerID             string                            `bson:"manager_id"       yaml:"manager_id"                   json:"manager_id"`
	ApproverID            string                            `bson:"approver_id"      yaml:"approver_id"                  json:"approver_id"`
	StartTime             int64                             `bson:"start_time"       yaml:"start_time"                   json:"start_time"`
	EndTime               int64                             `bson:"end_time"         yaml:"end_time"                     json:"end_time"`
	ScheduleExecuteTime   int64                             `bson:"schedule_execute_time"       yaml:"schedule_execute_time"                   json:"schedule_execute_time"`
	Description           string                            `bson:"description"       yaml:"description"                 json:"description"`
	CreatedBy             string                            `bson:"created_by"        yaml:"created_by"                  json:"created_by"`
	CreateTime            int64                             `bson:"create_time"       yaml:"create_time"                 json:"create_time"`
	UpdatedBy             string                            `bson:"updated_by"        yaml:"updated_by"                  json:"updated_by"`
	UpdateTime            int64                             `bson:"update_time"       yaml:"update_time"                 json:"update_time"`
	JiraSprintAssociation *ReleasePlanJiraSprintAssociation `bson:"jira_sprint_association"       yaml:"jira_sprint_association"                   json:"jira_sprint_association"`

	Approval *Approval `bson:"approval"       yaml:"approval"                   json:"approval,omitempty"`

	Jobs []*ReleaseJob `bson:"jobs"       yaml:"jobs"                   json:"jobs"`

	Status config.ReleasePlanStatus `bson:"status"       yaml:"status"                   json:"status"`

	PlanningTime       int64 `bson:"planning_time"            yaml:"planning_time"                   json:"planning_time"`
	FinishPlanningTime int64 `bson:"finish_planning_time"     yaml:"finish_planning_time"            json:"finish_planning_time"`
	ApprovalTime       int64 `bson:"approval_time"            yaml:"approval_time"                   json:"approval_time"`
	ExecutingTime      int64 `bson:"executing_time"           yaml:"executing_time"                  json:"executing_time"`
	SuccessTime        int64 `bson:"success_time"             yaml:"success_time"                    json:"success_time"`

	InstanceCode string        `bson:"instance_code"       yaml:"instance_code"                   json:"instance_code"`
	HookSettings *HookSettings `bson:"hook_settings"       yaml:"hook_settings"                   json:"hook_settings"`

	WaitForFinishPlanningExternalCheckTime int64  `bson:"wait_for_finish_planning_external_check_time"       yaml:"wait_for_finish_planning_external_check_time"                   json:"wait_for_finish_planning_external_check_time"`
	WaitForApproveExternalCheckTime        int64  `bson:"wait_for_approve_external_check_time"       yaml:"wait_for_approve_external_check_time"                   json:"wait_for_approve_external_check_time"`
	WaitForExecuteExternalCheckTime        int64  `bson:"wait_for_execute_external_check_time"       yaml:"wait_for_execute_external_check_time"                   json:"wait_for_execute_external_check_time"`
	WaitForAllDoneExternalCheckTime        int64  `bson:"wait_for_all_done_external_check_time"      yaml:"wait_for_all_done_external_check_time"                   json:"wait_for_all_done_external_check_time"`
	ExternalCheckFailedReason              string `bson:"external_check_failed_reason"       yaml:"external_check_failed_reason"                   json:"external_check_failed_reason"`
}

type HookSettings struct {
	Enable         bool                   `json:"enable" bson:"enable"`
	EnableCallBack bool                   `json:"enable_call_back" bson:"enable_call_back"`
	HookEvents     []ReleasePlanHookEvent `json:"hook_events" bson:"hook_events"`
}

type ReleasePlanJiraSprintAssociation struct {
	JiraID  string                  `bson:"jira_id"       yaml:"jira_id"                   json:"jira_id"`
	Sprints []ReleasePlanJiraSprint `bson:"sprints"       yaml:"sprints"                   json:"sprints"`
}

type ReleasePlanJiraSprint struct {
	ProjectName string `bson:"project_name"    yaml:"project_name"                json:"project_name"`
	ProjectKey  string `bson:"project_key"     yaml:"project_key"                 json:"project_key"`
	BoardID     int    `bson:"board_id"        yaml:"board_id"                    json:"board_id"`
	SprintID    int    `bson:"sprint_id"       yaml:"sprint_id"                   json:"sprint_id"`
	SprintName  string `bson:"sprint_name"     yaml:"sprint_name"                 json:"sprint_name"`
}

func (ReleasePlan) TableName() string {
	return "release_plan"
}

type ReleaseJob struct {
	ID        string                    `bson:"id"             yaml:"id"                     json:"id"`
	Name      string                    `bson:"name"           yaml:"name"                   json:"name"`
	Manager   string                    `bson:"manager"        yaml:"manager"                json:"manager"`
	ManagerID string                    `bson:"manager_id"     yaml:"manager_id"             json:"manager_id"`
	Type      config.ReleasePlanJobType `bson:"type"           yaml:"type"                   json:"type"`
	Spec      interface{}               `bson:"spec,omitempty" yaml:"spec,omitempty"         json:"spec,omitempty"`

	ReleaseJobRuntime `bson:",inline" yaml:",inline" json:",inline"`
}

type ReleaseJobRuntime struct {
	Status config.ReleasePlanJobStatus `bson:"status"     yaml:"status"                 json:"status"`
	// ReleasePlan can return to PlanningStatus when some release jobs have been executed
	// So we need to record the last status of the release job
	LastStatus config.ReleasePlanJobStatus `bson:"last_status"       yaml:"last_status"                   json:"last_status"`
	// Updated is used to indicate whether the release job has been updated
	Updated      bool   `bson:"updated"       yaml:"updated"                   json:"updated"`
	ExecutedBy   string `bson:"executed_by"       yaml:"executed_by"                   json:"executed_by"`
	ExecutedTime int64  `bson:"executed_time"       yaml:"executed_time"                   json:"executed_time"`
}

type TextReleaseJobSpec struct {
	Content string `bson:"content"       yaml:"content"                   json:"content"`
	Remark  string `bson:"remark"       yaml:"remark"                   json:"remark"`
}

type WorkflowReleaseJobSpec struct {
	Workflow *WorkflowV4   `bson:"workflow,omitempty"       yaml:"workflow,omitempty"                   json:"workflow,omitempty"`
	Status   config.Status `bson:"status,omitempty"         yaml:"status,omitempty"                     json:"status,omitempty"`
	TaskID   int64         `bson:"task_id,omitempty"        yaml:"task_id,omitempty"                    json:"task_id,omitempty"`
}

type ReleasePlanLog struct {
	ID         primitive.ObjectID `bson:"_id,omitempty"               json:"id"`
	PlanID     string             `bson:"plan_id"                     json:"plan_id"`
	Username   string             `bson:"username"                    json:"username"`
	Account    string             `bson:"account"                     json:"account"`
	Verb       string             `bson:"verb"                        json:"verb"`
	TargetName string             `bson:"target_name"                 json:"target_name"`
	TargetType string             `bson:"target_type"                 json:"target_type"`
	Before     interface{}        `bson:"before"                      json:"before"`
	After      interface{}        `bson:"after"                       json:"after"`
	Detail     string             `bson:"detail"                      json:"detail"`
	CreatedAt  int64              `bson:"created_at"                  json:"created_at"`
}

func (ReleasePlanLog) TableName() string {
	return "release_plan_log"
}
