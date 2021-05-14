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

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/koderover/zadig/lib/microservice/aslan/config"
	"github.com/koderover/zadig/lib/setting"
	e "github.com/koderover/zadig/lib/tool/errors"
	"github.com/koderover/zadig/lib/types"
)

type Workflow struct {
	ID              primitive.ObjectID `bson:"_id,omitempty"                json:"id,omitempty"`
	Name            string             `bson:"name"                         json:"name"`
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
	NotifyCtl       *NotifyCtl         `bson:"notify_ctl,omitempty"         json:"notify_ctl,omitempty"`
	HookCtl         *WorkflowHookCtrl  `bson:"hook_ctl"                     json:"hook_ctl"`
	IsFavorite      bool               `bson:"-"                            json:"is_favorite"`
	LastestTask     *TaskInfo          `bson:"-"                            json:"lastest_task"`
	LastSucessTask  *TaskInfo          `bson:"-"                            json:"last_task_success"`
	LastFailureTask *TaskInfo          `bson:"-"                            json:"last_task_failure"`
	TotalDuration   int64              `bson:"-"                            json:"total_duration"`
	TotalNum        int                `bson:"-"                            json:"total_num"`
	TotalSuccess    int                `bson:"-"                            json:"total_success"`

	// ResetImage indicate whether reset image to original version after completion
	ResetImage bool `json:"reset_image" bson:"reset_image"`
	// IsParallel 控制单一工作流的任务是否支持并行处理
	IsParallel bool `json:"is_parallel" bson:"is_parallel"`
}

type WorkflowHookCtrl struct {
	Enabled bool            `bson:"enabled" json:"enabled"`
	Items   []*WorkflowHook `bson:"items" json:"items"`
}

type WorkflowHook struct {
	AutoCancel          bool              `bson:"auto_cancel"             json:"auto_cancel"`
	CheckPatchSetChange bool              `bson:"check_patch_set_change"  json:"check_patch_set_change"`
	MainRepo            MainHookRepo      `bson:"main_repo"               json:"main_repo"`
	WorkflowArgs        *WorkflowTaskArgs `bson:"workflow_args"           json:"workflow_args"`
}

type MainHookRepo struct {
	Source       string                 `bson:"source,omitempty"          json:"source,omitempty"`
	RepoOwner    string                 `bson:"repo_owner"                json:"repo_owner"`
	RepoName     string                 `bson:"repo_name"                 json:"repo_name"`
	Branch       string                 `bson:"branch"                    json:"branch"`
	MatchFolders []string               `bson:"match_folders"             json:"match_folders,omitempty"`
	CodehostID   int                    `bson:"codehost_id"               json:"codehost_id"`
	Events       []config.HookEventType `bson:"events"                    json:"events"`
	Label        string                 `bson:"label"                     json:"label"`
	Revision     string                 `bson:"revision"                  json:"revision"`
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
	ID           primitive.ObjectID  `bson:"_id,omitempty"                 json:"id,omitempty"`
	Number       uint64              `bson:"number"                        json:"number"`
	Frequency    string              `bson:"frequency"                     json:"frequency"`
	Time         string              `bson:"time"                          json:"time"`
	MaxFailures  int                 `bson:"max_failures,omitempty"        json:"max_failures,omitempty"`
	TaskArgs     *TaskArgs           `bson:"task_args,omitempty"           json:"task_args,omitempty"`
	WorkflowArgs *WorkflowTaskArgs   `bson:"workflow_args,omitempty"       json:"workflow_args,omitempty"`
	TestArgs     *TestTaskArgs       `bson:"test_args,omitempty"           json:"test_args,omitempty"`
	Type         config.ScheduleType `bson:"type"                          json:"type"`
	Cron         string              `bson:"cron"                          json:"cron"`
	IsModified   bool                `bson:"-"                             json:"-"`
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
	NotificationId string              `bson:"notification_id"         json:"notification_id"`
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
	Target         *ServiceModuleTarget `bson:"target"                 json:"target"`
	BuildModuleVer string               `bson:"build_module_ver"       json:"build_module_ver"`
}

