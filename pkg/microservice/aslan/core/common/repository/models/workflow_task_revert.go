/*
 * Copyright 2025 The KodeRover Authors.
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
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type WorkflowTaskRevert struct {
	ID            primitive.ObjectID `bson:"_id,omitempty"             json:"id,omitempty"`
	TaskID        int64              `bson:"task_id"                   json:"task_id"`
	WorkflowName  string             `bson:"workflow_name"             json:"workflow_name"`
	JobName       string             `bson:"job_name"                  json:"job_name"`
	RevertSpec    interface{}        `bson:"revert_spec"               json:"revert_spec"`
	CreateTime    int64              `bson:"create_time"               json:"create_time,omitempty"`
	TaskCreator   string             `bson:"creator"                   json:"creator,omitempty"`
	TaskCreatorID string             `bson:"creator_id"                json:"creator_id,omitempty"`
	Status        config.Status      `bson:"status"                    json:"status,omitempty"`
}

func (WorkflowTaskRevert) TableName() string {
	return "workflow_task_revert"
}
