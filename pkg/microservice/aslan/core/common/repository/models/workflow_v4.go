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
	"github.com/koderover/zadig/pkg/tool/lark"
	"github.com/koderover/zadig/pkg/types"
)

type WorkflowV4 struct {
	ID              primitive.ObjectID       `bson:"_id,omitempty"       yaml:"-"                   json:"id"`
	Name            string                   `bson:"name"                yaml:"name"                json:"name"`
	DisplayName     string                   `bson:"display_name"        yaml:"display_name"        json:"display_name"`
	Category        setting.WorkflowCategory `bson:"category"            yaml:"category"            json:"category"`
	KeyVals         []*KeyVal                `bson:"key_vals"            yaml:"key_vals"            json:"key_vals"`
	Params          []*Param                 `bson:"params"              yaml:"params"              json:"params"`
	Stages          []*WorkflowStage         `bson:"stages"              yaml:"stages"              json:"stages"`
	Project         string                   `bson:"project"             yaml:"project"             json:"project"`
	Description     string                   `bson:"description"         yaml:"description"         json:"description"`
	CreatedBy       string                   `bson:"created_by"          yaml:"created_by"          json:"created_by"`
	CreateTime      int64                    `bson:"create_time"         yaml:"create_time"         json:"create_time"`
	UpdatedBy       string                   `bson:"updated_by"          yaml:"updated_by"          json:"updated_by"`
	UpdateTime      int64                    `bson:"update_time"         yaml:"update_time"         json:"update_time"`
	MultiRun        bool                     `bson:"multi_run"           yaml:"multi_run"           json:"multi_run"`
	NotifyCtls      []*NotifyCtl             `bson:"notify_ctls"         yaml:"notify_ctls"         json:"notify_ctls"`
	HookCtls        []*WorkflowV4Hook        `bson:"hook_ctl"            yaml:"-"                   json:"hook_ctl"`
	JiraHookCtls    []*JiraHook              `bson:"jira_hook_ctls"      yaml:"-"                   json:"jira_hook_ctls"`
	MeegoHookCtls   []*MeegoHook             `bson:"meego_hook_ctls"     yaml:"-"                   json:"meego_hook_ctls"`
	GeneralHookCtls []*GeneralHook           `bson:"general_hook_ctls"   yaml:"-"                   json:"general_hook_ctls"`
	NotificationID  string                   `bson:"notification_id"     yaml:"-"                   json:"notification_id"`
	HookPayload     *HookPayload             `bson:"hook_payload"        yaml:"-"                   json:"hook_payload,omitempty"`
	BaseName        string                   `bson:"base_name"           yaml:"-"                   json:"base_name"`
	ShareStorages   []*ShareStorage          `bson:"share_storages"      yaml:"share_storages"      json:"share_storages"`
}

type WorkflowStage struct {
	Name     string    `bson:"name"          yaml:"name"         json:"name"`
	Parallel bool      `bson:"parallel"      yaml:"parallel"     json:"parallel"`
	Approval *Approval `bson:"approval"      yaml:"approval"     json:"approval"`
	Jobs     []*Job    `bson:"jobs"          yaml:"jobs"         json:"jobs"`
}

type Approval struct {
	Enabled        bool                `bson:"enabled"                     yaml:"enabled"                       json:"enabled"`
	Type           config.ApprovalType `bson:"type"                        yaml:"type"                          json:"type"`
	Description    string              `bson:"description"                 yaml:"description"                   json:"description"`
	NativeApproval *NativeApproval     `bson:"native_approval"             yaml:"native_approval,omitempty"     json:"native_approval,omitempty"`
	LarkApproval   *LarkApproval       `bson:"lark_approval"               yaml:"lark_approval,omitempty"       json:"lark_approval,omitempty"`
}

type NativeApproval struct {
	Timeout         int                    `bson:"timeout"                     yaml:"timeout"                    json:"timeout"`
	ApproveUsers    []*User                `bson:"approve_users"               yaml:"approve_users"              json:"approve_users"`
	NeededApprovers int                    `bson:"needed_approvers"            yaml:"needed_approvers"           json:"needed_approvers"`
	RejectOrApprove config.ApproveOrReject `bson:"reject_or_approve"           yaml:"-"                          json:"reject_or_approve"`
}

