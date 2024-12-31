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
	WebHookNotifyEventWorkflow WebHookNotifyEvent = "workflow"
)

type WebHookNotifyObjectKind string

const (
	WebHookNotifyObjectKindWorkflow WebHookNotifyObjectKind = "workflow"
)

type WebHookNotify struct {
	ObjectKind WebHookNotifyObjectKind `json:"object_kind"`
	Event      WebHookNotifyEvent      `json:"event"`
	Workflow   *WorkflowNotify         `json:"workflow"`
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
	CommitID      string `json:"commit_id"`
	CommitURL     string `json:"commit_url"`
	CommitMessage string `json:"commit_message"`
}
