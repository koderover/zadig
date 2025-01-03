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

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Cronjob struct {
	ID              primitive.ObjectID `bson:"_id,omitempty"                       json:"id"`
	Name            string             `bson:"name"                                json:"name"`
	Type            string             `bson:"type"                                json:"type"`
	Number          uint64             `bson:"number"                              json:"number"`
	UnixStamp       int64              `bson:"unix_stamp"                          json:"unix_stamp"`
	Frequency       string             `bson:"frequency"                           json:"frequency"`
	Time            string             `bson:"time"                                json:"time"`
	Cron            string             `bson:"cron"                                json:"cron"`
	ProductName     string             `bson:"product_name,omitempty"              json:"product_name,omitempty"`
	MaxFailure      int                `bson:"max_failures,omitempty"              json:"max_failures,omitempty"`
	TaskArgs        *TaskArgs          `bson:"task_args,omitempty"                 json:"task_args,omitempty"`
	WorkflowArgs    *WorkflowTaskArgs  `bson:"workflow_args,omitempty"             json:"workflow_args,omitempty"`
	WorkflowV4Args  *WorkflowV4        `bson:"workflow_v4_args"                    json:"workflow_v4_args"`
	TestArgs        *TestTaskArgs      `bson:"test_args,omitempty"                 json:"test_args,omitempty"`
	EnvAnalysisArgs *EnvArgs           `bson:"env_analysis_args,omitempty"         json:"env_analysis_args,omitempty"`
	EnvArgs         *EnvArgs           `bson:"env_args,omitempty"                  json:"env_args,omitempty"`
	ReleasePlanArgs *ReleasePlanArgs   `bson:"release_plan_args,omitempty"         json:"release_plan_args,omitempty"`
	JobType         string             `bson:"job_type"                            json:"job_type"`
	Enabled         bool               `bson:"enabled"                             json:"enabled"`
}

type EnvArgs struct {
	Name        string `bson:"name"                    json:"name"`
	ProductName string `bson:"product_name"            json:"product_name"`
	EnvName     string `bson:"env_name"                json:"env_name"`
	Production  bool   `bson:"production"              json:"production"`
}

type ReleasePlanArgs struct {
	ID    string `bson:"id"             json:"id"`
	Name  string `bson:"name"           json:"name"`
	Index int64  `bson:"index"          json:"index"`
}

func (Cronjob) TableName() string {
	return "cronjob"
}