type LarkApproval struct {
	Timeout      int                 `bson:"timeout"                     yaml:"timeout"                    json:"timeout"`
	ApprovalID   string              `bson:"approval_id"                 yaml:"approval_id"                json:"approval_id"`
	ApproveUsers []*LarkApprovalUser `bson:"approve_users"               yaml:"approve_users"              json:"approve_users"`
}

type LarkApprovalUser struct {
	lark.UserInfo   `bson:",inline"  yaml:",inline"  json:",inline"`
	RejectOrApprove config.ApproveOrReject `bson:"reject_or_approve"           yaml:"-"                          json:"reject_or_approve"`
	Comment         string                 `bson:"comment"                     yaml:"-"                          json:"comment"`
	OperationTime   int64                  `bson:"operation_time"              yaml:"-"                          json:"operation_time"`
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
	// only for webhook workflow args to skip some tasks.
	Skipped   bool                `bson:"skipped"      yaml:"skipped"    json:"skipped"`
	RunPolicy config.JobRunPolicy `bson:"run_policy"   yaml:"run_policy" json:"run_policy"`
	Spec      interface{}         `bson:"spec"         yaml:"spec"       json:"spec"`
}

type CustomDeployJobSpec struct {
	Namespace          string `bson:"namespace"              json:"namespace"             yaml:"namespace"`
	ClusterID          string `bson:"cluster_id"             json:"cluster_id"            yaml:"cluster_id"`
	DockerRegistryID   string `bson:"docker_registry_id"     json:"docker_registry_id"    yaml:"docker_registry_id"`
	SkipCheckRunStatus bool   `bson:"skip_check_run_status"  json:"skip_check_run_status" yaml:"skip_check_run_status"`
	// support two sources, runtime/fixed.
	Source string `bson:"source"                 json:"source"                yaml:"source"`
	// unit is minute.
	Timeout int64            `bson:"timeout"                json:"timeout"               yaml:"timeout"`
	Targets []*DeployTargets `bson:"targets"                json:"targets"               yaml:"targets"`
}

type DeployTargets struct {
	// workload_type/workload_name/container_name.
	Target string `bson:"target"           json:"target"            yaml:"target"`
	Image  string `bson:"image,omitempty"  json:"image,omitempty"   yaml:"image,omitempty"`
}

type PluginJobSpec struct {
	Properties *JobProperties  `bson:"properties"               yaml:"properties"              json:"properties"`
	Plugin     *PluginTemplate `bson:"plugin"                   yaml:"plugin"                  json:"plugin"`
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
	ServiceName      string              `bson:"service_name"        yaml:"service_name"     json:"service_name"`
	ServiceModule    string              `bson:"service_module"      yaml:"service_module"   json:"service_module"`
	BuildName        string              `bson:"build_name"          yaml:"build_name"       json:"build_name"`
	Image            string              `bson:"-"                   yaml:"-"                json:"image"`
	Package          string              `bson:"-"                   yaml:"-"                json:"package"`
	KeyVals          []*KeyVal           `bson:"key_vals"            yaml:"key_vals"         json:"key_vals"`
	Repos            []*types.Repository `bson:"repos"               yaml:"repos"            json:"repos"`
	ShareStorageInfo *ShareStorageInfo   `bson:"share_storage_info"   yaml:"share_storage_info"   json:"share_storage_info"`
}

type ZadigDeployJobSpec struct {
	Env                string `bson:"env"                      yaml:"env"                         json:"env"`
	DeployType         string `bson:"deploy_type"              yaml:"-"                           json:"deploy_type"`
	SkipCheckRunStatus bool   `bson:"skip_check_run_status"    yaml:"skip_check_run_status"       json:"skip_check_run_status"`
	// fromjob/runtime, runtime 表示运行时输入，fromjob 表示从上游构建任务中获取
	Source config.DeploySourceType `bson:"source"     yaml:"source"     json:"source"`
	// 当 source 为 fromjob 时需要，指定部署镜像来源是上游哪一个构建任务
	JobName          string             `bson:"job_name"             yaml:"job_name"             json:"job_name"`
	ServiceAndImages []*ServiceAndImage `bson:"service_and_images"   yaml:"service_and_images"   json:"service_and_images"`
}

