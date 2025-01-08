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

package service

import (
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/setting"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/lark"
	"github.com/koderover/zadig/v2/pkg/types"
)

// ScheduleType 触发模式
type ScheduleType string

const (
	// TimingSchedule 定时循环
	TimingSchedule ScheduleType = "timing"
	// GapSchedule 间隔循环
	GapSchedule ScheduleType = "gap"
	// UnixstampSchedule 时间戳定时
	UnixstampSchedule ScheduleType = "unix_stamp"
)

type PipelineResource struct {
	Version         string           `bson:"version" json:"version"`
	Kind            string           `bson:"kind" json:"kind"`
	Metadata        PipelineMetadata `bson:"metadata" json:"metadata"`
	Spec            PipelineSpec     `bson:"spec" json:"spec"`
	LastExecuteTime *time.Time       `json:"lastExecuteTime,omitempty" bson:"lastExecuteTime"`
	HookPayload     *HookPayload     `bson:"hook_payload" json:"hook_payload"`

	Statistic      *PipelineStatistic `json:"statistic,omitempty" bson:"-"`
	NotificationID string             `bson:"notification_id" json:"notification_id"`
}

type PipelineMetadata struct {
	Name             string `bson:"name" json:"name"`
	Project          string `bson:"project" json:"project"`
	ProjectID        string `bson:"projectId" json:"projectId"`
	Revision         int    `bson:"revision" json:"revision"`
	AccountID        string `bson:"accountId" json:"accountId"`
	OriginYamlString string `bson:"originYamlString" json:"originYamlString"`
	ID               string `bson:"id" json:"id"`

	CreatedAt string `bson:"created_at" json:"created_at"`
	UpdatedAt string `bson:"updated_at" json:"updated_at"`
}

type PipelineSpec struct {
	Schedules        []*Schedule       `bson:"schedules" json:"schedules"`
	Triggers         []PipelineTrigger `bson:"triggers" json:"triggers"`
	Steps            map[string]Dict   `bson:"steps" json:"steps"`
	Stages           []string          `bson:"stages" json:"stages"`
	Variables        []Variable        `bson:"variables" json:"variables"`
	Contexts         []interface{}     `bson:"contexts" json:"contexts"`
	Services         Dict              `bson:"services" json:"services"`
	ExtractVariables []string          `bson:"-" json:"extractVariables"`
	StepNames        []string          `bson:"-" json:"-" `
	OrderedSteps     []Dict            `bson:"-" json:"-" `
}

type Schedule struct {
	ID              primitive.ObjectID `bson:"_id,omitempty"                 json:"id,omitempty"`
	Number          uint64             `bson:"number"                        json:"number"`
	UnixStamp       int64              `bson:"unix_stamp"                    json:"unix_stamp"`
	Frequency       string             `bson:"frequency"                     json:"frequency"`
	Time            string             `bson:"time"                          json:"time"`
	MaxFailures     int                `bson:"max_failures,omitempty"        json:"max_failures,omitempty"`
	TaskArgs        *TaskArgs          `bson:"task_args,omitempty"           json:"task_args,omitempty"`
	WorkflowArgs    *WorkflowTaskArgs  `bson:"workflow_args,omitempty"       json:"workflow_args,omitempty"`
	TestArgs        *TestTaskArgs      `bson:"test_args,omitempty"           json:"test_args,omitempty"`
	WorkflowV4Args  *WorkflowV4        `bson:"workflow_v4_args"              json:"workflow_v4_args"`
	EnvAnalysisArgs *EnvArgs           `bson:"env_analysis_args,omitempty"   json:"env_analysis_args,omitempty"`
	EnvArgs         *EnvArgs           `bson:"env_args,omitempty"            json:"env_args,omitempty"`
	ReleasePlanArgs *ReleasePlanArgs   `bson:"release_plan_args,omitempty"   json:"release_plan_args,omitempty"`
	Type            ScheduleType       `bson:"type"                          json:"type"`
	Cron            string             `bson:"cron"                          json:"cron"`
	IsModified      bool               `bson:"-"                             json:"-"`
	// 自由编排工作流的开关是放在schedule里面的
	Enabled bool `bson:"enabled"                       json:"enabled"`
}