type ArtifactStage struct {
	Enabled bool              `bson:"enabled"                json:"enabled"`
	Modules []*ArtifactModule `bson:"modules"                json:"modules"`
}

// ArtifactModule ...
type ArtifactModule struct {
	Target *ServiceModuleTarget `bson:"target"                 json:"target"`
}

type TestStage struct {
	Enabled   bool            `bson:"enabled"                    json:"enabled"`
	TestNames []string        `bson:"test_names,omitempty"       json:"test_names,omitempty"`
	Tests     []*TestExecArgs `bson:"tests,omitempty"     json:"tests,omitempty"`
}

// TestExecArgs ...
type TestExecArgs struct {
	Name string    `bson:"test_name"             json:"test_name"`
	Envs []*KeyVal `bson:"envs"           json:"envs"`
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

type RepoImage struct {
	RepoId    string `json:"repo_id" bson:"repo_id"`
	Name      string `json:"name" bson:"name" yaml:"name"`
	Username  string `json:"-" yaml:"username"`
	Password  string `json:"-" yaml:"password"`
	Host      string `json:"host" yaml:"host"`
	Namespace string `json:"namespace" yaml:"namespace"`
}

type ProductDistribute struct {
	Target            *ServiceModuleTarget `bson:"target"                 json:"target"`
	ImageDistribute   bool                 `bson:"image_distribute"       json:"image_distribute"`
	JumpBoxDistribute bool                 `bson:"jump_box_distribute"    json:"jump_box_distribute"`
	QstackDistribute  bool                 `bson:"qstack_distribute"      json:"qstack_distribute"`
}

type NotifyCtl struct {
	Enabled         bool     `bson:"enabled"                          json:"enabled"`
	WebHookType     string   `bson:"webhook_type"                     json:"webhook_type"`
	WeChatWebHook   string   `bson:"weChat_webHook,omitempty"         json:"weChat_webHook,omitempty"`
	DingDingWebHook string   `bson:"dingding_webhook,omitempty"       json:"dingding_webhook,omitempty"`
	FeiShuWebHook   string   `bson:"feishu_webhook,omitempty"         json:"feishu_webhook,omitempty"`
	AtMobiles       []string `bson:"at_mobiles,omitempty"             json:"at_mobiles,omitempty"`
	IsAtAll         bool     `bson:"is_at_all,omitempty"              json:"is_at_all,omitempty"`
	NotifyTypes     []string `bson:"notify_type"                      json:"notify_type"`
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
	Owner      string `bson:"owner"          json:"owner,omitempty"`
	Repo       string `bson:"repo"           json:"repo,omitempty"`
	Branch     string `bson:"branch"         json:"branch,omitempty"`
	Ref        string `bson:"ref"            json:"ref,omitempty"`
	IsPr       bool   `bson:"is_pr"          json:"is_pr,omitempty"`
	CheckRunID int64  `bson:"check_run_id"   json:"check_run_id,omitempty"`
	DeliveryID string `bson:"delivery_id"    json:"delivery_id,omitempty"`
}

type TargetArgs struct {
	Name             string            `bson:"name"                      json:"name"`
	ServiceName      string            `bson:"service_name"              json:"service_name"`
	Build            *BuildArgs        `bson:"build"                     json:"build"`
	Version          string            `bson:"version"                   json:"version"`
	Deploy           []DeployEnv       `bson:"deloy"                     json:"deploy"`
	Image            string            `bson:"image"                     json:"image"`
	BinFile          string            `bson:"bin_file"                  json:"bin_file"`
	Envs             []*KeyVal         `bson:"envs"                      json:"envs"`
	HasBuild         bool              `bson:"has_build"                 json:"has_build"`
	JenkinsBuildArgs *JenkinsBuildArgs `bson:"jenkins_build_args"        json:"jenkins_build_args"`
}

type JenkinsBuildArgs struct {
	JobName            string               `bson:"job_name"            json:"job_name"`
	JenkinsBuildParams []*JenkinsBuildParam `bson:"jenkins_build_param" json:"jenkins_build_params"`
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

//Validate validate schedule setting
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