type ServiceAndImage struct {
	ServiceName   string `bson:"service_name"        yaml:"service_name"     json:"service_name"`
	ServiceModule string `bson:"service_module"      yaml:"service_module"   json:"service_module"`
	Image         string `bson:"image"               yaml:"image"            json:"image"`
}

type ZadigDistributeImageJobSpec struct {
	// fromjob/runtime, `runtime` means runtime input, `fromjob` means that it is obtained from the upstream build job
	Source config.DeploySourceType `bson:"source"     yaml:"source"     json:"source"`
	// required when source is `fromjob`, specify which upstream build job the distribution image source is
	JobName string `bson:"job_name"                       json:"job_name"                      yaml:"job_name"`
	// not required when source is fromjob, directly obtained from upstream build job information
	SourceRegistryID string              `bson:"source_registry_id"             json:"source_registry_id"            yaml:"source_registry_id"`
	TargetRegistryID string              `bson:"target_registry_id"             json:"target_registry_id"            yaml:"target_registry_id"`
	Tatgets          []*DistributeTarget `bson:"targets"                        json:"targets"                       yaml:"targets"`
	// unit is minute.
	Timeout   int64  `bson:"timeout"                        json:"timeout"                       yaml:"timeout"`
	ClusterID string `bson:"cluster_id"                     json:"cluster_id"                    yaml:"cluster_id"`
}

type DistributeTarget struct {
	ServiceName   string `bson:"service_name"              yaml:"service_name"               json:"service_name"`
	ServiceModule string `bson:"service_module"            yaml:"service_module"             json:"service_module"`
	SourceTag     string `bson:"source_tag,omitempty"      yaml:"source_tag,omitempty"       json:"source_tag,omitempty"`
	TargetTag     string `bson:"target_tag,omitempty"      yaml:"target_tag,omitempty"       json:"target_tag,omitempty"`
	SourceImage   string `bson:"source_image,omitempty"    yaml:"-"                          json:"source_image,omitempty"`
	TargetImage   string `bson:"target_image,omitempty"    yaml:"-"                          json:"target_image,omitempty"`
	// if UpdateTag was false, use SourceTag as TargetTag.
	UpdateTag bool `bson:"update_tag"                yaml:"update_tag"                json:"update_tag"`
}

type ZadigTestingJobSpec struct {
	TestModules []*TestModule `bson:"test_modules"     yaml:"test_modules"     json:"test_modules"`
}

type TestModule struct {
	Name             string              `bson:"name"                yaml:"name"             json:"name"`
	ProjectName      string              `bson:"project_name"        yaml:"project_name"     json:"project_name"`
	KeyVals          []*KeyVal           `bson:"key_vals"            yaml:"key_vals"         json:"key_vals"`
	Repos            []*types.Repository `bson:"repos"               yaml:"repos"            json:"repos"`
	ShareStorageInfo *ShareStorageInfo   `bson:"share_storage_info"   yaml:"share_storage_info"   json:"share_storage_info"`
}

type ZadigScanningJobSpec struct {
	Scannings []*ScanningModule `bson:"scannings"     yaml:"scannings"     json:"scannings"`
}

type ScanningModule struct {
	Name             string              `bson:"name"                yaml:"name"             json:"name"`
	ProjectName      string              `bson:"project_name"        yaml:"project_name"     json:"project_name"`
	Repos            []*types.Repository `bson:"repos"               yaml:"repos"            json:"repos"`
	ShareStorageInfo *ShareStorageInfo   `bson:"share_storage_info"   yaml:"share_storage_info"   json:"share_storage_info"`
}

type BlueGreenDeployJobSpec struct {
	ClusterID        string             `bson:"cluster_id"             json:"cluster_id"            yaml:"cluster_id"`
	Namespace        string             `bson:"namespace"              json:"namespace"             yaml:"namespace"`
	DockerRegistryID string             `bson:"docker_registry_id"     json:"docker_registry_id"    yaml:"docker_registry_id"`
	Targets          []*BlueGreenTarget `bson:"targets"                json:"targets"               yaml:"targets"`
}

