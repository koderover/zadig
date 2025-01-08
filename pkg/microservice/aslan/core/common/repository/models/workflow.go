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
	"fmt"

	"github.com/koderover/zadig/v2/pkg/tool/lark"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/setting"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/types"
)

type Workflow struct {
	ID              primitive.ObjectID `bson:"_id,omitempty"                json:"id,omitempty"`
	Name            string             `bson:"name"                         json:"name"`
	DisplayName     string             `bson:"display_name"                 json:"display_name"`
	Enabled         bool               `bson:"enabled"                      json:"enabled"`
	ProductTmplName string             `bson:"product_tmpl_name"            json:"product_tmpl_name"`
	Team            string             `bson:"team,omitempty"               json:"team,omitempty"`
	EnvName         string             `bson:"env_name"                     json:"env_name,omitempty"`
	Description     string             `bson:"description,omitempty"        json:"description,omitempty"`
	UpdateBy        string             `bson:"update_by"                    json:"update_by,omitempty"`
	CreateBy        string             `bson:"create_by"                    json:"create_by,omitempty"`
	UpdateTime      int64              `bson:"update_time"                  json:"update_time"`
	CreateTime      int64              `bson:"create_time"                  json:"create_time"`
	Schedules       *ScheduleCtrl      `bson:"schedules,omitempty"          json:"schedules,omitempty"`
	ScheduleEnabled bool               `bson:"schedule_enabled"             json:"schedule_enabled"`
	Slack           *Slack             `bson:"slack,omitempty"              json:"slack,omitempty"`
	BuildStage      *BuildStage        `bson:"build_stage"                  json:"build_stage"`
	ArtifactStage   *ArtifactStage     `bson:"artifact_stage,omitempty"     json:"artifact_stage,omitempty"`
	TestStage       *TestStage         `bson:"test_stage"                   json:"test_stage"`
	SecurityStage   *SecurityStage     `bson:"security_stage"               json:"security_stage"`
	DistributeStage *DistributeStage   `bson:"distribute_stage"             json:"distribute_stage"`
	ExtensionStage  *ExtensionStage    `bson:"extension_stage"              json:"extension_stage"`
	// TODO: Deprecated.
	NotifyCtl *NotifyCtl `bson:"notify_ctl,omitempty"         json:"notify_ctl,omitempty"`
	// New since V1.12.0.
	NotifyCtls []*NotifyCtl      `bson:"notify_ctls"        json:"notify_ctls"`
	HookCtl    *WorkflowHookCtrl `bson:"hook_ctl"                     json:"hook_ctl"`
	BaseName   string            `bson:"base_name" json:"base_name"`

	// ResetImage indicate whether reset image to original version after completion
	ResetImage       bool                         `bson:"reset_image"                  json:"reset_image"`
	ResetImagePolicy setting.ResetImagePolicyType `bson:"reset_image_policy,omitempty" json:"reset_image_policy,omitempty"`
	// IsParallel 控制单一工作流的任务是否支持并行处理
	IsParallel bool `json:"is_parallel" bson:"is_parallel"`
}

type WorkflowHookCtrl struct {
	Enabled bool            `bson:"enabled" json:"enabled"`
	Items   []*WorkflowHook `bson:"items" json:"items"`
}

type WorkflowHook struct {
	AutoCancel          bool              `bson:"auto_cancel"               json:"auto_cancel"`
	CheckPatchSetChange bool              `bson:"check_patch_set_change"    json:"check_patch_set_change"`
	MainRepo            *MainHookRepo     `bson:"main_repo"                 json:"main_repo"`
	WorkflowArgs        *WorkflowTaskArgs `bson:"workflow_args"             json:"workflow_args"`
	IsYaml              bool              `bson:"is_yaml,omitempty"         json:"is_yaml,omitempty"`
	IsManual            bool              `bson:"is_manual,omitempty"       json:"is_manual,omitempty"`
	YamlPath            string            `bson:"yaml_path,omitempty"       json:"yaml_path,omitempty"`
}

