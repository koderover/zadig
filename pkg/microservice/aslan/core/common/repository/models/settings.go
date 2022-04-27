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

type SystemSetting struct {
	ID                  primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	WorkflowConcurrency int64              `bson:"workflow_concurrency" json:"workflow_concurrency"`
	BuildConcurrency    int64              `bson:"build_concurrency" json:"build_concurrency"`
	DefaultLogin        string             `bson:"default_login" json:"default_login"`
	UpdateTime          int64              `bson:"update_time" json:"update_time"`
}

func (SystemSetting) TableName() string {
	return "system_setting"
}