type BlueGreenReleaseJobSpec struct {
	FromJob string `bson:"from_job"               json:"from_job"              yaml:"from_job"`
}

type BlueGreenTarget struct {
	K8sServiceName     string `bson:"k8s_service_name"       json:"k8s_service_name"      yaml:"k8s_service_name"`
	BlueK8sServiceName string `bson:"blue_k8s_service_name"  json:"blue_k8s_service_name" yaml:"-"`
	ContainerName      string `bson:"container_name"         json:"container_name"        yaml:"container_name"`
	Image              string `bson:"image"                  json:"image"                 yaml:"image"`
	// unit is minute.
	DeployTimeout    int64  `bson:"deploy_timeout"         json:"deploy_timeout"        yaml:"deploy_timeout"`
	WorkloadName     string `bson:"workload_name"          json:"workload_name"         yaml:"workload_name"`
	BlueWorkloadName string `bson:"blue_workload_name"     json:"blue_workload_name"    yaml:"-"`
	WorkloadType     string `bson:"workload_type"          json:"workload_type"         yaml:"workload_type"`
	Version          string `bson:"version"                json:"version"               yaml:"-"`
}

type CanaryDeployJobSpec struct {
	ClusterID        string          `bson:"cluster_id"             json:"cluster_id"            yaml:"cluster_id"`
	Namespace        string          `bson:"namespace"              json:"namespace"             yaml:"namespace"`
	DockerRegistryID string          `bson:"docker_registry_id"     json:"docker_registry_id"    yaml:"docker_registry_id"`
	Targets          []*CanaryTarget `bson:"targets"                json:"targets"               yaml:"targets"`
}

type CanaryReleaseJobSpec struct {
	FromJob string `bson:"from_job"               json:"from_job"              yaml:"from_job"`
	// unit is minute.
	ReleaseTimeout int64 `bson:"release_timeout"        json:"release_timeout"       yaml:"release_timeout"`
}

type CanaryTarget struct {
	K8sServiceName   string `bson:"k8s_service_name"       json:"k8s_service_name"      yaml:"k8s_service_name"`
	ContainerName    string `bson:"container_name"         json:"container_name"        yaml:"container_name"`
	Image            string `bson:"image"                  json:"image"                 yaml:"image"`
	CanaryPercentage int    `bson:"canary_percentage"      json:"canary_percentage"     yaml:"canary_percentage"`
	// unit is minute.
	DeployTimeout int64  `bson:"deploy_timeout"         json:"deploy_timeout"        yaml:"deploy_timeout"`
	WorkloadName  string `bson:"workload_name"          json:"workload_name"         yaml:"workload_name"`
	WorkloadType  string `bson:"workload_type"          json:"workload_type"         yaml:"workload_type"`
}

type GrayReleaseJobSpec struct {
	ClusterID        string `bson:"cluster_id"             json:"cluster_id"            yaml:"cluster_id"`
	Namespace        string `bson:"namespace"              json:"namespace"             yaml:"namespace"`
	DockerRegistryID string `bson:"docker_registry_id"     json:"docker_registry_id"    yaml:"docker_registry_id"`
	FromJob          string `bson:"from_job"               json:"from_job"              yaml:"from_job"`
	// unit is minute.
	DeployTimeout int64                `bson:"deploy_timeout"         json:"deploy_timeout"        yaml:"deploy_timeout"`
	GrayScale     int                  `bson:"gray_scale"             json:"gray_scale"            yaml:"gray_scale"`
	Targets       []*GrayReleaseTarget `bson:"targets"                json:"targets"               yaml:"targets"`
}

type GrayReleaseTarget struct {
	WorkloadType  string `bson:"workload_type"             json:"workload_type"            yaml:"workload_type"`
	WorkloadName  string `bson:"workload_name"             json:"workload_name"            yaml:"workload_name"`
	Replica       int    `bson:"replica,omitempty"         json:"replica,omitempty"        yaml:"replica,omitempty"`
	ContainerName string `bson:"container_name"            json:"container_name"           yaml:"container_name"`
	Image         string `bson:"image,omitempty"           json:"image,omitempty"          yaml:"image,omitempty"`
}