type MainHookRepo struct {
	Name          string                 `bson:"name,omitempty"            json:"name,omitempty"`
	Description   string                 `bson:"description,omitempty"     json:"description,omitempty"`
	Source        string                 `bson:"source,omitempty"          json:"source,omitempty"`
	RepoOwner     string                 `bson:"repo_owner"                json:"repo_owner"`
	RepoNamespace string                 `bson:"repo_namespace"            json:"repo_namespace"`
	RepoName      string                 `bson:"repo_name"                 json:"repo_name"`
	Branch        string                 `bson:"branch"                    json:"branch"`
	Tag           string                 `bson:"tag"                       json:"tag"`
	Committer     string                 `bson:"committer"                 json:"committer"`
	MatchFolders  []string               `bson:"match_folders"             json:"match_folders,omitempty"`
	CodehostID    int                    `bson:"codehost_id"               json:"codehost_id"`
	Events        []config.HookEventType `bson:"events"                    json:"events"`
	Label         string                 `bson:"label"                     json:"label"`
	Revision      string                 `bson:"revision"                  json:"revision"`
	IsRegular     bool                   `bson:"is_regular"                json:"is_regular"`
}

func (m *MainHookRepo) GetRepoNamespace() string {
	if m.RepoNamespace != "" {
		return m.RepoNamespace
	}
	return m.RepoOwner
}

func (m MainHookRepo) GetLabelValue() string {
	if m.Label == "" {
		return "Code-Review"
	}

	return m.Label
}

type ScheduleCtrl struct {
	Enabled bool        `bson:"enabled"    json:"enabled"`
	Items   []*Schedule `bson:"items"      json:"items"`
}

type Schedule struct {
	ID              primitive.ObjectID  `bson:"_id,omitempty"                 json:"id,omitempty"`
	Number          uint64              `bson:"number"                        json:"number"`
	Frequency       string              `bson:"frequency"                     json:"frequency"`
	UnixStamp       int64               `bson:"unix_stamp"                    json:"unix_stamp"`
	Time            string              `bson:"time"                          json:"time"`
	MaxFailures     int                 `bson:"max_failures,omitempty"        json:"max_failures,omitempty"`
	TaskArgs        *TaskArgs           `bson:"task_args,omitempty"           json:"task_args,omitempty"`
	WorkflowArgs    *WorkflowTaskArgs   `bson:"workflow_args,omitempty"       json:"workflow_args,omitempty"`
	TestArgs        *TestTaskArgs       `bson:"test_args,omitempty"           json:"test_args,omitempty"`
	WorkflowV4Args  *WorkflowV4         `bson:"workflow_v4_args"              json:"workflow_v4_args"`
	EnvAnalysisArgs *EnvArgs            `bson:"env_analysis_args,omitempty"   json:"env_analysis_args,omitempty"`
	EnvArgs         *EnvArgs            `bson:"env_args,omitempty"            json:"env_args,omitempty"`
	ReleasePlanArgs *ReleasePlanArgs    `bson:"release_plan_args,omitempty"   json:"release_plan_args,omitempty"`
	Type            config.ScheduleType `bson:"type"                          json:"type"`
	Cron            string              `bson:"cron"                          json:"cron"`
	IsModified      bool                `bson:"-"                             json:"-"`
	// 自由编排工作流的开关是放在schedule里面的
	Enabled bool `bson:"enabled"                       json:"enabled"`
}

