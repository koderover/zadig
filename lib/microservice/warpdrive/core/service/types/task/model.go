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

package task

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/koderover/zadig/lib/microservice/warpdrive/config"
)

type Task struct {
	//ID           primitive.ObjectID        `bson:"_id,omitempty"           json:"id,omitempty"`
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
	// ConfigPayload 系统配置信息
	ConfigPayload *ConfigPayload `json:"config_payload,omitempty"`
	Error         string         `bson:"error,omitempty"                json:"error,omitempty"`
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

type RenderInfo struct {
	Name        string `bson:"name"                     json:"name"`
	Revision    int64  `bson:"revision"                 json:"revision"`
	ProductTmpl string `bson:"product_tmpl"             json:"product_tmpl"`
	Descritpion string `bson:"description"              json:"description"`
}

type TaskArgs struct {
	ProductName    string        `bson:"product_name"            json:"product_name"`
	PipelineName   string        `bson:"pipeline_name"           json:"pipeline_name"`
	Builds         []*Repository `bson:"builds"                  json:"builds"`
	BuildArgs      []*KeyVal     `bson:"build_args"              json:"build_args"`
	Deploy         DeployArgs    `bson:"deploy"                  json:"deploy"`
	Test           TestArgs      `bson:"test"                    json:"test,omitempty"`
	HookPayload    *HookPayload  `bson:"hook_payload"            json:"hook_payload,omitempty"`
	TaskCreator    string        `bson:"task_creator"            json:"task_creator,omitempty"`
	ReqID          string        `bson:"req_id"                  json:"req_id"`
	IsQiNiu        bool          `bson:"is_qiniu"                json:"is_qiniu"`
	NotificationId string        `bson:"notification_id"         json:"notification_id"`
}

type DeployArgs struct {
	// 目标部署环境
	Namespace string `json:"namespace"`
	// 镜像或者二进制名称后缀, 一般为branch或者PR
	Tag string `json:"suffix"`
	// 部署镜像名称
	// 格式: xxx.com/{namespace}/{service name}:{timestamp}-{suffix}}
	// timestamp format: 20060102150405
	Image string `json:"image"`
	// 部署二进制包名称
	// 格式: {service name}-{timestamp}-{suffix}}.tar.gz
	// timestamp format: 20060102150405
	PackageFile string `json:"package_file"`
}

type WorkflowTaskArgs struct {
	WorkflowName    string `bson:"workflow_name"                json:"workflow_name"`
	ProductTmplName string `bson:"product_tmpl_name"            json:"product_tmpl_name"`
	Description     string `bson:"description,omitempty"        json:"description,omitempty"`
	//为了兼容老数据，namespace可能会存多个环境名称，用逗号隔开
	Namespace          string          `bson:"namespace"                    json:"namespace"`
	BaseNamespace      string          `bson:"base_namespace,omitempty"     json:"base_namespace,omitempty"`
	EnvRecyclePolicy   string          `bson:"env_recycle_policy,omitempty" json:"env_recycle_policy,omitempty"`
	EnvUpdatePolicy    string          `bson:"env_update_policy,omitempty"  json:"env_update_policy,omitempty"`
	Target             []*TargetArgs   `bson:"targets"                      json:"targets"`
	Artifact           []*ArtifactArgs `bson:"artifact_args"                json:"artifact_args"`
	Tests              []*TestArgs     `bson:"tests"                        json:"tests"`
	VersionArgs        *VersionArgs    `bson:"version_args,omitempty"       json:"version_args,omitempty"`
	ReqID              string          `bson:"req_id"                       json:"req_id"`
	RegistryID         string          `bson:"registry_id,omitempty"        json:"registry_id,omitempty"`
	DistributeEnabled  bool            `bson:"distribute_enabled"           json:"distribute_enabled"`
	WorklowTaskCreator string          `bson:"workflow_task_creator"        json:"workflow_task_creator"`
	// Ignore docker build cache
	IgnoreCache bool `json:"ignore_cache" bson:"ignore_cache"`
	// Ignore workspace cache and reset volume
	ResetCache bool `json:"reset_cache" bson:"reset_cache"`

	// NotificationId is the id of scmnotify.Notification
	NotificationId string `bson:"notification_id" json:"notification_id"`

	// webhook触发工作流任务时，触发任务的repo信息、prID和commitID
	MergeRequestID string `bson:"merge_request_id" json:"merge_request_id"`
	CommitID       string `bson:"commit_id"        json:"commit_id"`
	Source         string `bson:"source"           json:"source"`
	CodehostID     int    `bson:"codehost_id"      json:"codehost_id"`
	RepoOwner      string `bson:"repo_owner"       json:"repo_owner"`
	RepoName       string `bson:"repo_name"        json:"repo_name"`

	//github check run
	HookPayload *HookPayload `bson:"hook_payload"            json:"hook_payload,omitempty"`
	// 请求模式，openAPI表示外部客户调用
	RequestMode string `json:"request_mode,omitempty"`
	IsParallel  bool   `json:"is_parallel" bson:"is_parallel"`
}

type TestArgs struct {
	Namespace      string        `bson:"namespace" json:"namespace"`
	TestModuleName string        `bson:"test_module_name" json:"test_module_name"`
	Envs           []*KeyVal     `bson:"envs" json:"envs"`
	Builds         []*Repository `bson:"builds" json:"builds"`
}

type ArtifactArgs struct {
	Name        string      `bson:"name"                      json:"name"`
	ServiceName string      `bson:"service_name"              json:"service_name"`
	Image       string      `bson:"image"                     json:"image"`
	Deploy      []DeployEnv `bson:"deloy"                     json:"deploy"`
}

type VersionArgs struct {
	Enabled bool     `bson:"enabled" json:"enabled"`
	Version string   `bson:"version" json:"version"`
	Desc    string   `bson:"desc"    json:"desc"`
	Labels  []string `bson:"labels"  json:"labels"`
}

type DeployEnv struct {
	Env         string `json:"env"`
	Type        string `json:"type"`
	ProductName string `json:"product_name,omitempty"`
}

type HookPayload struct {
	Owner      string `bson:"owner"          json:"owner,omitempty"`
	Repo       string `bson:"repo"           json:"repo,omitempty"`
	Branch     string `bson:"branch"         json:"branch,omitempty"`
	Ref        string `bson:"ref"            json:"ref,omitempty"`
	IsPr       bool   `bson:"is_pr"          json:"is_pr,omitempty"`
	CheckRunID int64  `bson:"check_run_id"   json:"check_run_id,omitempty"`
	DeliveryID string `bson:"delivery_id"    json:"delivery_id,omitempty"`
}

type TargetArgs struct {
	Name        string      `bson:"name"                      json:"name"`
	ServiceName string      `bson:"service_name"              json:"service_name"`
	Build       *BuildArgs  `bson:"build"                     json:"build"`
	Version     string      `bson:"version"                   json:"version"`
	Deploy      []DeployEnv `bson:"deloy"                     json:"deploy"`
	Image       string      `bson:"image"                     json:"image"`
	BinFile     string      `bson:"bin_file"                  json:"bin_file"`
	Envs        []*KeyVal   `bson:"envs"                      json:"envs"`
	HasBuild    bool        `bson:"has_build"                 json:"has_build"`
}

type TestTaskArgs struct {
	ProductName     string `bson:"product_name"            json:"product_name"`
	TestName        string `bson:"test_name"               json:"test_name"`
	TestTaskCreator string `bson:"test_task_creator"       json:"test_task_creator"`
	NotificationId  string `bson:"notification_id"         json:"notification_id"`
	ReqID           string `bson:"req_id"                  json:"req_id"`
	// webhook触发测试任务时，触发任务的repo、prID和commitID
	MergeRequestID string `bson:"merge_request_id" json:"merge_request_id"`
	CommitID       string `bson:"commit_id"        json:"commit_id"`
	Source         string `bson:"source"           json:"source"`
	CodehostID     int    `bson:"codehost_id"      json:"codehost_id"`
	RepoOwner      string `bson:"repo_owner"       json:"repo_owner"`
	RepoName       string `bson:"repo_name"        json:"repo_name"`
}

type BuildArgs struct {
	Repos []*Repository `bson:"repos"               json:"repos"`
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

type ProductService struct {
	ServiceName string           `bson:"service_name"               json:"service_name"`
	Type        string           `bson:"type"                       json:"type"`
	Revision    int64            `bson:"revision"                   json:"revision"`
	Containers  []*Container     `bson:"containers"                 json:"containers,omitempty"`
	Configs     []*ServiceConfig `bson:"configs,omitempty"          json:"configs,omitempty"`
	Render      *RenderInfo      `bson:"render,omitempty"           json:"render,omitempty"` // 记录每个服务render信息 便于更新单个服务
	EnvConfigs  []*EnvConfig     `bson:"-"                          json:"env_configs,omitempty"`
}

type Container struct {
	Name  string `bson:"name"           json:"name"`
	Image string `bson:"image"          json:"image"`
}

type ServiceConfig struct {
	ConfigName string `bson:"config_name"           json:"config_name"`
	Revision   int64  `bson:"revision"              json:"revision"`
}

type EnvConfig struct {
	EnvName string   `bson:"env_name,omitempty" json:"env_name"`
	HostIDs []string `bson:"host_ids,omitempty" json:"host_ids"`
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

type Stage struct {
	// 注意: 同一个stage暂时不能运行不同类型的Task
	TaskType    config.TaskType                   `bson:"type"               json:"type"`
	Status      config.Status                     `bson:"status"             json:"status"`
	RunParallel bool                              `bson:"run_parallel"       json:"run_parallel"`
	Desc        string                            `bson:"desc,omitempty"     json:"desc,omitempty"`
	SubTasks    map[string]map[string]interface{} `bson:"sub_tasks"          json:"sub_tasks"`
	AfterAll    bool                              `json:"after_all" bson:"after_all"`
}

func (Task) TableName() string {
	return "pipeline_task_v2"
}

func IToi(before interface{}, after interface{}) error {
	b, err := json.Marshal(before)
	if err != nil {
		return fmt.Errorf("marshal task error: %v", err)
	}

	if err := json.Unmarshal(b, &after); err != nil {
		return fmt.Errorf("unmarshal task error: %v", err)
	}

	return nil
}