// Validate validate schedule setting
func (schedule *Schedule) Validate() error {
	switch schedule.Type {
	case TimingSchedule:
		// 默认间隔循环间隔是1
		if schedule.Number <= 0 {
			schedule.Number = 1
		}
		if err := isValidJobTime(schedule.Time); err != nil {
			return err
		}
		if schedule.Frequency != setting.FrequencyDay &&
			schedule.Frequency != setting.FrequencyMondy &&
			schedule.Frequency != setting.FrequencyTuesday &&
			schedule.Frequency != setting.FrequencyWednesday &&
			schedule.Frequency != setting.FrequencyThursday &&
			schedule.Frequency != setting.FrequencyFriday &&
			schedule.Frequency != setting.FrequencySaturday &&
			schedule.Frequency != setting.FrequencySunday {
			return fmt.Errorf("%s 定时任务频率错误", e.InvalidFormatErrMsg)
		}
		return nil

	case GapSchedule:
		if schedule.Frequency != setting.FrequencyHours &&
			schedule.Frequency != setting.FrequencyMinutes {
			return fmt.Errorf("%s 间隔任务频率错误", e.InvalidFormatErrMsg)
		}
		if schedule.Number <= 0 {
			return fmt.Errorf("%s 间隔循环时间间隔不能小于等于0", e.InvalidFormatErrMsg)
		}
		//if schedule.Frequency == FrequencyMinutes && schedule.Number < 1 {
		//	log.Info("minimum schedule minutes must >= 30")
		//	return fmt.Errorf("%s 定时任务最短为30分钟", e.InvalidFormatErrMsg)
		//
		//}
		return nil

	default:
		return fmt.Errorf("%s 间隔任务模式未设置", e.InvalidFormatErrMsg)
	}
}

func isValidJobTime(t string) error {

	hour := int((t[0]-'0')*10 + (t[1] - '0'))
	min := int((t[3]-'0')*10 + (t[4] - '0'))

	if hour < 0 || hour > 23 || min < 0 || min > 59 {
		return fmt.Errorf("%s 时间范围: 小时[0-24] 分钟[0-59]", e.InvalidFormatErrMsg)
	}

	return nil
}

type PipelineTrigger struct {
	Owner        string   `bson:"repo_owner"               json:"repo_owner"`
	Repo         string   `bson:"repo"                     json:"repo"`
	Branch       string   `bson:"branch"                   json:"branch"`
	Events       []string `bson:"events"                   json:"events"`
	Disabled     bool     `bson:"disabled"                 json:"disabled"`
	Provider     string   `bson:"provider"                 json:"provider"`
	CodehostID   int      `bson:"codehost_id"              json:"codehost_id"`
	MatchFolders []string `bson:"match_folders"            json:"match_folders"`
}

type Variable struct {
	Key       string `bson:"key" json:"key"`
	Value     string `bson:"value" json:"value"`
	Encrypted bool   `bson:"encrypted" json:"encrypted"`
}

type Dict map[string]interface{}

// TaskArgs 单服务工作流任务参数
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
	NotificationID string        `bson:"notification_id"         json:"notification_id"`
}

type Repository struct {
	// Source is github, gitlab
	Source        string `bson:"source,omitempty"          json:"source,omitempty"`
	RepoOwner     string `bson:"repo_owner"                json:"repo_owner"`
	RepoName      string `bson:"repo_name"                 json:"repo_name"`
	RemoteName    string `bson:"remote_name,omitempty"     json:"remote_name,omitempty"`
	Branch        string `bson:"branch"                    json:"branch"`
	PR            int    `bson:"pr,omitempty"              json:"pr,omitempty"`
	Tag           string `bson:"tag,omitempty"             json:"tag,omitempty"`
	CommitID      string `bson:"commit_id,omitempty"       json:"commit_id,omitempty"`
	CommitMessage string `bson:"commit_message,omitempty"  json:"commit_message,omitempty"`
	CheckoutPath  string `bson:"checkout_path,omitempty"   json:"checkout_path,omitempty"`
	SubModules    bool   `bson:"submodules,omitempty"      json:"submodules,omitempty"`
	// Hidden defines whether the frontend needs to hide this repo
	Hidden bool `bson:"hidden" json:"hidden"`
	// UseDefault defines if the repo can be configured in start pipeline task page
	UseDefault bool `bson:"use_default,omitempty"          json:"use_default,omitempty"`
	// IsPrimary used to generated image and package name, each build has one primary repo
	IsPrimary  bool `bson:"is_primary"                     json:"is_primary"`
	CodehostID int  `bson:"codehost_id"                    json:"codehost_id"`
	// add
	OauthToken  string `bson:"oauth_token"                  json:"oauth_token"`
	Address     string `bson:"address"                      json:"address"`
	AuthorName  string `bson:"author_name,omitempty"        json:"author_name,omitempty"`
	CheckoutRef string `bson:"checkout_ref,omitempty"       json:"checkout_ref,omitempty"`
}

