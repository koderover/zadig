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

package models

import "go.mongodb.org/mongo-driver/bson/primitive"

type CallbackRequest struct {
	ID   primitive.ObjectID `bson:"_id,omitempty"  json:"id,omitempty"`
	Type string             `bson:"type"           json:"type"`
	// this is called pipeline name because it is called so all around this system
	PipelineName  string                 `bson:"task_name"      json:"task_name"`
	ProjectName   string                 `bson:"project_name"   json:"project_name"`
	TaskID        int64                  `bson:"task_id"        json:"task_id"`
	Status        string                 `bson:"status"         json:"status"`
	StatusMessage string                 `bson:"status_message" json:"status_message"`
	Payload       map[string]interface{} `bson:"payload"        json:"payload"`
}

func (CallbackRequest) TableName() string {
	return "callback_request"
}