type K8sPatchJobSpec struct {
	ClusterID  string       `bson:"cluster_id"             json:"cluster_id"            yaml:"cluster_id"`
	Namespace  string       `bson:"namespace"              json:"namespace"             yaml:"namespace"`
	PatchItems []*PatchItem `bson:"patch_items"            json:"patch_items"           yaml:"patch_items"`
}

type PatchItem struct {
	ResourceName    string   `bson:"resource_name"                json:"resource_name"               yaml:"resource_name"`
	ResourceKind    string   `bson:"resource_kind"                json:"resource_kind"               yaml:"resource_kind"`
	ResourceGroup   string   `bson:"resource_group"               json:"resource_group"              yaml:"resource_group"`
	ResourceVersion string   `bson:"resource_version"             json:"resource_version"            yaml:"resource_version"`
	PatchContent    string   `bson:"patch_content"                json:"patch_content"               yaml:"patch_content"`
	Params          []*Param `bson:"params"                       json:"params"                      yaml:"params"`
	// support strategic-merge/merge/json
	PatchStrategy string `bson:"patch_strategy"          json:"patch_strategy"         yaml:"patch_strategy"`
}

type GrayRollbackJobSpec struct {
	ClusterID string `bson:"cluster_id"             json:"cluster_id"            yaml:"cluster_id"`
	Namespace string `bson:"namespace"              json:"namespace"             yaml:"namespace"`
	// unit is minute.
	RollbackTimeout int64                 `bson:"rollback_timeout"       json:"rollback_timeout"      yaml:"rollback_timeout"`
	Targets         []*GrayRollbackTarget `bson:"targets"                json:"targets"               yaml:"targets"`
}

type GrayRollbackTarget struct {
	WorkloadType  string `bson:"workload_type"             json:"workload_type"            yaml:"workload_type"`
	WorkloadName  string `bson:"workload_name"             json:"workload_name"            yaml:"workload_name"`
	OriginImage   string `bson:"-"                         json:"origin_image"             yaml:"origin_image,omitempty"`
	OriginReplica int    `bson:"-"                         json:"origin_replica"           yaml:"origin_replica,omitempty"`
}

type JiraJobSpec struct {
	ProjectID    string     `bson:"project_id"  json:"project_id"  yaml:"project_id"`
	IssueType    string     `bson:"issue_type"  json:"issue_type"  yaml:"issue_type"`
	Issues       []*IssueID `bson:"issues" json:"issues" yaml:"issues"`
	TargetStatus string     `bson:"target_status" json:"target_status" yaml:"target_status"`
	Source       string     `bson:"source" json:"source" yaml:"source"`
}

type IstioJobSpec struct {
	First             bool              `bson:"first"              json:"first"              yaml:"first"`
	ClusterID         string            `bson:"cluster_id"         json:"cluster_id"         yaml:"cluster_id"`
	FromJob           string            `bson:"from_job"           json:"from_job"           yaml:"from_job"`
	RegistryID        string            `bson:"registry_id"        json:"registry_id"        yaml:"registry_id"`
	Namespace         string            `bson:"namespace"          json:"namespace"          yaml:"namespace"`
	Timeout           int64             `bson:"timeout"            json:"timeout"            yaml:"timeout"`
	ReplicaPercentage int64             `bson:"replica_percentage" json:"replica_percentage" yaml:"replica_percentage"`
	Weight            int64             `bson:"weight"             json:"weight"             yaml:"weight"`
	Targets           []*IstioJobTarget `bson:"targets"            json:"targets"            yaml:"targets"`
}

type IstioRollBackJobSpec struct {
	ClusterID string            `bson:"cluster_id"  json:"cluster_id"  yaml:"cluster_id"`
	Namespace string            `bson:"namespace"   json:"namespace"   yaml:"namespace"`
	Timeout   int64             `bson:"timeout"     json:"timeout"     yaml:"timeout"`
	Targets   []*IstioJobTarget `bson:"targets"     json:"targets"     yaml:"targets"`
}

type ApolloJobSpec struct {
	ApolloID      string             `bson:"apolloID" json:"apolloID" yaml:"apolloID"`
	NamespaceList []*ApolloNamespace `bson:"namespaceList" json:"namespaceList" yaml:"namespaceList"`
}

