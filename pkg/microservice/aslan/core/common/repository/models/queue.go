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

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/setting"
)

type Queue struct {
	ID                      primitive.ObjectID           `bson:"_id,omitempty"                              json:"id,omitempty"`
	TaskID                  int64                        `bson:"task_id"                                    json:"task_id"`
	ProductName             string                       `bson:"product_name"                               json:"product_name"`
	PipelineName            string                       `bson:"pipeline_name"                              json:"pipeline_name"`
	PipelineDisplayName     string                       `bson:"pipeline_display_name"                      json:"pipeline_display_name"`
	Namespace               string                       `bson:"namespace,omitempty"                        json:"namespace,omitempty"`
	Type                    config.PipelineType          `bson:"type"                                       json:"type"`
	Status                  config.Status                `bson:"status"                                     json:"status,omitempty"`
	Description             string                       `bson:"description,omitempty"                      json:"description,omitempty"`
	TaskCreator             string                       `bson:"task_creator"                               json:"task_creator,omitempty"`
	TaskRevoker             string                       `bson:"task_revoker,omitempty"                     json:"task_revoker,omitempty"`
	CreateTime              int64                        `bson:"create_time"                                json:"create_time,omitempty"`
	StartTime               int64                        `bson:"start_time"                                 json:"start_time,omitempty"`
	EndTime                 int64                        `bson:"end_time"                                   json:"end_time,omitempty"`
	SubTasks                []map[string]interface{}     `bson:"sub_tasks"                                  json:"sub_tasks"`
	Stages                  []*Stage                     `bson:"stages"                                     json:"stages"`
	ReqID                   string                       `bson:"req_id,omitempty"                           json:"req_id,omitempty"`
	AgentHost               string                       `bson:"agent_host,omitempty"                       json:"agent_host,omitempty"`
	DockerHost              string                       `bson:"-"                                          json:"docker_host,omitempty"`
	TeamID                  int                          `bson:"team_id,omitempty"                          json:"team_id,omitempty"`
	TeamName                string                       `bson:"team,omitempty"                             json:"team,omitempty"`
	IsDeleted               bool                         `bson:"is_deleted"                                 json:"is_deleted"`
	IsArchived              bool                         `bson:"is_archived"                                json:"is_archived"`
	AgentID                 string                       `bson:"agent_id"                                   json:"agent_id"`
	MultiRun                bool                         `bson:"multi_run"                                  json:"multi_run"`
	Target                  string                       `bson:"target,omitempty"                           json:"target"` // target service name, for k8s: containerName, for pm: serviceName
	BuildModuleVer          string                       `bson:"build_module_ver,omitempty"                 json:"build_module_ver"`
	ServiceName             string                       `bson:"service_name,omitempty"                     json:"service_name,omitempty"`
	TaskArgs                *TaskArgs                    `bson:"task_args,omitempty"                        json:"task_args,omitempty"`     // TaskArgs job parameters for single-service workflow
	WorkflowArgs            *WorkflowTaskArgs            `bson:"workflow_args"                              json:"workflow_args,omitempty"` // WorkflowArgs job parameters for multi-service workflow
	ScanningArgs            *ScanningArgs                `bson:"scanning_args,omitempty"                    json:"scanning_args,omitempty"` // ScanningArgs argument for scanning tasks
	TestArgs                *TestTaskArgs                `bson:"test_args,omitempty"                        json:"test_args,omitempty"`     // TestArgs parameters for testing
	ServiceTaskArgs         *ServiceTaskArgs             `bson:"service_args,omitempty"                     json:"service_args,omitempty"`  // ServiceTaskArgs parameters for script-deployed workflows
	ArtifactPackageTaskArgs *ArtifactPackageTaskArgs     `bson:"artifact_package_args,omitempty"            json:"artifact_package_args,omitempty"`
	ConfigPayload           *ConfigPayload               `bson:"configpayload,omitempty"                    json:"config_payload"`
	Error                   string                       `bson:"error,omitempty"                            json:"error,omitempty"`
	Services                [][]*ProductService          `bson:"services"                                   json:"services"`
	Render                  *RenderInfo                  `bson:"render"                                     json:"render"`
	StorageURI              string                       `bson:"storage_uri,omitempty"                      json:"storage_uri,omitempty"`
	TestReports             map[string]interface{}       `bson:"test_reports,omitempty"                     json:"test_reports,omitempty"`
	RwLock                  sync.Mutex                   `bson:"-"                                          json:"-"`
	ResetImage              bool                         `bson:"resetImage"                                 json:"resetImage"`
	ResetImagePolicy        setting.ResetImagePolicyType `bson:"reset_image_policy"                         json:"reset_image_policy"`
	TriggerBy               *TriggerBy                   `bson:"trigger_by,omitempty"                       json:"trigger_by,omitempty"`
	Features                []string                     `bson:"features"                                   json:"features"`
	IsRestart               bool                         `bson:"is_restart"                                 json:"is_restart"`
	StorageEndpoint         string                       `bson:"storage_endpoint"                           json:"storage_endpoint"`
}