type KeyVal struct {
	Key          string `bson:"key"                 json:"key"`
	Value        string `bson:"value"               json:"value"`
	IsCredential bool   `bson:"is_credential"       json:"is_credential"`
}

type DeployArgs struct {
	// 目标部署环境
	Namespace string `json:"namespace"`
	// 镜像或者二进制名称后缀, 一般为branch或者PR
	Tag string `json:"suffix"`
	// 部署镜像名称
	// 格式: registy.xxx.com/{namespace}/{service name}:{timestamp}-{suffix}}
	// timestamp format: 20060102150405
	Image string `json:"image"`
	// 部署二进制包名称
	// 格式: {service name}-{timestamp}-{suffix}}.tar.gz
	// timestamp format: 20060102150405
	PackageFile string `json:"package_file"`
}

// TestArgs ...
type TestArgs struct {
	Namespace      string        `bson:"namespace" json:"namespace"`
	TestModuleName string        `bson:"test_module_name" json:"test_module_name"`
	Envs           []*KeyVal     `bson:"envs" json:"envs"`
	Builds         []*Repository `bson:"builds" json:"builds"`
}

// HookPayload ...
type HookPayload struct {
	Owner      string `bson:"owner"          json:"owner,omitempty"`
	Repo       string `bson:"repo"           json:"repo,omitempty"`
	Branch     string `bson:"branch"         json:"branch,omitempty"`
	Ref        string `bson:"ref"            json:"ref,omitempty"`
	IsPr       bool   `bson:"is_pr"          json:"is_pr,omitempty"`
	CheckRunID int64  `bson:"check_run_id"   json:"check_run_id,omitempty"`
	DeliveryID string `bson:"delivery_id"    json:"delivery_id,omitempty"`
}

// WorkflowTaskArgs 多服务工作流任务参数
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
	StorageID          string          `bson:"storage_id,omitempty"         json:"storage_id,omitempty"`
	DistributeEnabled  bool            `bson:"distribute_enabled"           json:"distribute_enabled"`
	WorklowTaskCreator string          `bson:"workflow_task_creator"        json:"workflow_task_creator"`
	// Ignore docker build cache
	IgnoreCache bool `json:"ignore_cache" bson:"ignore_cache"`
	// Ignore workspace cache and reset volume
	ResetCache bool `json:"reset_cache" bson:"reset_cache"`

	// NotificationID is the id of scmnotify.Notification
	NotificationID string `bson:"notification_id" json:"notification_id"`

	// webhook触发工作流任务时，触发任务的repo信息、prID和commitID
	MergeRequestID string `bson:"merge_request_id" json:"merge_request_id"`
	CommitID       string `bson:"commit_id"        json:"commit_id"`
	Source         string `bson:"source"           json:"source"`
	CodehostID     int    `bson:"codehost_id"      json:"codehost_id"`
	RepoOwner      string `bson:"repo_owner"       json:"repo_owner"`
	RepoNamespace  string `bson:"repo_namespace"   json:"repo_namespace"`
	RepoName       string `bson:"repo_name"        json:"repo_name"`

	//github check run
	HookPayload *HookPayload `bson:"hook_payload"            json:"hook_payload,omitempty"`
	// 请求模式，openAPI表示外部客户调用
	RequestMode string `json:"request_mode,omitempty"`
	IsParallel  bool   `json:"is_parallel" bson:"is_parallel"`
}