// TaskArgs 单服务工作流任务参数
type TaskArgs struct {
	ProductName    string              `bson:"product_name"            json:"product_name"`
	PipelineName   string              `bson:"pipeline_name"           json:"pipeline_name"`
	Builds         []*types.Repository `bson:"builds"                  json:"builds"`
	BuildArgs      []*KeyVal           `bson:"build_args"              json:"build_args"`
	Deploy         DeployArgs          `bson:"deploy"                  json:"deploy"`
	Test           TestArgs            `bson:"test"                    json:"test,omitempty"`
	HookPayload    *HookPayload        `bson:"hook_payload"            json:"hook_payload,omitempty"`
	TaskCreator    string              `bson:"task_creator"            json:"task_creator,omitempty"`
	ReqID          string              `bson:"req_id"                  json:"req_id"`
	IsQiNiu        bool                `bson:"is_qiniu"                json:"is_qiniu"`
	NotificationID string              `bson:"notification_id"         json:"notification_id"`
	CodeHostID     int                 `bson:"codehost_id"             json:"codehost_id"`
}

type CallbackArgs struct {
	CallbackUrl  string                 `bson:"callback_url" json:"callback_url"`   // url-encoded full path
	CallbackVars map[string]interface{} `bson:"callback_vars" json:"callback_vars"` // custom defied vars, will be set to body of callback request
}

// WorkflowTaskArgs 多服务工作流任务参数
type WorkflowTaskArgs struct {
	WorkflowName    string `bson:"workflow_name"                json:"workflow_name"`
	ProductTmplName string `bson:"product_tmpl_name"            json:"product_tmpl_name"`
	Description     string `bson:"description,omitempty"        json:"description,omitempty"`
	//为了兼容老数据，namespace可能会存多个环境名称，用逗号隔开
	Namespace           string          `bson:"namespace"                    json:"namespace"`
	BaseNamespace       string          `bson:"base_namespace,omitempty"     json:"base_namespace,omitempty"`
	EnvRecyclePolicy    string          `bson:"env_recycle_policy,omitempty" json:"env_recycle_policy,omitempty"`
	EnvUpdatePolicy     string          `bson:"env_update_policy,omitempty"  json:"env_update_policy,omitempty"`
	Target              []*TargetArgs   `bson:"targets"                      json:"targets"`
	Artifact            []*ArtifactArgs `bson:"artifact_args"                json:"artifact_args"`
	Tests               []*TestArgs     `bson:"tests"                        json:"tests"`
	VersionArgs         *VersionArgs    `bson:"version_args,omitempty"       json:"version_args,omitempty"`
	ReqID               string          `bson:"req_id"                       json:"req_id"`
	RegistryID          string          `bson:"registry_id,omitempty"        json:"registry_id,omitempty"`
	StorageID           string          `bson:"storage_id,omitempty"         json:"storage_id,omitempty"`
	DistributeEnabled   bool            `bson:"distribute_enabled"           json:"distribute_enabled"`
	WorkflowTaskCreator string          `bson:"workflow_task_creator"        json:"workflow_task_creator"`
	// Ignore docker build cache
	IgnoreCache bool `json:"ignore_cache" bson:"ignore_cache"`
	// Ignore workspace cache and reset volume
	ResetCache bool `json:"reset_cache" bson:"reset_cache"`

	// NotificationID is the id of scmnotify.Notification
	NotificationID string `bson:"notification_id" json:"notification_id"`

	// webhook触发工作流任务时，触发任务的repo信息、prID和commitID、分支信息
	MergeRequestID string `bson:"merge_request_id" json:"merge_request_id"`
	Ref            string `bson:"ref" json:"ref"`
	EventType      string `bson:"event_type" json:"event_type"`
	CommitID       string `bson:"commit_id"        json:"commit_id"`
	Source         string `bson:"source"           json:"source"`
	CodehostID     int    `bson:"codehost_id"      json:"codehost_id"`
	RepoOwner      string `bson:"repo_owner"       json:"repo_owner"`
	RepoNamespace  string `bson:"repo_namespace"   json:"repo_namespace"`
	RepoName       string `bson:"repo_name"        json:"repo_name"`
	Committer      string `bson:"committer,omitempty"        json:"committer,omitempty"`
	//github check run
	HookPayload *HookPayload `bson:"hook_payload"            json:"hook_payload,omitempty"`
	// 请求模式，openAPI表示外部客户调用
	RequestMode string `json:"request_mode,omitempty"`
	IsParallel  bool   `json:"is_parallel" bson:"is_parallel"`
	EnvName     string `json:"env_name" bson:"-"`

	Callback      *CallbackArgs   `bson:"callback"                    json:"callback"`
	ReleaseImages []*ReleaseImage `bson:"release_images,omitempty"    json:"release_images,omitempty"`
}

