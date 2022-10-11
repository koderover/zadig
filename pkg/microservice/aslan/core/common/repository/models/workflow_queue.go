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
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
)

type WorkflowQueue struct {
	ID                  primitive.ObjectID `bson:"_id,omitempty"                              json:"id,omitempty"`
	TaskID              int64              `bson:"task_id"                                    json:"task_id"`
	ProjectName         string             `bson:"project_name"                               json:"project_name"`
	WorkflowName        string             `bson:"workflow_name"                              json:"workflow_name"`
	WorkflowDisplayName string             `bson:"workflow_display_name"                      json:"workflow_display_name"`
	Status              config.Status      `bson:"status"                                     json:"status,omitempty"`
	Stages              []*StageTask       `bson:"stages"                                     json:"stages"`
	TaskCreator         string             `bson:"task_creator"                               json:"task_creator,omitempty"`
	TaskRevoker         string             `bson:"task_revoker,omitempty"                     json:"task_revoker,omitempty"`
	CreateTime          int64              `bson:"create_time"                                json:"create_time,omitempty"`
	MultiRun            bool               `bson:"multi_run"                                  json:"multi_run"`
}

func (WorkflowQueue) TableName() string {
	return "workflow_queue"
}