type TargetArgs struct {
	Name             string            `bson:"name"                          json:"name"`
	ImageName        string            `bson:"image_name"                    json:"image_name"`
	ServiceName      string            `bson:"service_name"                  json:"service_name"`
	ServiceType      string            `bson:"service_type,omitempty"        json:"service_type,omitempty"`
	ProductName      string            `bson:"product_name"                  json:"product_name"`
	Build            *BuildArgs        `bson:"build"                         json:"build"`
	Deploy           []DeployEnv       `bson:"deploy"                        json:"deploy"`
	Image            string            `bson:"image,omitempty"               json:"image,omitempty"`
	BinFile          string            `bson:"bin_file"                      json:"bin_file"`
	Envs             []*KeyVal         `bson:"envs"                          json:"envs"`
	HasBuild         bool              `bson:"has_build"                     json:"has_build"`
	JenkinsBuildArgs *JenkinsBuildArgs `bson:"jenkins_build_args,omitempty"  json:"jenkins_build_args,omitempty"`
}

type JenkinsBuildArgs struct {
	JobName            string                     `bson:"job_name"            json:"job_name"`
	JenkinsBuildParams []*types.JenkinsBuildParam `bson:"jenkins_build_param" json:"jenkins_build_params"`
}

type BuildArgs struct {
	Repos []*Repository `bson:"repos"               json:"repos"`
}

type ArtifactArgs struct {
	Name        string      `bson:"name"                      json:"name"`
	ImageName   string      `bson:"image_name,omitempty"      json:"image_name,omitempty"`
	ServiceName string      `bson:"service_name"              json:"service_name"`
	Image       string      `bson:"image"                     json:"image"`
	Deploy      []DeployEnv `bson:"deloy"                     json:"deploy"`
}

type DeployEnv struct {
	Env         string `json:"env"`
	Type        string `json:"type"`
	ProductName string `json:"product_name,omitempty"`
}

type PipelineStatistic struct {
	AvgTime      int                `json:"avgTime" bson:"avgTime"`
	MaxTime      int                `json:"maxTime" bson:"maxTime"`
	Workflows    []*WorkflowSummary `json:"workflows" bson:"workflows"`
	SuccessCount int                `json:"successCount" bson:"successCount"`
	ID           string             `json:"-" bson:"_id"`
}

type WorkflowSummary struct {
	Status   string    `json:"status" bson:"status"`
	ID       string    `json:"id" bson:"id"`
	Created  time.Time `json:"created" bson:"created"`
	Finished time.Time `json:"finished" bson:"finished"`
}

type VersionArgs struct {
	Enabled bool     `bson:"enabled" json:"enabled"`
	Version string   `bson:"version" json:"version"`
	Desc    string   `bson:"desc"    json:"desc"`
	Labels  []string `bson:"labels"  json:"labels"`
}

type TestTaskArgs struct {
	ProductName     string `bson:"product_name"            json:"product_name"`
	TestName        string `bson:"test_name"               json:"test_name"`
	TestTaskCreator string `bson:"test_task_creator"       json:"test_task_creator"`
	NotificationID  string `bson:"notification_id"         json:"notification_id"`
	ReqID           string `bson:"req_id"                  json:"req_id"`
	// webhook触发测试任务时，触发任务的repo、prID和commitID
	MergeRequestID string `bson:"merge_request_id" json:"merge_request_id"`
	CommitID       string `bson:"commit_id"        json:"commit_id"`
	Source         string `bson:"source"           json:"source"`
	CodehostID     int    `bson:"codehost_id"      json:"codehost_id"`
	RepoOwner      string `bson:"repo_owner"       json:"repo_owner"`
	RepoName       string `bson:"repo_name"        json:"repo_name"`
}

type EnvArgs struct {
	Name        string `bson:"name"                   json:"name"`
	ProductName string `bson:"product_name"           json:"product_name"`
	EnvName     string `bson:"env_name"               json:"env_name"`
	Production  bool   `bson:"production"             json:"production"`
}