type ScanningArgs struct {
	ScanningName string `json:"scanning_name" bson:"scanning_name"`
	ScanningID   string `json:"scanning_id"   bson:"scanning_id"`

	// NotificationID is the id of scmnotify.Notification
	NotificationID string `bson:"notification_id" json:"notification_id"`
}

type ReleaseImage struct {
	Image         string `bson:"image"                    json:"image"`
	ServiceName   string `bson:"service_name"             json:"service_name"`
	ServiceModule string `bson:"service_module"           json:"service_module"`
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
	RepoNamespace  string `bson:"repo_namespace"   json:"repo_namespace"`
	RepoName       string `bson:"repo_name"        json:"repo_name"`
	Ref            string `bson:"ref" json:"ref"`
	Branch         string `bson:"branch" json:"branch"`
	EventType      string `bson:"event_type" json:"event_type"`
}

type Slack struct {
	Channel    string                 `bson:"channel"      json:"channel"`
	Enabled    bool                   `bson:"enabled"      json:"enabled"`
	Notifiers  []string               `bson:"notifiers"    json:"notifiers"`
	NotifyType config.SlackNotifyType `bson:"notify_type"  json:"notify_type"`
}

type BuildStage struct {
	Enabled bool           `bson:"enabled"                json:"enabled"`
	Modules []*BuildModule `bson:"modules"                json:"modules"`
}

// BuildModule ...
type BuildModule struct {
	Target            *ServiceModuleTarget      `bson:"target"                 json:"target"`
	HideServiceModule bool                      `bson:"hide_service_module"    json:"hide_service_module"`
	BuildModuleVer    string                    `bson:"build_module_ver"       json:"build_module_ver"`
	BranchFilter      []*types.BranchFilterInfo `bson:"branch_filter"          json:"branch_filter"`
}

type ArtifactStage struct {
	Enabled bool              `bson:"enabled"                json:"enabled"`
	Modules []*ArtifactModule `bson:"modules"                json:"modules"`
}

// ArtifactModule ...
type ArtifactModule struct {
	HideServiceModule bool                 `bson:"hide_service_module"    json:"hide_service_module"`
	Target            *ServiceModuleTarget `bson:"target"                 json:"target"`
}

type TestStage struct {
	Enabled   bool            `bson:"enabled"                    json:"enabled"`
	TestNames []string        `bson:"test_names,omitempty"       json:"test_names,omitempty"`
	Tests     []*TestExecArgs `bson:"tests,omitempty"     json:"tests,omitempty"`
}

// TestExecArgs ...
type TestExecArgs struct {
	Name    string    `bson:"test_name"             json:"test_name"`
	Envs    []*KeyVal `bson:"envs"                  json:"envs"`
	Project string    `bson:"project"               json:"project"`
}

type SecurityStage struct {
	Enabled bool `bson:"enabled"                    json:"enabled"`
}

type DistributeStage struct {
	Enabled     bool                 `bson:"enabled"              json:"enabled"`
	S3StorageID string               `bson:"s3_storage_id"        json:"s3_storage_id"`
	ImageRepo   string               `bson:"image_repo"           json:"image_repo"`
	JumpBoxHost string               `bson:"jump_box_host"        json:"jump_box_host"`
	Distributes []*ProductDistribute `bson:"distributes"          json:"distributes"`

	// repos to release images
	Releases []RepoImage `bson:"releases" json:"releases"`
}

