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

package models

import (
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/types"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type SprintWorkItemTask struct {
	ID                 primitive.ObjectID       `bson:"_id,omitempty"        yaml:"-"                    json:"id"`
	SprintWorkItemIDs  []string                 `bson:"sprint_workitem_ids"  yaml:"sprint_workitem_ids"  json:"sprint_workitem_ids"`
	WorkflowName       string                   `bson:"workflow_name"        yaml:"workflow_name"        json:"workflow_name"`
	WorkflowTaskID     int64                    `bson:"workflow_task_id"     yaml:"workflow_task_id"     json:"workflow_task_id"`
	Status             config.Status            `bson:"status"               yaml:"status"               json:"status"`
	ServiceModuleDatas []*WorkflowServiceModule `bson:"service_module_datas" yaml:"service_module_datas" json:"service_module_datas"`
	Creator            types.UserBriefInfo      `bson:"creator"              yaml:"creator"              json:"creator"`
	CreateTime         int64                    `bson:"create_time"          yaml:"create_time"          json:"create_time"`
	StartTime          int64                    `bson:"start_time"           yaml:"start_time"           json:"start_time"`
	EndTime            int64                    `bson:"end_time"             yaml:"end_time"             json:"end_time"`
}

func (SprintWorkItemTask) TableName() string {
	return "sprint_workitem_task"
}