type ReleasePlanArgs struct {
	ID    string `bson:"id"             json:"id"`
	Name  string `bson:"name"           json:"name"`
	Index int64  `bson:"index"          json:"index"`
}

type CreateBuildRequest struct {
	UserID       string     `json:"userId" bson:"userId"`
	UserName     string     `json:"userName" bson:"userName"`
	NoCache      bool       `json:"noCache" bson:"noCache"`
	ResetVolume  bool       `json:"cleanVolume" bson:"cleanVolume"`
	Variables    []Variable `json:"variables" bson:"variables"`
	PipelineName string     `json:"pipeline_name" bson:"pipeline_name"`
	ProductName  string     `json:"product_name" bson:"product_name"`
}

type DomainWebHookUser struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"   json:"id,omitempty"`
	Domain    string             `bson:"domain"          json:"domain"`
	UserCount int                `bson:"user_count"      json:"user_count"`
	CreatedAt int64              `bson:"created_at"      json:"created_at"`
}

type OperationRequest struct {
	Data string `json:"data"`
}

type PrivateKey struct {
	ID         primitive.ObjectID `bson:"_id,omitempty"          json:"id"`
	Name       string             `bson:"name"                   json:"name"`
	UserName   string             `bson:"user_name"              json:"user_name"`
	IP         string             `bson:"ip"                     json:"ip"`
	Label      string             `bson:"label"                  json:"label"`
	IsProd     bool               `bson:"is_prod"                json:"is_prod"`
	PrivateKey string             `bson:"private_key"            json:"private_key"`
	CreateTime int64              `bson:"create_time"            json:"create_time"`
	UpdateTime int64              `bson:"update_time"            json:"update_time"`
	UpdateBy   string             `bson:"update_by"              json:"update_by"`
}

type Container struct {
	Name  string `bson:"name"           json:"name"`
	Image string `bson:"image"          json:"image"`
}

type RenderInfo struct {
	Name        string `bson:"name"                     json:"name"`
	Revision    int64  `bson:"revision"                 json:"revision"`
	ProductTmpl string `bson:"product_tmpl"             json:"product_tmpl"`
	Description string `bson:"description"              json:"description"`
}

type RenderKV struct {
	Key      string   `bson:"key"               json:"key"`
	Value    string   `bson:"value"             json:"value"`
	Alias    string   `bson:"alias"             json:"alias"`
	State    KeyState `bson:"state"             json:"state"`
	Services []string `bson:"services"          json:"services"`
}

// KeyState can be new, deleted, normal
type KeyState string

type Pipeline struct {
	Name        string       `bson:"name"                         json:"name"`
	Type        PipelineType `bson:"type"                         json:"type"`
	Enabled     bool         `bson:"enabled"                      json:"enabled"`
	TeamName    string       `bson:"team"                         json:"team"`
	ProductName string       `bson:"product_name"                 json:"product_name"`
	// target 服务名称, k8s为容器名称, 物理机为服务名
	Target string `bson:"target"                    json:"target"`
	// 使用预定义编译管理模块中的内容生成SubTasks,
	// 查询条件为 服务名称: Target, 版本: BuildModuleVer
	// 如果为空，则使用pipeline自定义SubTasks
	BuildModuleVer string                    `bson:"build_module_ver"             json:"build_module_ver"`
	SubTasks       []*map[string]interface{} `bson:"sub_tasks"                    json:"sub_tasks"`
	Description    string                    `bson:"description,omitempty"        json:"description,omitempty"`
	UpdateBy       string                    `bson:"update_by"                    json:"update_by,omitempty"`
	CreateTime     int64                     `bson:"create_time"                  json:"create_time,omitempty"`
	UpdateTime     int64                     `bson:"update_time"                  json:"update_time,omitempty"`
	Schedules      ScheduleCtrl              `bson:"schedules,omitempty"          json:"schedules,omitempty"`
	Notifiers      []string                  `bson:"notifiers,omitempty"          json:"notifiers,omitempty"`
	RunCount       int                       `bson:"run_count,omitempty"          json:"run_count,omitempty"`
	DailyRunCount  float32                   `bson:"daily_run_count,omitempty"    json:"daily_run_count,omitempty"`
	PassRate       float32                   `bson:"pass_rate,omitempty"          json:"pass_rate,omitempty"`
	IsDeleted      bool                      `bson:"is_deleted"                   json:"is_deleted"`
}

