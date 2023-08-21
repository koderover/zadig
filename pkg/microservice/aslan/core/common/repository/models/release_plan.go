/*
 * Copyright 2023 The KodeRover Authors.
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
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
)

type ReleasePlan struct {
	ID      primitive.ObjectID `bson:"_id,omitempty"       yaml:"-"                   json:"id"`
	Index   int64              `bson:"index"       yaml:"index"                   json:"index"`
	Name    string             `bson:"name"       yaml:"name"                   json:"name"`
	Manager string             `bson:"manager"       yaml:"manager"                   json:"manager"`
	// ManagerID is the user id of the manager
	ManagerID   string `bson:"manager_id"       yaml:"manager_id"                   json:"manager_id"`
	StartTime   int64  `bson:"start_time"       yaml:"start_time"                   json:"start_time"`
	EndTime     int64  `bson:"end_time"       yaml:"end_time"                   json:"end_time"`
	Description string `bson:"description"       yaml:"description"                   json:"description"`
	CreatedBy   string `bson:"created_by"       yaml:"created_by"                   json:"created_by"`
	CreateTime  int64  `bson:"create_time"       yaml:"create_time"                   json:"create_time"`
	UpdatedBy   string `bson:"updated_by"       yaml:"updated_by"                   json:"updated_by"`
	UpdateTime  int64  `bson:"update_time"       yaml:"update_time"                   json:"update_time"`

	Approval *Approval `bson:"approval"       yaml:"approval"                   json:"approval"`

	Jobs []*ReleaseJob `bson:"jobs"       yaml:"jobs"                   json:"jobs"`

	Logs   []*ReleasePlanLog        `bson:"logs"       yaml:"logs"                   json:"logs"`
	Status config.ReleasePlanStatus `bson:"status"       yaml:"status"                   json:"status"`
}

func (ReleasePlan) TableName() string {
	return "release_plan"
}

type ReleaseJob struct {
	ID   string      `bson:"id"       yaml:"id"                   json:"id"`
	Name string      `bson:"name"       yaml:"name"                   json:"name"`
	Type string      `bson:"type"       yaml:"type"                   json:"type"`
	Spec interface{} `bson:"spec"       yaml:"spec"                   json:"spec"`

	ReleaseJobRuntime `bson:",inline" yaml:",inline" json:",inline"`
}

type ReleaseJobRuntime struct {
	Status config.Status `bson:"status"     yaml:"status"                 json:"status"`
	// ReleasePlan can return to PlanningStatus when some release jobs have been executed
	// So we need to record the last status of the release job
	LastStatus config.Status `bson:"last_status"       yaml:"last_status"                   json:"last_status"`
	// Updated is used to indicate whether the release job has been updated
	Updated      bool   `bson:"updated"       yaml:"updated"                   json:"updated"`
	ExecutorBy   string `bson:"executor_by"       yaml:"executor_by"                   json:"executor_by"`
	ExecutorTime int64  `bson:"executor_time"       yaml:"executor_time"                   json:"executor_time"`
}

type TextReleaseJobSpec struct {
	Content       string `bson:"content"       yaml:"content"                   json:"content"`
	ReleaseRemark string `bson:"remark"       yaml:"remark"                   json:"remark"`
}

type WorkflowReleaseJobSpec struct {
	//ProjectName  string      `bson:"project_name"       yaml:"project_name"                   json:"project_name"`
	//WorkflowName string      `bson:"workflow_name"       yaml:"workflow_name"                   json:"workflow_name"`
	Workflow *WorkflowV4 `bson:"workflow"       yaml:"workflow"                   json:"workflow"`
}

type ReleasePlanLog struct {
	Username   string      `bson:"username"                    json:"username"`
	Action     string      `bson:"action"                      json:"action"`
	Target     string      `bson:"target"                      json:"target"`
	TargetType string      `bson:"target_type"                 json:"target_type"`
	Before     interface{} `bson:"before"                      json:"before"`
	After      interface{} `bson:"after"                       json:"after"`
	CreatedAt  int64       `bson:"created_at"                  json:"created_at"`
}
