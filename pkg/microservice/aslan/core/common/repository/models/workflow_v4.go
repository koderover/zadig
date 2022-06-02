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
	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/setting"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type WorkflowV4 struct {
	ID          primitive.ObjectID `bson:"_id,omitempty"  json:"id,omitempty"`
	Name        string             `bson:"name"           json:"name"`
	Args        []*KeyVal          `bson:"args"           json:"args"`
	Stages      []*WorkflowStage   `bson:"stages"         json:"stages"`
	ProjectName string             `bson:"project_name"   json:"project_name"`
	Description string             `bson:"description"    json:"description"`
	CreatedBy   string             `bson:"created_by"     json:"created_by"`
	CreateTime  int64              `bson:"create_time"    json:"create_time"`
	UpdatedBy   string             `bson:"updated_by"     json:"updated_by"`
	UpdateTime  int64              `bson:"update_time"    json:"update_time"`
}

type WorkflowStage struct {
	Name string `bson:"name"          json:"name"`
	// default is custom defined by user.
	StageType string `bson:"type"           json:"type"`
	Jobs      []*Job `bson:"jobs"           json:"jobs"`
}

type Job struct {
	Name       string        `bson:"name"           json:"name"`
	Properties JobProperties `bson:"properties"     json:"properties"`
	JobType    string        `bson:"type"           json:"type"`
	Spec       interface{}   `bson:"spec"           json:"spec"`
	Steps      []*Step       `bson:"steps"          json:"steps"`
	Outputs    []*Output     `bson:"outputs"        json:"outputs"`
}

type JobProperties struct {
	Timeout         int64           `bson:"timeout"                json:"timeout"`
	Retry           int64           `bson:"retry"                  json:"retry"`
	ResourceRequest setting.Request `bson:"resource_request"       json:"resource_request"`
	ClusterID       string          `bson:"cluster_id"             json:"cluster_id"`
	BuildOS         string          `bson:"build_os"               json:"build_os"`
	Namespace       string          `bson:"namespace"              json:"namespace"`
	Args            []*KeyVal       `bson:"args"                   json:"args"`
	DockerHost      string          `bson:"-"                      json:"docker_host,omitempty"`
}

type Step struct {
	Name     string          `bson:"name"           json:"name"`
	Timeout  int64           `bson:"timeout"        json:"timeout"`
	StepType config.StepType `bson:"step_type"      json:"step_type"`
	Spec     interface{}     `bson:"spec"           json:"spec"`
}

type Output struct {
	Name        string `bson:"name"           json:"name"`
	Description string `bson:"description"    json:"description"`
}

func (WorkflowV4) TableName() string {
	return "workflow_v4"
}
