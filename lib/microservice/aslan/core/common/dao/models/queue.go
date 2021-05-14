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
	"sync"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/koderover/zadig/lib/microservice/aslan/config"
)

type Queue struct {
	ID           primitive.ObjectID       `bson:"_id,omitempty"           json:"id,omitempty"`
	TaskID       int64                    `bson:"task_id"                   json:"task_id"`
	ProductName  string                   `bson:"product_name"              json:"product_name"`
	PipelineName string                   `bson:"pipeline_name"             json:"pipeline_name"`
	Namespace    string                   `bson:"namespace,omitempty"       json:"namespace,omitempty"`
	Type         config.PipelineType      `bson:"type"                      json:"type"`
	Status       config.Status            `bson:"status"                    json:"status,omitempty"`
	Description  string                   `bson:"description,omitempty"     json:"description,omitempty"`
	TaskCreator  string                   `bson:"task_creator"              json:"task_creator,omitempty"`
	TaskRevoker  string                   `bson:"task_revoker,omitempty"    json:"task_revoker,omitempty"`
	CreateTime   int64                    `bson:"create_time"               json:"create_time,omitempty"`
	StartTime    int64                    `bson:"start_time"                json:"start_time,omitempty"`
	EndTime      int64                    `bson:"end_time"                  json:"end_time,omitempty"`
	SubTasks     []map[string]interface{} `bson:"sub_tasks"                 json:"sub_tasks"`
	Stages       []*Stage                 `bson:"stages"                    json:"stages"`
	ReqID        string                   `bson:"req_id,omitempty"          json:"req_id,omitempty"`
	AgentHost    string                   `bson:"agent_host,omitempty"      json:"agent_host,omitempty"`
	DockerHost   string                   `bson:"-"                         json:"docker_host,omitempty"`
	TeamID       int                      `bson:"team_id,omitempty"         json:"team_id,omitempty"`
	TeamName     string                   `bson:"team,omitempty"            json:"team,omitempty"`
	IsDeleted    bool                     `bson:"is_deleted"                json:"is_deleted"`
	IsArchived   bool                     `bson:"is_archived"               json:"is_archived"`
	AgentID      string                   `bson:"agent_id"        json:"agent_id"`
	// 是否允许同时运行多次
	MultiRun bool `bson:"multi_run"                 json:"multi_run"`
	// target 服务名称, k8s为容器名称, 物理机为服务名
	Target string `bson:"target,omitempty"                    json:"target"`
	// 使用预定义编译管理模块中的内容生成SubTasks,
	// 查询条件为 服务模板名称: ServiceTmpl, 版本: BuildModuleVer
	// 如果为空，则使用pipeline自定义SubTasks
	BuildModuleVer string `bson:"build_module_ver,omitempty"                 json:"build_module_ver"`
	ServiceName    string `bson:"service_name,omitempty"              json:"service_name,omitempty"`
	// TaskArgs 单服务工作流任务参数
	TaskArgs *TaskArgs `bson:"task_args,omitempty"         json:"task_args,omitempty"`
	// WorkflowArgs 多服务工作流任务参数
	WorkflowArgs *WorkflowTaskArgs `bson:"workflow_args"         json:"workflow_args,omitempty"`
	// TestArgs 测试任务参数
	TestArgs *TestTaskArgs `bson:"test_args,omitempty"         json:"test_args,omitempty"`
	// ServiceTaskArgs 脚本部署工作流任务参数
	ServiceTaskArgs *ServiceTaskArgs `bson:"service_args,omitempty"         json:"service_args,omitempty"`
	ConfigPayload   *ConfigPayload   `json:"config_payload,omitempty"`
	Error           string           `bson:"error,omitempty"                json:"error,omitempty"`
	// OrgID 单租户ID
	OrgID      int                 `bson:"org_id,omitempty"          json:"org_id,omitempty"`
	Services   [][]*ProductService `bson:"services"                  json:"services"`
	Render     *RenderInfo         `bson:"render"                    json:"render"`
	StorageUri string              `bson:"storage_uri,omitempty" json:"storage_uri,omitempty"`
	// interface{} 为types.TestReport
	TestReports map[string]interface{} `bson:"test_reports,omitempty" json:"test_reports,omitempty"`

	RwLock sync.Mutex `bson:"-" json:"-"`

	ResetImage bool `json:"resetImage" bson:"resetImage"`

	TriggerBy *TriggerBy `json:"trigger_by,omitempty" bson:"trigger_by,omitempty"`

	Features        []string `bson:"features" json:"features"`
	IsRestart       bool     `bson:"is_restart"                      json:"is_restart"`
	StorageEndpoint string   `bson:"storage_endpoint"            json:"storage_endpoint"`
}

