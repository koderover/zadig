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

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/types"
)

type WorkflowTask struct {
	ID           primitive.ObjectID `bson:"_id,omitempty"             json:"id,omitempty"`
	TaskID       int64              `bson:"task_id"                   json:"task_id"`
	WorkflowName string             `bson:"workflow_name"             json:"workflow_name"`
	Stages       []*StageTask       `bson:"stages"                    json:"stages"`
	ProjectName  string             `bson:"project_name,omitempty"    json:"project_name,omitempty"`
	IsDeleted    bool               `bson:"is_deleted"                json:"is_deleted"`
	IsArchived   bool               `bson:"is_archived"               json:"is_archived"`
}

func (WorkflowTask) TableName() string {
	return "workflow_task"
}

type StageTask struct {
	Name      string                 `bson:"name"          json:"name"`
	Status    config.Status          `bson:"status"        json:"status"`
	StartTime int64                  `bson:"start_time"    json:"start_time,omitempty"`
	EndTime   int64                  `bson:"end_time"      json:"end_time,omitempty"`
	Parallel  bool                   `bson:"parallel"      json:"parallel"`
	Approval  *commonmodels.Approval `bson:"approval"      json:"approval"`
	Jobs      []*JobTask             `bson:"jobs"          json:"jobs"`
	Error     string                 `bson:"error"         json:"error"`
}