// ScheduleCtrl ...
type ScheduleCtrl struct {
	Enabled bool        `bson:"enabled"    json:"enabled"`
	Items   []*Schedule `bson:"items"      json:"items"`
}

type WorkflowV4 struct {
	ID             primitive.ObjectID `bson:"_id,omitempty"       yaml:"-"            json:"id"`
	Name           string             `bson:"name"                yaml:"name"         json:"name"`
	DisplayName    string             `bson:"display_name"        yaml:"display_name" json:"display_name"`
	KeyVals        []*KeyVal          `bson:"key_vals"            yaml:"key_vals"     json:"key_vals"`
	Params         []*Param           `bson:"params"              yaml:"params"       json:"params"`
	Stages         []*WorkflowStage   `bson:"stages"              yaml:"stages"       json:"stages"`
	Project        string             `bson:"project"             yaml:"project"      json:"project"`
	Description    string             `bson:"description"         yaml:"description"  json:"description"`
	CreatedBy      string             `bson:"created_by"          yaml:"created_by"   json:"created_by"`
	CreateTime     int64              `bson:"create_time"         yaml:"create_time"  json:"create_time"`
	UpdatedBy      string             `bson:"updated_by"          yaml:"updated_by"   json:"updated_by"`
	UpdateTime     int64              `bson:"update_time"         yaml:"update_time"  json:"update_time"`
	MultiRun       bool               `bson:"multi_run"           yaml:"multi_run"    json:"multi_run"`
	NotificationID string             `bson:"notification_id"     yaml:"-"            json:"notification_id"`
	HookPayload    *HookPayload       `bson:"hook_payload"        yaml:"-"            json:"hook_payload,omitempty"`
	BaseName       string             `bson:"base_name"           yaml:"-"                   json:"base_name"`
	ShareStorages  []*ShareStorage    `bson:"share_storages"      yaml:"share_storages"      json:"share_storages"`
}

type ShareStorage struct {
	Name string `bson:"name"             json:"name"             yaml:"name"`
	Path string `bson:"path"             json:"path"             yaml:"path"`
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
	Timeout      int              `bson:"timeout"                     yaml:"timeout"                    json:"timeout"`
	ApprovalID   string           `bson:"approval_id"                 yaml:"approval_id"                json:"approval_id"`
	ApproveUsers []*lark.UserInfo `bson:"approve_users"               yaml:"approve_users"              json:"approve_users"`
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
	Skipped bool        `bson:"skipped"        yaml:"skipped"  json:"skipped"`
	Spec    interface{} `bson:"spec"           yaml:"spec"     json:"spec"`
}

type Param struct {
	Name        string `bson:"name"             json:"name"             yaml:"name"`
	Description string `bson:"description"      json:"description"      yaml:"description"`
	// support string/text type
	ParamsType   string   `bson:"type"                      json:"type"                        yaml:"type"`
	Value        string   `bson:"value"                     json:"value"                       yaml:"value,omitempty"`
	ChoiceOption []string `bson:"choice_option,omitempty"   json:"choice_option,omitempty"     yaml:"choice_option,omitempty"`
	Default      string   `bson:"default"                   json:"default"                     yaml:"default"`
	IsCredential bool     `bson:"is_credential"             json:"is_credential"               yaml:"is_credential"`
}

// Type pipeline type
type PipelineType string

const (
	// SingleType 单服务工作流
	SingleType = PipelineType("single")
	// WorkflowType 多服务工作流
	WorkflowType = PipelineType("workflow")
	// FreestyleType 自由编排工作流
	FreestyleType = PipelineType("freestyle")
	// TestType 测试
	TestType = PipelineType("test")
	// ServiceType 服务
	ServiceType = PipelineType("service")
)

type Workflow struct {
	Name      string        `json:"name"`
	Schedules *ScheduleCtrl `json:"schedules,omitempty"`
}
