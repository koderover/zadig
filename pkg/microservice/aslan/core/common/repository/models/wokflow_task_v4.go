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

type WorkflowTask struct {
	ID            primitive.ObjectID `bson:"_id,omitempty"             json:"id,omitempty"`
	TaskID        int64              `bson:"task_id"                   json:"task_id"`
	WorkflowName  string             `bson:"workflow_name"             json:"workflow_name"`
	WorkflowArgs  *WorkflowV4        `bson:"workflow_args"             json:"workflow_args"`
	KeyVals       []*KeyVal          `bson:"key_vals"                  json:"key_vals"`
	GlobalContext map[string]string  `bson:"global_context"            json:"global_context"`
	Status        config.Status      `bson:"status"                    json:"status,omitempty"`
	TaskCreator   string             `bson:"task_creator"              json:"task_creator,omitempty"`
	TaskRevoker   string             `bson:"task_revoker,omitempty"    json:"task_revoker,omitempty"`
	CreateTime    int64              `bson:"create_time"               json:"create_time,omitempty"`
	StartTime     int64              `bson:"start_time"                json:"start_time,omitempty"`
	EndTime       int64              `bson:"end_time"                  json:"end_time,omitempty"`
	Stages        []*StageTask       `bson:"stages"                    json:"stages"`
	ProjectName   string             `bson:"project_name,omitempty"    json:"project_name,omitempty"`
	IsDeleted     bool               `bson:"is_deleted"                json:"is_deleted"`
	IsArchived    bool               `bson:"is_archived"               json:"is_archived"`
	Error         string             `bson:"error,omitempty"           json:"error,omitempty"`
	IsRestart     bool               `bson:"is_restart"                json:"is_restart"`
	MultiRun      bool               `bson:"multi_run"                 json:"multi_run"`
}

func (WorkflowTask) TableName() string {
	return "workflow_task"
}

type StageTask struct {
	Name      string        `bson:"name"          json:"name"`
	Status    config.Status `bson:"status"        json:"status"`
	StartTime int64         `bson:"start_time"    json:"start_time,omitempty"`
	EndTime   int64         `bson:"end_time"      json:"end_time,omitempty"`
	Parallel  bool          `bson:"parallel"      json:"parallel"`
	Approval  *Approval     `bson:"approval"      json:"approval"`
	Jobs      []*JobTask    `bson:"jobs"          json:"jobs"`
	Error     string        `bson:"error"         json:"error"`
}

type JobTask struct {
	Name       string        `bson:"name"           json:"name"`
	JobType    string        `bson:"type"           json:"type"`
	Status     config.Status `bson:"status"         json:"status"`
	StartTime  int64         `bson:"start_time"     json:"start_time,omitempty"`
	EndTime    int64         `bson:"end_time"       json:"end_time,omitempty"`
	Error      string        `bson:"error"          json:"error"`
	Properties JobProperties `bson:"properties"     json:"properties"`
	Steps      []*StepTask   `bson:"steps"          json:"steps"`
	Outputs    []*Output     `bson:"outputs"        json:"outputs"`
}

type StepTask struct {
	Name     string          `bson:"name"           json:"name"      yaml:"name"`
	JobName  string          `bson:"job_name"       json:"job_name"  yaml:"job_name"`
	Error    string          `bson:"error"          json:"error"     yaml:"error"`
	StepType config.StepType `bson:"type"           json:"type"      yaml:"type"`
	// step input params,differ form steps
	Spec interface{} `bson:"spec"           json:"spec"   yaml:"spec"`
	// step output results,like testing results,differ form steps
	Result interface{} `bson:"result"         json:"result"  yaml:"result"`
}

type WorkflowTaskCtx struct {
	WorkflowName      string
	ProjectName       string
	TaskID            int64
	DockerHost        string
	Workspace         string
	DistDir           string
	DockerMountDir    string
	ConfigMapMountDir string
	WorkflowKeyVals   []*KeyVal
	GlobalContextGet  func(key string) (string, bool)
	GlobalContextSet  func(key, value string)
	GlobalContextEach func(f func(k, v string) bool)
}