type JobTask struct {
	Name       string                       `bson:"name"                json:"name"`
	JobType    string                       `bson:"type"                json:"type"`
	Status     config.Status                `bson:"status"              json:"status"`
	StartTime  int64                        `bson:"start_time"          json:"start_time,omitempty"`
	EndTime    int64                        `bson:"end_time"            json:"end_time,omitempty"`
	Error      string                       `bson:"error"               json:"error"`
	Properties commonmodels.JobProperties   `bson:"properties"          json:"properties"`
	Plugin     *commonmodels.PluginTemplate `bson:"plugin"              json:"plugin"`
	Steps      []*commonmodels.StepTask     `bson:"steps"               json:"steps"`
	Spec       interface{}                  `bson:"spec"          json:"spec"`
	Outputs    []*commonmodels.Output       `bson:"outputs"             json:"outputs"`
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

type JobProperties struct {
	Timeout         int64               `bson:"timeout"                json:"timeout"               yaml:"timeout"`
	Retry           int64               `bson:"retry"                  json:"retry"                 yaml:"retry"`
	ResourceRequest setting.Request     `bson:"res_req"                json:"res_req"               yaml:"res_req"`
	ResReqSpec      setting.RequestSpec `bson:"res_req_spec"           json:"res_req_spec"          yaml:"res_req_spec"`
	ClusterID       string              `bson:"cluster_id"             json:"cluster_id"            yaml:"cluster_id"`
	BuildOS         string              `bson:"build_os"               json:"build_os"              yaml:"build_os,omitempty"`
	ImageFrom       string              `bson:"image_from"             json:"image_from"            yaml:"image_from,omitempty"`
	ImageID         string              `bson:"image_id"               json:"image_id"              yaml:"image_id,omitempty"`
	Namespace       string              `bson:"namespace"              json:"namespace"             yaml:"namespace"`
	Envs            []*KeyVal           `bson:"envs"                   json:"envs"                  yaml:"envs"`
	// log user-defined variables, shows in workflow task detail.
	CustomEnvs   []*KeyVal            `bson:"custom_envs"            json:"custom_envs"           yaml:"custom_envs,omitempty"`
	Params       []*Param             `bson:"params"                 json:"params"                yaml:"params"`
	Paths        string               `bson:"-"                      json:"-"                     yaml:"-"`
	LogFileName  string               `bson:"log_file_name"          json:"log_file_name"         yaml:"log_file_name"`
	DockerHost   string               `bson:"-"                      json:"docker_host,omitempty" yaml:"docker_host,omitempty"`
	Registries   []*RegistryNamespace `bson:"registries"             json:"registries"            yaml:"registries"`
	Cache        types.Cache          `bson:"cache"                  json:"cache"                 yaml:"cache"`
	CacheEnable  bool                 `bson:"cache_enable"           json:"cache_enable"          yaml:"cache_enable"`
	CacheDirType types.CacheDirType   `bson:"cache_dir_type"         json:"cache_dir_type"        yaml:"cache_dir_type"`
	CacheUserDir string               `bson:"cache_user_dir"         json:"cache_user_dir"        yaml:"cache_user_dir"`
}

type Output struct {
	Name        string `bson:"name"           json:"name"             yaml:"name"`
	Description string `bson:"description"    json:"description"      yaml:"description"`
}

type Param struct {
	Name        string `bson:"name"             json:"name"             yaml:"name"`
	Description string `bson:"description"      json:"description"      yaml:"description"`
	// support string/text/choice/repo type
	ParamsType   string                 `bson:"type"                      json:"type"                        yaml:"type"`
	Value        string                 `bson:"value"                     json:"value"                       yaml:"value,omitempty"`
	Repo         *types.Repository      `bson:"repo"                     json:"repo"                         yaml:"repo,omitempty"`
	ChoiceOption []string               `bson:"choice_option,omitempty"   json:"choice_option,omitempty"     yaml:"choice_option,omitempty"`
	ChoiceValue  []string               `bson:"choice_value,omitempty"    json:"choice_value,omitempty"      yaml:"choice_value,omitempty"`
	Default      string                 `bson:"default"                   json:"default"                     yaml:"default"`
	IsCredential bool                   `bson:"is_credential"             json:"is_credential"               yaml:"is_credential"`
	Source       config.ParamSourceType `bson:"source,omitempty"          json:"source,omitempty"            yaml:"source,omitempty"`
}

type PluginTemplate struct {
	Name        string    `bson:"name"             json:"name"             yaml:"name"`
	IsOffical   bool      `bson:"is_offical"       json:"is_offical"       yaml:"is_offical"`
	Description string    `bson:"description"      json:"description"      yaml:"description"`
	RepoURL     string    `bson:"repo_url"         json:"repo_url"         yaml:"-"`
	Version     string    `bson:"version"          json:"version"          yaml:"version"`
	Image       string    `bson:"image"            json:"image"            yaml:"image"`
	Args        []string  `bson:"args"             json:"args"             yaml:"args"`
	Cmds        []string  `bson:"cmds"             json:"cmds"             yaml:"cmds"`
	Envs        []*Env    `bson:"envs"             json:"envs"             yaml:"envs"`
	Inputs      []*Param  `bson:"inputs"           json:"inputs"           yaml:"inputs"`
	Outputs     []*Output `bson:"outputs"          json:"outputs"          yaml:"outputs"`
}

type Env struct {
	Name  string `bson:"name"             json:"name"             yaml:"name"`
	Value string `bson:"value"            json:"value"            yaml:"value"`
}

type KeyVal struct {
	Key          string                            `bson:"key"                       json:"key"                         yaml:"key"`
	Value        string                            `bson:"value"                     json:"value"                       yaml:"value"`
	Type         commonmodels.ParameterSettingType `bson:"type,omitempty"            json:"type,omitempty"              yaml:"type"`
	ChoiceOption []string                          `bson:"choice_option,omitempty"   json:"choice_option,omitempty"     yaml:"choice_option,omitempty"`
	ChoiceValue  []string                          `bson:"choice_value,omitempty"    json:"choice_value,omitempty"      yaml:"choice_value,omitempty"`
	IsCredential bool                              `bson:"is_credential"             json:"is_credential"               yaml:"is_credential"`
}

type StepDeploySpec struct {
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

type StepHelmDeploySpec struct {
	Env                string                                `bson:"env"                              json:"env"                                 yaml:"env"`
	ServiceName        string                                `bson:"service_name"                     json:"service_name"                        yaml:"service_name"`
	ServiceType        string                                `bson:"service_type"                     json:"service_type"                        yaml:"service_type"`
	SkipCheckRunStatus bool                                  `bson:"skip_check_run_status"            json:"skip_check_run_status"               yaml:"skip_check_run_status"`
	ImageAndModules    []*commonmodels.ImageAndServiceModule `bson:"image_and_service_modules"        json:"image_and_service_modules"           yaml:"image_and_service_modules"`
	ClusterID          string                                `bson:"cluster_id"                       json:"cluster_id"                          yaml:"cluster_id"`
	ReleaseName        string                                `bson:"release_name"                     json:"release_name"                        yaml:"release_name"`
	Timeout            int                                   `bson:"timeout"                          json:"timeout"                             yaml:"timeout"`
	ReplaceResources   []commonmodels.Resource               `bson:"replace_resources"                json:"replace_resources"                   yaml:"replace_resources"`
}

type StepCustomDeploySpec struct {
	Namespace          string                  `bson:"namespace"              json:"namespace"             yaml:"namespace"`
	ClusterID          string                  `bson:"cluster_id"             json:"cluster_id"            yaml:"cluster_id"`
	Timeout            int64                   `bson:"timeout"                json:"timeout"               yaml:"timeout"`
	WorkloadType       string                  `bson:"workload_type"          json:"workload_type"         yaml:"workload_type"`
	WorkloadName       string                  `bson:"workload_name"          json:"workload_name"         yaml:"workload_name"`
	ContainerName      string                  `bson:"container_name"         json:"container_name"        yaml:"container_name"`
	Image              string                  `bson:"image"                  json:"image"                 yaml:"image"`
	SkipCheckRunStatus bool                    `bson:"skip_check_run_status"  json:"skip_check_run_status" yaml:"skip_check_run_status"`
	ReplaceResources   []commonmodels.Resource `bson:"replace_resources"      json:"replace_resources"     yaml:"replace_resources"`
}

type JobTaskDeploySpec struct {
	Env                string              `bson:"env"                              json:"env"                                 yaml:"env"`
	ServiceName        string              `bson:"service_name"                     json:"service_name"                        yaml:"service_name"`
	ServiceType        string              `bson:"service_type"                     json:"service_type"                        yaml:"service_type"`
	ServiceModule      string              `bson:"service_module"                   json:"service_module"                      yaml:"service_module"`
	SkipCheckRunStatus bool                `bson:"skip_check_run_status"            json:"skip_check_run_status"               yaml:"skip_check_run_status"`
	Image              string              `bson:"image"                            json:"image"                               yaml:"image"`
	ClusterID          string              `bson:"cluster_id"                       json:"cluster_id"                          yaml:"cluster_id"`
	Timeout            int                 `bson:"timeout"                          json:"timeout"                             yaml:"timeout"`
	ReplaceResources   []Resource          `bson:"replace_resources"                json:"replace_resources"                   yaml:"replace_resources"`
	RelatedPodLabels   []map[string]string `bson:"-"                                json:"-"                                   yaml:"-"`
}

type Resource struct {
	Name      string `bson:"name"                              json:"name"                                 yaml:"name"`
	Kind      string `bson:"kind"                              json:"kind"                                 yaml:"kind"`
	Container string `bson:"container"                         json:"container"                            yaml:"container"`
	Origin    string `bson:"origin"                            json:"origin"                               yaml:"origin"`
}
