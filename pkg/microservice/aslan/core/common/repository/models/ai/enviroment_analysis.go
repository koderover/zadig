/*
Copyright 2023 The KodeRover Authors.

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

package ai

import "go.mongodb.org/mongo-driver/bson/primitive"

type EnvAIAnalysis struct {
	ID          primitive.ObjectID `bson:"_id,omitempty"                json:"id,omitempty"`
	ProjectName string             `bson:"project_name"                 json:"project_name"`
	DeployType  string             `bson:"deploy_type"                  json:"deploy_type"`
	EnvName     string             `bson:"env_name"                     json:"env_name"`
	Production  bool               `bson:"production"                   json:"production"`
	Status      string             `bson:"status"                       json:"status"`
	Result      string             `bson:"result"                       json:"result"`
	Err         string             `bson:"err"                          json:"err"`
	StartTime   int64              `bson:"start_time"                   json:"start_time"`
	EndTime     int64              `bson:"end_time"                     json:"end_time"`
	Updated     int64              `bson:"updated"                      json:"updated"`
	TriggerName string             `bson:"trigger_name"                 json:"trigger_name"`
	CreatedBy   string             `bson:"created_by"                   json:"created_by"`
}

func (EnvAIAnalysis) TableName() string {
	return "env_ai_analysis"
}
