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
	ID                 primitive.ObjectID `bson:"_id,omitempty"             json:"id,omitempty"`
	TaskID             int64              `bson:"task_id"                   json:"task_id"`
	WorkflowName       string             `bson:"workflow_name"             json:"workflow_name"`
	Params             []*Param           `bson:"params"                    json:"params"`
	WorkflowArgs       *WorkflowV4        `bson:"workflow_args"             json:"workflow_args"`
	OriginWorkflowArgs *WorkflowV4        `bson:"origin_workflow_args"      json:"origin_workflow_args"`
	KeyVals            []*KeyVal          `bson:"key_vals"                  json:"key_vals"`
	GlobalContext      map[string]string  `bson:"global_context"            json:"global_context"`
	Status             config.Status      `bson:"status"                    json:"status,omitempty"`
	TaskCreator        string             `bson:"task_creator"              json:"task_creator,omitempty"`
	TaskRevoker        string             `bson:"task_revoker,omitempty"    json:"task_revoker,omitempty"`
	CreateTime         int64              `bson:"create_time"               json:"create_time,omitempty"`
	StartTime          int64              `bson:"start_time"                json:"start_time,omitempty"`
	EndTime            int64              `bson:"end_time"                  json:"end_time,omitempty"`
	Stages             []*StageTask       `bson:"stages"                    json:"stages"`
	ProjectName        string             `bson:"project_name,omitempty"    json:"project_name,omitempty"`
	IsDeleted          bool               `bson:"is_deleted"                json:"is_deleted"`
	IsArchived         bool               `bson:"is_archived"               json:"is_archived"`
	Error              string             `bson:"error,omitempty"           json:"error,omitempty"`
	IsRestart          bool               `bson:"is_restart"                json:"is_restart"`
	MultiRun           bool               `bson:"multi_run"                 json:"multi_run"`
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
	Name      string        `bson:"name"                json:"name"`
	JobType   string        `bson:"type"                json:"type"`
	Status    config.Status `bson:"status"              json:"status"`
	StartTime int64         `bson:"start_time"          json:"start_time,omitempty"`
	EndTime   int64         `bson:"end_time"            json:"end_time,omitempty"`
	Error     string        `bson:"error"               json:"error"`
	Timeout   int64         `bson:"timeout"             json:"timeout"`
	Retry     int64         `bson:"retry"               json:"retry"`
	Spec      interface{}   `bson:"spec"                json:"spec"`
	Outputs   []*Output     `bson:"outputs"             json:"outputs"`
}

type JobTaskCustomDeploySpec struct {
	Namespace          string     `bson:"namespace"              json:"namespace"             yaml:"namespace"`
	ClusterID          string     `bson:"cluster_id"             json:"cluster_id"            yaml:"cluster_id"`
	Timeout            int64      `bson:"timeout"                json:"timeout"               yaml:"timeout"`
	WorkloadType       string     `bson:"workload_type"          json:"workload_type"         yaml:"workload_type"`
	WorkloadName       string     `bson:"workload_name"          json:"workload_name"         yaml:"workload_name"`
	ContainerName      string     `bson:"container_name"         json:"container_name"        yaml:"container_name"`
	Image              string     `bson:"image"                  json:"image"                 yaml:"image"`
	SkipCheckRunStatus bool       `bson:"skip_check_run_status"  json:"skip_check_run_status" yaml:"skip_check_run_status"`
	ReplaceResources   []Resource `bson:"replace_resources"      json:"replace_resources"     yaml:"replace_resources"`
}

type JobTaskDeploySpec struct {
	Env                string     `bson:"env"                              json:"env"                                 yaml:"env"`
	ServiceName        string     `bson:"service_name"                     json:"service_name"                        yaml:"service_name"`
	ServiceType        string     `bson:"service_type"                     json:"service_type"                        yaml:"service_type"`
	ServiceModule      string     `bson:"service_module"                   json:"service_module"                      yaml:"service_module"`
	SkipCheckRunStatus bool       `bson:"skip_check_run_status"            json:"skip_check_run_status"               yaml:"skip_check_run_status"`
	Image              string     `bson:"image"                            json:"image"                               yaml:"image"`
	ClusterID          string     `bson:"cluster_id"                       json:"cluster_id"                          yaml:"cluster_id"`
	Timeout            int        `bson:"timeout"                          json:"timeout"                             yaml:"timeout"`
	ReplaceResources   []Resource `bson:"replace_resources"                json:"replace_resources"                   yaml:"replace_resources"`
}

type Resource struct {
	Name      string `bson:"name"                              json:"name"                                 yaml:"name"`
	Kind      string `bson:"kind"                              json:"kind"                                 yaml:"kind"`
	Container string `bson:"container"                         json:"container"                            yaml:"container"`
	Origin    string `bson:"origin"                            json:"origin"                               yaml:"origin"`
}

type JobTaskHelmDeploySpec struct {
	Env                string                   `bson:"env"                              json:"env"                                 yaml:"env"`
	ServiceName        string                   `bson:"service_name"                     json:"service_name"                        yaml:"service_name"`
	ServiceType        string                   `bson:"service_type"                     json:"service_type"                        yaml:"service_type"`
	SkipCheckRunStatus bool                     `bson:"skip_check_run_status"            json:"skip_check_run_status"               yaml:"skip_check_run_status"`
	ImageAndModules    []*ImageAndServiceModule `bson:"image_and_service_modules"        json:"image_and_service_modules"           yaml:"image_and_service_modules"`
	ClusterID          string                   `bson:"cluster_id"                       json:"cluster_id"                          yaml:"cluster_id"`
	ReleaseName        string                   `bson:"release_name"                     json:"release_name"                        yaml:"release_name"`
	Timeout            int                      `bson:"timeout"                          json:"timeout"                             yaml:"timeout"`
	ReplaceResources   []Resource               `bson:"replace_resources"                json:"replace_resources"                   yaml:"replace_resources"`
}

type ImageAndServiceModule struct {
	ServiceModule string `bson:"service_module"                     json:"service_module"                        yaml:"service_module"`
	Image         string `bson:"image"                              json:"image"                                 yaml:"image"`
}

type JobTaskBuildSpec struct {
	Properties JobProperties `bson:"properties"          json:"properties"        yaml:"properties"`
	Steps      []*StepTask   `bson:"steps"               json:"steps"             yaml:"steps"`
}

type JobTaskPluginSpec struct {
	Properties JobProperties   `bson:"properties"          json:"properties"        yaml:"properties"`
	Plugin     *PluginTemplate `bson:"plugin"              json:"plugin"            yaml:"plugin"`
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
