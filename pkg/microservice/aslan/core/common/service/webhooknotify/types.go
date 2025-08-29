/*
Copyright 2021 The KodeRover Authors.

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

package webhooknotify

import (
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/types"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	TokenHeader       = "X-Zadig-Token"
	InstanceHeader    = "X-Zadig-Instance"
	EventHeader       = "X-Zadig-Event"
	EventUUIDHeader   = "X-Zadig-Event-UUID"
	WebhookUUIDHeader = "X-Zadig-Webhook-UUID"

	TimeoutSeconds = 60
)

type WebHookNotifyEvent string

const (
	WebHookNotifyEventWorkflow    WebHookNotifyEvent = "workflow"
	WebHookNotifyEventReleasePlan WebHookNotifyEvent = "release_plan"
)

type WebHookNotifyObjectKind string

const (
	WebHookNotifyObjectKindWorkflow    WebHookNotifyObjectKind = "workflow"
	WebHookNotifyObjectKindReleasePlan WebHookNotifyObjectKind = "release_plan"
)

type WebHookNotify struct {
	ObjectKind  WebHookNotifyObjectKind `json:"object_kind"`
	Event       WebHookNotifyEvent      `json:"event"`
	Workflow    *WorkflowNotify         `json:"workflow"`
	ReleasePlan *ReleasePlanHookBody    `json:"release_plan"`
}

type WorkflowNotify struct {
	TaskID              int64                         `json:"task_id"`
	ProjectName         string                        `json:"project_name"`
	ProjectDisplayName  string                        `json:"project_display_name"`
	WorkflowName        string                        `json:"workflow_name"`
	WorkflowDisplayName string                        `json:"workflow_display_name"`
	Status              config.Status                 `json:"status"`
	Remark              string                        `json:"remark"`
	DetailURL           string                        `json:"detail_url"`
	Error               string                        `json:"error"`
	CreateTime          int64                         `json:"create_time"`
	StartTime           int64                         `json:"start_time"`
	EndTime             int64                         `json:"end_time"`
	Stages              []*WorkflowNotifyStage        `json:"stages"`
	TaskCreator         string                        `json:"task_creator"`
	TaskCreatorID       string                        `json:"task_creator_id"`
	TaskCreatorPhone    string                        `json:"task_creator_phone"`
	TaskCreatorEmail    string                        `json:"task_creator_email"`
	TaskType            config.CustomWorkflowTaskType `json:"task_type"`
}

type WorkflowNotifyStage struct {
	Name      string                   `json:"name"`
	Status    config.Status            `json:"status"`
	StartTime int64                    `json:"start_time"`
	EndTime   int64                    `json:"end_time"`
	Jobs      []*WorkflowNotifyJobTask `json:"jobs"`
	Error     string                   `json:"error"`
}

type WorkflowNotifyJobTask struct {
	Name        string        `json:"name"`
	DisplayName string        `json:"display_name"`
	JobType     string        `json:"type"`
	Status      config.Status `json:"status"`
	StartTime   int64         `json:"start_time"`
	EndTime     int64         `json:"end_time"`
	Error       string        `json:"error"`
	Spec        interface{}   `json:"spec"`
}

type WorkflowNotifyJobTaskBuildSpec struct {
	Repositories []*WorkflowNotifyRepository `json:"repositories"`
	Image        string                      `json:"image"`
}

type WorkflowNotifyJobTaskDeploySpec struct {
	Env            string                               `json:"env"`
	ServiceName    string                               `json:"service_name"`
	ServiceModules []*WorkflowNotifyDeployServiceModule `json:"service_modules"`
}

type WorkflowNotifyDeployServiceModule struct {
	ServiceModule string `json:"service_module"`
	Image         string `json:"image"`
}

type WorkflowNotifyRepository struct {
	Source        string `json:"source"`
	RepoOwner     string `json:"repo_owner"`
	RepoNamespace string `json:"repo_namespace"`
	RepoName      string `json:"repo_name"`
	Branch        string `json:"branch"`
	PRs           []int  `json:"prs"`
	Tag           string `json:"tag"`
	AuthorName    string `json:"author_name"`
	CommitID      string `json:"commit_id"`
	CommitURL     string `json:"commit_url"`
	CommitMessage string `json:"commit_message"`
}

type ReleasePlanHookBody struct {
	ID                  primitive.ObjectID `json:"id"`
	Index               int64              `json:"index"`
	Name                string             `json:"name"`
	Manager             string             `json:"manager"`
	ManagerID           string             `json:"manager_id"`
	StartTime           int64              `json:"start_time"`
	EndTime             int64              `json:"end_time"`
	ScheduleExecuteTime int64              `json:"schedule_execute_time"`
	Description         string             `json:"description"`
	CreatedBy           string             `json:"created_by"`
	CreateTime          int64              `json:"create_time"`
	UpdatedBy           string             `json:"updated_by"`
	UpdateTime          int64              `json:"update_time"`

	Jobs []*ReleasePlanHookJob `json:"jobs"`

	Status config.ReleasePlanStatus `json:"status"`

	PlanningTime  int64 `json:"planning_time"`
	ApprovalTime  int64 `json:"approval_time"`
	ExecutingTime int64 `json:"executing_time"`
	SuccessTime   int64 `json:"success_time"`
}

type ReleasePlanHookJob struct {
	ID   string                    `json:"id"`
	Name string                    `json:"name"`
	Type config.ReleasePlanJobType `json:"type"`
	Spec interface{}               `json:"spec"`

	ReleasePlanHookJobRuntime `json:",inline"`
}

type ReleasePlanHookJobRuntime struct {
	Status       config.ReleasePlanJobStatus `json:"status"`
	ExecutedBy   string                      `json:"executed_by"`
	ExecutedTime int64                       `json:"executed_time"`
}

type ReleasePlanHookTextJobSpec struct {
	Content string `json:"content"`
	Remark  string `json:"remark"`
}

type ReleasePlanHookWorkflowJobSpec struct {
	Workflow *ReleasePlanHookWorkflowV4 `json:"workflow"`
	Status   config.Status              `json:"status"`
	TaskID   int64                      `json:"task_id"`
}

type ReleasePlanHookWorkflowV4 struct {
	Name                 string                          `json:"name"`
	DisplayName          string                          `json:"display_name"`
	Disabled             bool                            `json:"disabled"`
	Category             setting.WorkflowCategory        `json:"category"`
	Params               []*ReleasePlanHookWorkflowParam `json:"params"`
	Stages               []*ReleasePlanHookWorkflowStage `json:"stages"`
	Project              string                          `json:"project"`
	Description          string                          `json:"description"`
	CreatedBy            string                          `json:"created_by"`
	CreateTime           int64                           `json:"create_time"`
	UpdatedBy            string                          `json:"updated_by"`
	UpdateTime           int64                           `json:"update_time"`
	Remark               string                          `json:"remark"`
	EnableApprovalTicket bool                            `json:"enable_approval_ticket"`
	ApprovalTicketID     string                          `json:"approval_ticket_id"`
}

type ReleasePlanHookWorkflowParam struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	// support string/text/choice/repo type
	ParamsType   string                 `json:"type"`
	Value        string                 `json:"value"`
	Repo         *types.Repository      `json:"repo"`
	ChoiceOption []string               `json:"choice_option"`
	ChoiceValue  []string               `json:"choice_value"`
	Default      string                 `json:"default"`
	IsCredential bool                   `json:"is_credential"`
	Source       config.ParamSourceType `json:"source"`
}

type ReleasePlanHookWorkflowStage struct {
	Name string                        `json:"name"`
	Jobs []*ReleasePlanHookWorkflowJob `json:"jobs"`
}

type ReleasePlanHookWorkflowJob struct {
	Name      string              `json:"name"`
	JobType   config.JobType      `json:"type"`
	Spec      interface{}         `json:"spec"`
	RunPolicy config.JobRunPolicy `json:"run_policy"`
}