type ApolloNamespace struct {
	AppID      string      `bson:"appID" json:"appID" yaml:"appID"`
	ClusterID  string      `bson:"clusterID" json:"clusterID" yaml:"clusterID"`
	Env        string      `bson:"env" json:"env" yaml:"env"`
	Namespace  string      `bson:"namespace" json:"namespace" yaml:"namespace"`
	Type       string      `bson:"type" json:"type" yaml:"type"`
	KeyValList []*ApolloKV `bson:"kv" json:"kv" yaml:"kv"`
}

type MeegoTransitionJobSpec struct {
	Source          string                     `bson:"source"             json:"source"`
	ProjectKey      string                     `bson:"project_key"        json:"project_key"        yaml:"project_key"`
	ProjectName     string                     `bson:"project_name"       json:"project_name"       yaml:"project_name"`
	WorkItemType    string                     `bson:"work_item_type"     json:"work_item_type"     yaml:"work_item_type"`
	WorkItemTypeKey string                     `bson:"work_item_type_key" json:"work_item_type_key" yaml:"work_item_type_key"`
	Link            string                     `bson:"link"               json:"link"               yaml:"link"`
	WorkItems       []*MeegoWorkItemTransition `bson:"work_items"         json:"work_items"         yaml:"work_items"`
}

type MeegoWorkItemTransition struct {
	ID             int    `bson:"id"               json:"id"               yaml:"id"`
	Name           string `bson:"name"             json:"name"             yaml:"name"`
	TransitionID   int64  `bson:"transition_id"    json:"transition_id"    yaml:"transition_id"`
	TargetStateKey string `bson:"target_state_key" json:"target_state_key" yaml:"target_state_key"`
	Status         string `bson:"status"           json:"status"           yaml:"status,omitempty"`
}

type IstioJobTarget struct {
	WorkloadName       string `bson:"workload_name"             json:"workload_name"             yaml:"workload_name"`
	ContainerName      string `bson:"container_name"            json:"container_name"            yaml:"container_name"`
	VirtualServiceName string `bson:"virtual_service_name"      json:"virtual_service_name"      yaml:"virtual_service_name"`
	Host               string `bson:"host"                      json:"host"                      yaml:"host"`
	Image              string `bson:"image"                     json:"image"                     yaml:"-"`
	CurrentReplica     int    `bson:"current_replica,omitempty" json:"current_replica,omitempty" yaml:"-"`
	TargetReplica      int    `bson:"target_replica,omitempty"  json:"target_replica,omitempty"  yaml:"-"`
}

type NacosJobSpec struct {
	NacosID     string               `bson:"nacos_id"         json:"nacos_id"         yaml:"nacos_id"`
	NamespaceID string               `bson:"namespace_id"     json:"namespace_id"     yaml:"namespace_id"`
	NacosDatas  []*types.NacosConfig `bson:"nacos_datas"      json:"nacos_datas"      yaml:"nacos_datas"`
	DataFixed   bool                 `bson:"data_fixed"       json:"data_fixed"       yaml:"data_fixed"`
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
	CustomEnvs          []*KeyVal            `bson:"custom_envs"            json:"custom_envs"           yaml:"custom_envs,omitempty"`
	Params              []*Param             `bson:"params"                 json:"params"                yaml:"params"`
	Paths               string               `bson:"-"                      json:"-"                     yaml:"-"`
	LogFileName         string               `bson:"log_file_name"          json:"log_file_name"         yaml:"log_file_name"`
	DockerHost          string               `bson:"-"                      json:"docker_host,omitempty" yaml:"docker_host,omitempty"`
	Registries          []*RegistryNamespace `bson:"registries"             json:"registries"            yaml:"registries"`
	Cache               types.Cache          `bson:"cache"                  json:"cache"                 yaml:"cache"`
	CacheEnable         bool                 `bson:"cache_enable"           json:"cache_enable"          yaml:"cache_enable"`
	CacheDirType        types.CacheDirType   `bson:"cache_dir_type"         json:"cache_dir_type"        yaml:"cache_dir_type"`
	CacheUserDir        string               `bson:"cache_user_dir"         json:"cache_user_dir"        yaml:"cache_user_dir"`
	ShareStorageInfo    *ShareStorageInfo    `bson:"share_storage_info"     json:"share_storage_info"    yaml:"share_storage_info"`
	ShareStorageDetails []*StorageDetail     `bson:"share_storage_details"  json:"share_storage_details" yaml:"-"`
	UseHostDockerDaemon bool                 `bson:"use_host_docker_daemon,omitempty" json:"use_host_docker_daemon,omitempty" yaml:"use_host_docker_daemon"`
}