type TriggerBy struct {
	// 触发此次任务的代码库信息
	CodehostID int    `bson:"codehost_id,omitempty"      json:"codehost_id,omitempty"`
	RepoOwner  string `bson:"repo_owner,omitempty"       json:"repo_owner,omitempty"`
	RepoName   string `bson:"repo_name,omitempty"        json:"repo_name,omitempty"`
	Source     string `json:"source,omitempty" bson:"source,omitempty"`
	// 触发此次任务的merge request id，用于判断多个任务对应的commit是否属于同一个merge request
	MergeRequestID string `json:"merge_request_id,omitempty" bson:"merge_request_id,omitempty"`
	// 触发此次任务的commit id
	CommitID string `json:"commit_id,omitempty" bson:"commit_id,omitempty"`
}

type ServiceTaskArgs struct {
	ProductName        string   `bson:"product_name"            json:"product_name"`
	ServiceName        string   `bson:"service_name"            json:"service_name"`
	Revision           int64    `bson:"revision"                json:"revision"`
	BuildName          string   `bson:"build_name"              json:"build_name"`
	EnvNames           []string `bson:"env_names"               json:"env_names"`
	ServiceTaskCreator string   `bson:"service_task_creator"    json:"service_task_creator"`
	Namespace          string   `bson:"namespace"               json:"namespace"`
	K8sNamespace       string   `bson:"k8s_namespace"           json:"k8s_namespace"`
	Updatable          bool     `bson:"updatable"               json:"updatable"`
}

type ConfigPayload struct {
	Aslan     AslanConfig    `json:"aslan"`
	Proxy     Proxy          `json:"proxy"`
	S3Storage S3Config       `json:"s3_storage"`
	Github    GithubConfig   `json:"github"`
	Gitlab    GitlabConfig   `json:"gitlab"`
	Build     BuildConfig    `json:"build"`
	Test      TestConfig     `json:"test"`
	Registry  RegistryConfig `json:"registry"`
	Release   ReleaseConfig  `json:"release"`
	// 推送线上镜像需要的用户名密码, 根据pipeline镜像发布模块动态配置
	ImageRelease ImageReleaseConfig `json:"image_release"`
	Docker       DockerConfig       `json:"docker"`

	ClassicBuild bool `json:"classic_build"`

	CustomDNSSupported bool `json:"custom_dns_supported"`
	HubServerAddr      string
	DeployClusterId    string

	RepoConfigs map[string]*RegistryNamespace

	// IgnoreCache means ignore docker build cache
	IgnoreCache bool `json:"ignore_cache"`

	// ResetCache means ignore workspace cache
	ResetCache bool `json:"reset_cache"`
	//
	JenkinsBuildConfig JenkinsBuildConfig `json:"jenkins_build_config"`
}

type AslanConfig struct {
	URL              string
	WarpdriveService string
}

type ProxyConfig struct {
	HTTPAddr   string
	HTTPSAddr  string
	Socks5Addr string
	NoProxy    string
}

type S3Config struct {
	Ak       string
	Sk       string
	Endpoint string
	Bucket   string
	Path     string
	Protocol string
}

type GithubConfig struct {
	// github API access token
	AccessToken string
	// github API admin access token
	AdminAccessToken string
	// Aslan webhook url
	HookURL string
	// Aslan webhook secret
	HookSecret string
	// github ssh key with base64 encoded
	SSHKey string
	// github knownhost
	KnownHost string
	// github app private key
	AppKey string
	// gihhub app id
	AppID int
}

type GitlabConfig struct {
	APIServer string
	// Github API access token
	AccessToken string
	// gitlab ssh key with base64 encoded
	SSHKey string
	// gitlab knownhost
	KnownHost string
}

type BuildConfig struct {
	KubeNamespace string
}

type TestConfig struct {
	KubeNamespace string
}

type RegistryConfig struct {
	Addr        string
	AccessKey   string
	SecretKey   string
	Namespace   string
	RepoAddress string
}

type ReleaseConfig struct {
	// ReaperImage sets build job image
	// e.g. xxx.com/poetry-resources/reaper-plugin:1.0.0
	ReaperImage string
	// ReaperBinaryFile sets download url of reaper binary file in build job
	// e.g. http://resource.koderover.com/reaper-20201014203000
	ReaperBinaryFile string
	// PredatorImage sets docker build image
	// e.g. xxx.com/poetry-resources/predator-plugin:v0.1.0
	PredatorImage string
}

type ImageReleaseConfig struct {
	Addr      string
	AccessKey string
	SecretKey string
}

type DockerConfig struct {
	HostList []string
}

func (Queue) TableName() string {
	return "pipeline_queue"
}

type JenkinsBuildConfig struct {
	JenkinsBuildImage string
}