type ExtensionStage struct {
	Enabled    bool      `bson:"enabled"              json:"enabled"`
	URL        string    `bson:"url"                  json:"url"`
	Path       string    `bson:"path"                 json:"path"`
	IsCallback bool      `bson:"is_callback"          json:"is_callback"`
	Timeout    int       `bson:"timeout"              json:"timeout"`
	Headers    []*KeyVal `bson:"headers"              json:"headers"`
}

type RepoImage struct {
	RepoID        string `json:"repo_id" bson:"repo_id"`
	Name          string `json:"name" bson:"name" yaml:"name"`
	Username      string `json:"-" yaml:"username"`
	Password      string `json:"-" yaml:"password"`
	Host          string `json:"host" yaml:"host"`
	Namespace     string `json:"namespace" yaml:"namespace"`
	DeployEnabled bool   `json:"deploy_enabled" yaml:"deploy_enabled"`
	DeployEnv     string `json:"deploy_env"   yaml:"deploy_env"`
}

type ProductDistribute struct {
	Target            *ServiceModuleTarget `bson:"target"                 json:"target"`
	ImageDistribute   bool                 `bson:"image_distribute"       json:"image_distribute"`
	JumpBoxDistribute bool                 `bson:"jump_box_distribute"    json:"jump_box_distribute"`
	QstackDistribute  bool                 `bson:"qstack_distribute"      json:"qstack_distribute"`
}

type NotifyCtl struct {
	Enabled         bool                      `bson:"enabled"                       yaml:"enabled"                       json:"enabled"`
	WebHookType     setting.NotifyWebHookType `bson:"webhook_type"                  yaml:"webhook_type"                  json:"webhook_type"`
	WeChatWebHook   string                    `bson:"weChat_webHook,omitempty"      yaml:"weChat_webHook,omitempty"      json:"weChat_webHook,omitempty"`
	DingDingWebHook string                    `bson:"dingding_webhook,omitempty"    yaml:"dingding_webhook,omitempty"    json:"dingding_webhook,omitempty"`
	FeiShuAppID     string                    `bson:"feishu_app_id,omitempty"       yaml:"feishu_app_id,omitempty"       json:"feishu_app_id,omitempty"`
	FeiShuWebHook   string                    `bson:"feishu_webhook,omitempty"      yaml:"feishu_webhook,omitempty"      json:"feishu_webhook,omitempty"`
	FeishuChat      *LarkChat                 `bson:"feishu_chat,omitempty"         yaml:"feishu_chat,omitempty"         json:"feishu_chat,omitempty"`
	MailUsers       []*User                   `bson:"mail_users,omitempty"          yaml:"mail_users,omitempty"          json:"mail_users,omitempty"`
	WebHookNotify   WebhookNotify             `bson:"webhook_notify,omitempty"      yaml:"webhook_notify,omitempty"      json:"webhook_notify,omitempty"`
	AtMobiles       []string                  `bson:"at_mobiles,omitempty"          yaml:"at_mobiles,omitempty"          json:"at_mobiles,omitempty"`
	WechatUserIDs   []string                  `bson:"wechat_user_ids,omitempty"     yaml:"wechat_user_ids,omitempty"     json:"wechat_user_ids,omitempty"`
	LarkUserIDs     []string                  `bson:"lark_user_ids,omitempty"       yaml:"lark_user_ids,omitempty"       json:"lark_user_ids,omitempty"`
	LarkAtUsers     []*lark.UserInfo          `bson:"lark_at_users"                 yaml:"lark_at_users"                 json:"lark_at_users"`
	IsAtAll         bool                      `bson:"is_at_all,omitempty"           yaml:"is_at_all,omitempty"           json:"is_at_all,omitempty"`
	NotifyTypes     []string                  `bson:"notify_type"                   yaml:"notify_type"                   json:"notify_type"`
}

type WebhookNotify struct {
	Address string `bson:"address"       yaml:"address"        json:"address"`
	Token   string `bson:"token"         yaml:"token"          json:"token"`
}

