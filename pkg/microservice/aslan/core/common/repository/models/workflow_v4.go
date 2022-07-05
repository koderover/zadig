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
	"fmt"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"gopkg.in/yaml.v3"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/types"
)

type WorkflowV4 struct {
	ID          primitive.ObjectID `bson:"_id,omitempty"  yaml:"-"            json:"id"`
	Name        string             `bson:"name"           yaml:"name"         json:"name"`
	KeyVals     []*KeyVal          `bson:"key_vals"       yaml:"key_vals"     json:"key_vals"`
	Stages      []*WorkflowStage   `bson:"stages"         yaml:"stages"       json:"stages"`
	Project     string             `bson:"project"        yaml:"project"      json:"project"`
	Description string             `bson:"description"    yaml:"description"  json:"description"`
	CreatedBy   string             `bson:"created_by"     yaml:"created_by"   json:"created_by"`
	CreateTime  int64              `bson:"create_time"    yaml:"create_time"  json:"create_time"`
	UpdatedBy   string             `bson:"updated_by"     yaml:"updated_by"   json:"updated_by"`
	UpdateTime  int64              `bson:"update_time"    yaml:"update_time"  json:"update_time"`
	MultiRun    bool               `bson:"multi_run"      yaml:"multi_run"    json:"multi_run"`
}

type WorkflowStage struct {
	Name     string    `bson:"name"          yaml:"name"         json:"name"`
	Parallel bool      `bson:"parallel"      yaml:"parallel"     json:"parallel"`
	Approval *Approval `bson:"approval"      yaml:"approval"     json:"approval"`
	Jobs     []*Job    `bson:"jobs"          yaml:"jobs"         json:"jobs"`
}

type Approval struct {
	Enabled         bool                   `bson:"enabled"                     yaml:"enabled"                    json:"enabled"`
	ApproveUsers    []*User                `bson:"approve_users"               yaml:"approve_users"              json:"approve_users"`
	Timeout         int                    `bson:"timeout"                     yaml:"timeout"                    json:"timeout"`
	NeededApprovers int                    `bson:"needed_approvers"            yaml:"needed_approvers"           json:"needed_approvers"`
	Description     string                 `bson:"description"                 yaml:"description"                json:"description"`
	RejectOrApprove config.ApproveOrReject `bson:"reject_or_approve"           yaml:"-"                          json:"reject_or_approve"`
}

type User struct {
	UserID          string                 `bson:"user_id"                     yaml:"user_id"                    json:"user_id"`
	UserName        string                 `bson:"user_name"                   yaml:"user_name"                  json:"user_name"`
	RejectOrApprove config.ApproveOrReject `bson:"reject_or_approve"           yaml:"-"                          json:"reject_or_approve"`
	Comment         string                 `bson:"comment"                     yaml:"-"                          json:"comment"`
	OperationTime   int64                  `bson:"operation_time"              yaml:"-"                          json:"operation_time"`
}

type Job struct {
	Name    string         `bson:"name"           yaml:"name"     json:"name"`
	JobType config.JobType `bson:"type"           yaml:"type"     json:"type"`
	Spec    interface{}    `bson:"spec"           yaml:"spec"     json:"spec"`
}

type FreestyleJobSpec struct {
	Properties *JobProperties `bson:"properties"     yaml:"properties"    json:"properties"`
	Steps      []*Step        `bson:"steps"          yaml:"steps"         json:"steps"`
	Outputs    []*Output      `bson:"outputs"        yaml:"outputs"       json:"outputs"`
}

type ZadigBuildJobSpec struct {
	DockerRegistryID string             `bson:"docker_registry_id"     yaml:"docker_registry_id"     json:"docker_registry_id"`
	ServiceAndBuilds []*ServiceAndBuild `bson:"service_and_builds"     yaml:"service_and_builds"     json:"service_and_builds"`
}

type ServiceAndBuild struct {
	ServiceName   string              `bson:"service_name"        yaml:"service_name"     json:"service_name"`
	ServiceModule string              `bson:"service_module"      yaml:"service_module"   json:"service_module"`
	BuildName     string              `bson:"build_name"          yaml:"build_name"       json:"build_name"`
	Image         string              `bson:"-"                   yaml:"-"                json:"image"`
	Package       string              `bson:"-"                   yaml:"-"                json:"package"`
	KeyVals       []*KeyVal           `bson:"key_vals"            yaml:"key_vals"         json:"key_vals"`
	Repos         []*types.Repository `bson:"-"                   yaml:"-"                json:"repos"`
}

type ZadigDeployJobSpec struct {
	Env        string `bson:"env"       yaml:"env"     json:"env"`
	DeployType string `bson:"-"         yaml:"-"       json:"deploy_type"`
	// fromjob/runtime, runtime 表示运行时输入，fromjob 表示从上游构建任务中获取
	Source config.DeploySourceType `bson:"source"     yaml:"source"     json:"source"`
	// 当 source 为 fromjob 时需要，指定部署镜像来源是上游哪一个构建任务
	JobName          string             `bson:"job_name"             yaml:"job_name"        json:"job_name"`
	ServiceAndImages []*ServiceAndImage `bson:"-"                    yaml:"-"               json:"service_and_images"`
}

type ServiceAndImage struct {
	ServiceName   string `bson:"service_name"        yaml:"service_name"     json:"service_name"`
	ServiceModule string `bson:"service_module"      yaml:"service_module"   json:"service_module"`
	Image         string `bson:"image"               yaml:"image"           json:"image"`
}

type JobProperties struct {
	Timeout         int64           `bson:"timeout"                json:"timeout"`
	Retry           int64           `bson:"retry"                  json:"retry"`
	ResourceRequest setting.Request `bson:"resource_request"       json:"resource_request"`
	ClusterID       string          `bson:"cluster_id"             json:"cluster_id"`
	BuildOS         string          `bson:"build_os"               json:"build_os"`
	ImageFrom       string          `bson:"image_from"             json:"image_from"`
	Namespace       string          `bson:"namespace"              json:"namespace"`
	Args            []*KeyVal       `bson:"args"                   json:"args"`
	Paths           string          `bson:"-"                      json:"-"`
	LogFileName     string          `bson:"log_file_name"          json:"log_file_name"`
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

func IToiYaml(before interface{}, after interface{}) error {
	b, err := yaml.Marshal(before)
	if err != nil {
		return fmt.Errorf("marshal task error: %v", err)
	}

	if err := yaml.Unmarshal(b, after); err != nil {
		return fmt.Errorf("unmarshal task error: %v", err)
	}
	return nil
}

func (WorkflowV4) TableName() string {
	return "workflow_v4"
}
