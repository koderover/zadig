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

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
)

type WorkflowTask struct {
	ID           primitive.ObjectID  `bson:"_id,omitempty"             json:"id,omitempty"`
	TaskID       int64               `bson:"task_id"                   json:"task_id"`
	WorkflowName string              `bson:"workflow_name"             json:"workflow_name"`
	Type         config.PipelineType `bson:"type"                      json:"type"`
	Status       config.Status       `bson:"status"                    json:"status,omitempty"`
	Description  string              `bson:"description,omitempty"     json:"description,omitempty"`
	TaskCreator  string              `bson:"task_creator"              json:"task_creator,omitempty"`
	TaskRevoker  string              `bson:"task_revoker,omitempty"    json:"task_revoker,omitempty"`
	CreateTime   int64               `bson:"create_time"               json:"create_time,omitempty"`
	StartTime    int64               `bson:"start_time"                json:"start_time,omitempty"`
	EndTime      int64               `bson:"end_time"                  json:"end_time,omitempty"`
	Stages       []*StageTask        `bson:"stages"                    json:"stages"`
	ReqID        string              `bson:"req_id,omitempty"          json:"req_id,omitempty"`
	DockerHost   string              `bson:"-"                         json:"docker_host,omitempty"`
	TeamName     string              `bson:"team,omitempty"            json:"team,omitempty"`
	IsDeleted    bool                `bson:"is_deleted"                json:"is_deleted"`
	IsArchived   bool                `bson:"is_archived"               json:"is_archived"`
	Error        string              `bson:"error,omitempty"           json:"error,omitempty"`
	IsRestart    bool                `bson:"is_restart"                json:"is_restart"`
}

func (WorkflowTask) TableName() string {
	return "workflow_task"
}

type StageTask struct {
	Name      string        `bson:"name"          json:"name"`
	Status    config.Status `bson:"status"        json:"status"`
	StartTime int64         `bson:"start_time"    json:"start_time,omitempty"`
	EndTime   int64         `bson:"end_time"      json:"end_time,omitempty"`
	// default is custom defined by user.
	StageType string      `bson:"type"           json:"type"`
	Parallel  bool        `bson:"parallel"       json:"parallel"`
	Spec      interface{} `bson:"spec"           json:"spec"`
	Jobs      []*JobTask  `bson:"jobs"           json:"jobs"`
}

type JobTask struct {
	Name       string         `bson:"name"           json:"name"`
	JobType    string         `bson:"type"           json:"type"`
	Status     config.Status  `bson:"status"         json:"status"`
	StartTime  int64          `bson:"start_time"     json:"start_time,omitempty"`
	EndTime    int64          `bson:"end_time"       json:"end_time,omitempty"`
	Error      string         `bson:"error"          json:"error"`
	Namepace   string         `bson:"namespace"      json:"namespace"`
	PodName    string         `bson:"pod_name"       json:"pod_name"`
	Properties *JobProperties `bson:"properties"     json:"properties"`
	Steps      []*StepTask    `bson:"steps"          json:"steps"`
}

type StepTask struct {
	Name     string          `bson:"name"           json:"name"`
	JobName  string          `bson:"job_name"       json:"job_name"`
	Error    string          `bson:"error"          json:"error"`
	StepType config.StepType `bson:"step_type"      json:"step_type"`
	// step input params,differ form steps
	Spec interface{} `bson:"Spec"           json:"Spec"`
	// step output results,like testing results,differ form steps
	Result interface{} `bson:"result"         json:"result"`
}

type WorkflowTaskCtx struct {
	WorkflowName      string
	TaskID            int64
	DockerHost        string
	Workspace         string
	DistDir           string
	DockerMountDir    string
	ConfigMapMountDir string
}
