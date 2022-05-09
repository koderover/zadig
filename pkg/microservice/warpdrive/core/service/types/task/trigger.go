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

package task

import (
	"fmt"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/warpdrive/config"
)

type Trigger struct {
	TaskType        config.TaskType  `bson:"type"                       json:"type"`
	Enabled         bool             `bson:"enabled"                    json:"enabled"`
	TaskStatus      config.Status    `bson:"status"                     json:"status"`
	URL             string           `bson:"url,omitempty"              json:"url,omitempty"`
	Path            string           `bson:"path,omitempty"             json:"path,omitempty"`
	IsCallback      bool             `bson:"is_callback,omitempty"      json:"is_callback,omitempty"`
	CallbackType    string           `bson:"callback_type,omitempty"    json:"callback_type,omitempty"`
	CallbackPayload *CallbackPayload `bson:"callback_payload,omitempty" json:"callback_payload,omitempty"`
	Headers         []*models.KeyVal `bson:"headers,omitempty"          json:"headers,omitempty"`
	Timeout         int              `bson:"timeout"                    json:"timeout,omitempty"`
	IsRestart       bool             `bson:"is_restart"                 json:"is_restart"`
	Error           string           `bson:"error,omitempty"            json:"error,omitempty"`
	StartTime       int64            `bson:"start_time"                 json:"start_time,omitempty"`
	EndTime         int64            `bson:"end_time"                   json:"end_time,omitempty"`
}

type CallbackPayload struct {
	QRCodeURL string `bson:"QR_code_URL,omitempty" json:"QR_code_URL,omitempty"`
}

// ToSubTask ...
func (t *Trigger) ToSubTask() (map[string]interface{}, error) {
	var task map[string]interface{}
	if err := IToi(t, &task); err != nil {
		return nil, fmt.Errorf("convert trigger to interface error: %s", err)
	}
	return task, nil
}

type WebhookPayload struct {
	EventName      string         `json:"event_name"`
	ProjectName    string         `json:"project_name"`
	TaskName       string         `json:"task_name"`
	TaskID         int64          `json:"task_id"`
	TaskOutput     []*TaskOutput  `json:"task_output,omitempty"`
	TaskEnvs       []*KeyVal      `json:"task_envs,omitempty"`
	ServiceInfos   []*ServiceInfo `json:"service_infos"`
	Creator        string         `json:"creator"`
	WorkflowStatus config.Status  `json:"workflow_status"`
}

type TaskOutput struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type CallbackPayloadObj struct {
	TaskName      string           `json:"task_name"`
	ProjectName   string           `json:"project_name"`
	TaskID        int              `json:"task_id"`
	Type          string           `json:"type"`
	Status        string           `json:"status"`
	StatusMessage string           `json:"status_message"`
	Payload       *CallbackPayload `json:"payload"`
}
