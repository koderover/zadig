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

package types

import (
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/setting"
)

type OpenAPIRepoInput struct {
	CodeHostName string `json:"codehost_name"`
	// git type
	RepoNamespace string `json:"repo_namespace"`
	RepoName      string `json:"repo_name"`
	Branch        string `json:"branch"`
	PR            int    `json:"pr"`
	PRs           []int  `json:"prs"`
	EnableCommit  bool   `json:"enable_commit"`
	CommitID      string `json:"commit_id"`
	RemoteName    string `json:"remote_name"`
	CheckoutPath  string `json:"checkout_path"`
	SubModules    bool   `json:"submodules"`
	// perforce type
	Stream       string `json:"stream"`
	ViewMapping  string `json:"view_mapping"`
	ChangelistID int    `json:"changelist_id"`
	ShelveID     int    `json:"shelve_id"`
}

type OpenAPIWebhookConfigDetail struct {
	CodeHostName  string                 `json:"codehost_name"`
	RepoNamespace string                 `json:"repo_namespace"`
	RepoName      string                 `json:"repo_name"`
	Branch        string                 `json:"branch"`
	Events        []config.HookEventType `json:"events"`
	MatchFolders  []string               `json:"match_folders"`
}

type OpenAPIAdvancedSetting struct {
	ClusterName  string                 `json:"cluster_name"`
	StrategyName string                 `json:"strategy_name"`
	Timeout      int64                  `json:"timeout"`
	Spec         setting.RequestSpec    `json:"resource_spec"`
	Webhooks     *OpenAPIWebhookSetting `json:"webhooks,omitempty"`
	// Cache settings is for build only for now, remove this line if there are further changes
	CacheSetting        *OpenAPICacheSetting `json:"cache_setting"`
	UseHostDockerDaemon bool                 `json:"use_host_docker_daemon"`
}

type OpenAPIWebhookSetting struct {
	Enabled  bool                          `json:"enabled"`
	HookList []*OpenAPIWebhookConfigDetail `json:"hook_list"`
}

type OpenAPICacheSetting struct {
	Enabled  bool   `json:"enabled"`
	CacheDir string `json:"cache_dir"`
}

type OpenAPIServiceBuildArgs struct {
	ServiceModule string              `json:"service_module"`
	ServiceName   string              `json:"service_name"`
	RepoInfo      []*OpenAPIRepoInput `json:"repo_info"`
	Inputs        []*KV               `json:"inputs"`
}

type KV struct {
	Key          string `json:"key"`
	Value        string `json:"value"`
	Type         string `json:"type,omitempty"`
	IsCredential bool   `json:"is_credential,omitempty"`
}

type OpenAPIUserBriefInfo struct {
	UID          string `json:"uid" validate:"required"`           // 用户ID
	Account      string `json:"account" validate:"required"`       // 用户账号 (登陆名)
	Name         string `json:"name" validate:"required"`          // 用户名称 (昵称)
	IdentityType string `json:"identity_type" validate:"required"` // 用户身份类型
}

type OpenAPIRollBackStat struct {
	ProjectKey    string                  `json:"project_key" validate:"required"`    // 项目标识
	EnvName       string                  `json:"env_name" validate:"required"`       // 环境名称
	EnvType       config.EnvType          `json:"env_type" validate:"required"`       // 环境类型
	Production    bool                    `json:"production" validate:"required"`     // 是否是生产环境
	OperationType config.EnvOperationType `json:"operation_type" validate:"required"` // 操作类型
	ServiceName   string                  `json:"service_name" validate:"required"`   // 服务名称或应用名称
	ServiceType   config.ServiceType      `json:"service_type" validate:"required"`   // 服务类型
	OriginService *OpenAPIEnvService      `json:"origin_service"`                     // 回滚之前的服务信息
	UpdateService *OpenAPIEnvService      `json:"update_service"`                     // 回滚之后的服务信息
	OriginSaeApp  *OpenAPISaeApplication  `json:"origin_sae_app"`                     // 回滚之前的sae应用信息
	UpdateSaeApp  *OpenAPISaeApplication  `json:"update_sae_app"`                     // 回滚之后的sae应用信息
	CreatBy       *OpenAPIUserBriefInfo   `json:"create_by" validate:"required"`      // 创建者信息
	CreatTime     int64                   `json:"create_time" validate:"required"`    // 创建时间
}

type OpenAPIEnvService struct {
	ServiceName    string              `json:"service_name" validate:"required"` // 服务名称
	ReleaseName    string              `json:"release_name" validate:"required"` // release名称，用于helm chart类型服务
	Containers     []*OpenAPIContainer `json:"containers" validate:"required"`   // 镜像信息
	RenderedYaml   string              `json:"rendered_yaml"`                    // 渲染后的yaml，仅用于k8s类型服务
	ValuesYaml     string              `json:"values_yaml"`                      // values内容，仅用于helm和helm chart类型服务
	OverrideValues string              `json:"override_values"`                  // 覆盖的键值对，内容格式为json，仅用于helm和helm chart类型服务
	UpdateTime     int64               `json:"update_time" validate:"required"`  // 服务更新时间
}

type OpenAPIContainer struct {
	Name      string                `json:"name" validate:"required"`                 // 容器名称
	Type      setting.ContainerType `json:"type" validate:"required"`                 // 容器类型
	Image     string                `json:"image" validate:"required"`                // 完整镜像地址
	ImageName string                `json:"image_name,omitempty" validate:"required"` // 镜像名称
}

type OpenAPISaeApplication struct {
	AppName    string `json:"app_name" validate:"required"`    // 应用名称
	AppID      string `json:"app_id" validate:"required"`      // 应用ID
	ImageUrl   string `json:"image_url" validate:"required"`   // 镜像地址
	PackageUrl string `json:"package_url" validate:"required"` // 包地址
	Instances  int32  `json:"instances" validate:"required"`   // 实例数
}