type TaskInfo struct {
	TaskID       int64         `bson:"task_id"               json:"task_id"`
	PipelineName string        `bson:"pipeline_name"         json:"pipeline_name"`
	Status       config.Status `bson:"status"                json:"status"`
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

type TestArgs struct {
	Namespace      string              `bson:"namespace" json:"namespace"`
	TestModuleName string              `bson:"test_module_name" json:"test_module_name"`
	Envs           []*KeyVal           `bson:"envs" json:"envs"`
	Builds         []*types.Repository `bson:"builds" json:"builds"`
}

type HookPayload struct {
	Owner          string `bson:"owner"            json:"owner,omitempty"`
	Repo           string `bson:"repo"             json:"repo,omitempty"`
	Branch         string `bson:"branch"           json:"branch,omitempty"`
	Ref            string `bson:"ref"              json:"ref,omitempty"`
	IsPr           bool   `bson:"is_pr"            json:"is_pr,omitempty"`
	CheckRunID     int64  `bson:"check_run_id"     json:"check_run_id,omitempty"`
	MergeRequestID string `bson:"merge_request_id" json:"merge_request_id,omitempty"`
	CommitID       string `bson:"commit_id"        json:"commit_id,omitempty"`
	DeliveryID     string `bson:"delivery_id"      json:"delivery_id,omitempty"`
	CodehostID     int    `bson:"codehost_id"      json:"codehost_id"`
	EventType      string `bson:"event_type"       json:"event_type"`
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
	BuildName        string            `bson:"build_name,omitempty"          json:"build_name"`
}

type JenkinsBuildArgs struct {
	JobName            string                     `bson:"job_name"            json:"job_name"`
	JenkinsBuildParams []*types.JenkinsBuildParam `bson:"jenkins_build_param" json:"jenkins_build_params"`
}

type BuildArgs struct {
	Repos []*types.Repository `bson:"repos"               json:"repos"`
}

type DeployEnv struct {
	Env         string `json:"env"`
	Type        string `json:"type"`
	ProductName string `json:"product_name,omitempty"`
}

type ArtifactArgs struct {
	Name         string      `bson:"name"                                json:"name"`
	ImageName    string      `bson:"image_name,omitempty"                json:"image_name,omitempty"`
	ServiceName  string      `bson:"service_name"                        json:"service_name"`
	Image        string      `bson:"image,omitempty"                     json:"image,omitempty"`
	Deploy       []DeployEnv `bson:"deploy"                              json:"deploy"`
	WorkflowName string      `bson:"workflow_name,omitempty"             json:"workflow_name,omitempty"`
	TaskID       int64       `bson:"task_id,omitempty"                   json:"task_id,omitempty"`
	FileName     string      `bson:"file_name,omitempty"                 json:"file_name,omitempty"`
	URL          string      `bson:"url,omitempty"                       json:"url,omitempty"`
}

type VersionArgs struct {
	Enabled bool     `bson:"enabled" json:"enabled"`
	Version string   `bson:"version" json:"version"`
	Desc    string   `bson:"desc"    json:"desc"`
	Labels  []string `bson:"labels"  json:"labels"`
}

type BuildModuleArgs struct {
	BuildName    string
	Target       string
	ServiceName  string
	ProductName  string
	Variables    []*KeyVal
	Env          *Product
	WorkflowName string
	TaskID       int64
	FileName     string
	URL          string
	TaskType     string
}

func (Workflow) TableName() string {
	return "workflow"
}

func (d *DistributeStage) IsDistributeS3Enabled() bool {
	if d != nil && d.Enabled {
		for _, d := range d.Distributes {
			if d.QstackDistribute {
				return true
			}
		}
	}

	return false
}

// Validate validate schedule setting
func (schedule *Schedule) Validate() error {
	switch schedule.Type {
	case config.TimingSchedule:
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

	case config.GapSchedule:
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
