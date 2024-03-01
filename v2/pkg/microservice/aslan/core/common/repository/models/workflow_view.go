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

import "go.mongodb.org/mongo-driver/bson/primitive"

type WorkflowView struct {
	ID          primitive.ObjectID    `bson:"_id,omitempty"          json:"id,omitempty"`
	Name        string                `bson:"name"                   json:"name"`
	ProjectName string                `bson:"project_name"           json:"project_name"`
	Workflows   []*WorkflowViewDetail `bson:"workflows"              json:"workflows"`
	UpdateTime  int64                 `bson:"update_time"            json:"update_time"`
	UpdateBy    string                `bson:"update_by"              json:"update_by,omitempty"`
}

type WorkflowViewDetail struct {
	WorkflowName        string `bson:"workflow_name"                   json:"workflow_name"`
	WorkflowDisplayName string `bson:"-"                               json:"workflow_display_name"`
	WorkflowType        string `bson:"workflow_type"                   json:"workflow_type"`
	Enabled             bool   `bson:"enabled"                         json:"enabled"`
}

func (WorkflowView) TableName() string {
	return "workflow_view"
}