type Step struct {
	Name     string          `bson:"name"           json:"name"             yaml:"name"`
	Timeout  int64           `bson:"timeout"        json:"timeout"          yaml:"timeout"`
	StepType config.StepType `bson:"type"           json:"type"             yaml:"type"`
	Spec     interface{}     `bson:"spec"           json:"spec"             yaml:"spec"`
}

type Output struct {
	Name        string `bson:"name"           json:"name"             yaml:"name"`
	Description string `bson:"description"    json:"description"      yaml:"description"`
}

type WorkflowV4Hook struct {
	Name                string              `bson:"name"                      json:"name"`
	AutoCancel          bool                `bson:"auto_cancel"               json:"auto_cancel"`
	CheckPatchSetChange bool                `bson:"check_patch_set_change"    json:"check_patch_set_change"`
	Enabled             bool                `bson:"enabled"                   json:"enabled"`
	MainRepo            *MainHookRepo       `bson:"main_repo"                 json:"main_repo"`
	Description         string              `bson:"description,omitempty"     json:"description,omitempty"`
	Repos               []*types.Repository `bson:"-"                         json:"repos,omitempty"`
	WorkflowArg         *WorkflowV4         `bson:"workflow_arg"              json:"workflow_arg"`
}

type JiraHook struct {
	Name        string      `bson:"name" json:"name"`
	Enabled     bool        `bson:"enabled" json:"enabled"`
	Description string      `bson:"description" json:"description"`
	WorkflowArg *WorkflowV4 `bson:"workflow_arg" json:"workflow_arg"`
}

type MeegoHook struct {
	Name        string      `bson:"name" json:"name"`
	Enabled     bool        `bson:"enabled" json:"enabled"`
	Description string      `bson:"description" json:"description"`
	WorkflowArg *WorkflowV4 `bson:"workflow_arg" json:"workflow_arg"`
}

type GeneralHook struct {
	Name        string      `bson:"name" json:"name"`
	Enabled     bool        `bson:"enabled" json:"enabled"`
	Description string      `bson:"description" json:"description"`
	WorkflowArg *WorkflowV4 `bson:"workflow_arg" json:"workflow_arg"`
}

type Param struct {
	Name        string `bson:"name"             json:"name"             yaml:"name"`
	Description string `bson:"description"      json:"description"      yaml:"description"`
	// support string/text/choice type
	ParamsType   string   `bson:"type"                      json:"type"                        yaml:"type"`
	Value        string   `bson:"value"                     json:"value"                       yaml:"value,omitempty"`
	ChoiceOption []string `bson:"choice_option,omitempty"   json:"choice_option,omitempty"     yaml:"choice_option,omitempty"`
	Default      string   `bson:"default"                   json:"default"                     yaml:"default"`
	IsCredential bool     `bson:"is_credential"             json:"is_credential"               yaml:"is_credential"`
}

type ShareStorage struct {
	Name string `bson:"name"             json:"name"             yaml:"name"`
	Path string `bson:"path"             json:"path"             yaml:"path"`
}

type ShareStorageInfo struct {
	Enabled       bool            `bson:"enabled"             json:"enabled"             yaml:"enabled"`
	ShareStorages []*ShareStorage `bson:"share_storages"      json:"share_storages"      yaml:"share_storages"`
}

type StorageDetail struct {
	Type      types.MediumType `bson:"type"             json:"type"             yaml:"type"`
	Name      string           `bson:"name"             json:"name"             yaml:"name"`
	SubPath   string           `bson:"sub_path"         json:"sub_path"         yaml:"sub_path"`
	MountPath string           `bson:"mount_path"       json:"mount_path"       yaml:"mount_path"`
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