type TriggerBy struct {
	// 触发此次任务的代码库信息
	CodehostID    int    `bson:"codehost_id,omitempty"      json:"codehost_id,omitempty"`
	RepoOwner     string `bson:"repo_owner,omitempty"       json:"repo_owner,omitempty"`
	RepoName      string `bson:"repo_name,omitempty"        json:"repo_name,omitempty"`
	RepoNamespace string `bson:"repo_namespace" json:"repo_namespace"`
	Source        string `json:"source,omitempty" bson:"source,omitempty"`
	// 触发此次任务的merge request id，用于判断多个任务对应的commit是否属于同一个merge request
	MergeRequestID string `json:"merge_request_id,omitempty" bson:"merge_request_id,omitempty"`
	// 触发此次任务的commit id
	CommitID string `bson:"commit_id,omitempty" json:"commit_id,omitempty"`
	// the git branch which triggered this task
	Ref       string `bson:"ref" json:"ref"`
	EventType string `bson:"event_type" json:"event_type"`
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

type ImageData struct {
	ImageUrl   string `bson:"image_url"   json:"image_url"`
	ImageName  string `bson:"image_name"  json:"image_name"`
	ImageTag   string `bson:"image_tag"   json:"image_tag"`
	CustomTag  string `bson:"custom_tag"  json:"custom_tag"`
	RegistryID string `bson:"registry_id" json:"registry_id"`
}

type ImagesByService struct {
	ServiceName string       `bson:"service_name" json:"service_name"`
	Images      []*ImageData `bson:"images" json:"images"`
}

type ArtifactPackageTaskArgs struct {
	ProjectName      string             `bson:"project_name"            json:"project_name"`
	EnvName          string             `bson:"env_name"                json:"env_name"`
	Images           []*ImagesByService `bson:"images"                  json:"images"`
	SourceRegistries []string           `bson:"source_registries"       json:"source_registries"`
	TargetRegistries []string           `bson:"target_registries"       json:"target_registries"`
}

type ConfigPayload struct {
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
	DeployClusterID    string
	AesKey             string `json:"aes_key"`

	RepoConfigs map[string]*RegistryNamespace

	// IgnoreCache means ignore docker build cache
	IgnoreCache bool `json:"ignore_cache"`

	// ResetCache means ignore workspace cache
	ResetCache         bool               `json:"reset_cache"`
	JenkinsBuildConfig JenkinsBuildConfig `json:"jenkins_build_config"`
	PrivateKeys        []*PrivateKey      `json:"private_keys"`
	K8SClusters        []*K8SClusterResp  `json:"k8s_clusters"`

	// RegistryID is the id of product registry
	RegistryID string `json:"registry_id"`

	// build concurrency settings
	BuildConcurrency int64 `json:"build_concurrency"`
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
	AccessToken string `json:"access_token"`
	// github ssh key with base64 encoded
	SSHKey string `json:"ssh_key"`
	// github knownhost
	KnownHost string `json:"known_host"`
	// github app private key
	AppKey string `json:"app_key"`
	// gihhub app id
	AppID int `json:"app_id"`
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
	// e.g. xxx.com/resources/reaper-plugin:1.0.0
	ReaperImage string
	// ReaperBinaryFile sets download url of reaper binary file in build job
	// e.g. http://resource.koderover.com/reaper-20201014203000
	ReaperBinaryFile string
	// PredatorImage sets docker build image
	// e.g. xxx.com/resources/predator-plugin:v0.1.0
	PredatorImage string
	// PackagerImage sets docker build image
	// e.g. xxx.com/resources/predator-plugin:v0.1.0
	PackagerImage string
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
