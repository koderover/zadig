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

package config

import (
	"regexp"
)

var (
	// RenderTemplateAlias ...
	RenderTemplateAlias = regexp.MustCompile(`{{\s?\.\w+\s?}}`)
	ServiceNameAlias    = regexp.MustCompile(`\$Service\$`)

	NameSpaceRegex = regexp.MustCompile(NameSpaceRegexString)
)

const (
	ServiceNameRegexString = "^[a-zA-Z0-9-_]+$"
	ConfigNameRegexString  = "^[a-zA-Z0-9-]+$"
	ImageRegexString       = "^[a-zA-Z0-9.:\\/-]+$"

	EnvRecyclePolicyAlways     = "always"
	EnvRecyclePolicyTaskStatus = "success"
	EnvRecyclePolicyNever      = "never"

	TopicProcess      = "task.process"
	TopicCancel       = "task.cancel"
	TopicAck          = "task.ack"
	TopicItReport     = "task.it.report"
	TopicNotification = "task.notification"
	TopicCronjob      = "cronjob"

	// 定时器的所属job类型
	WorkflowCronjob = "workflow"
	TestingCronjob  = "test"
)

var (
	ServiceNameRegex = regexp.MustCompile(ServiceNameRegexString)
	ConfigNameRegex  = regexp.MustCompile(ConfigNameRegexString)
	ImageRegex       = regexp.MustCompile(ImageRegexString)
)

// ScheduleType 触发模式
type ScheduleType string

const (
	// TimingSchedule 定时循环
	TimingSchedule ScheduleType = "timing"
	// GapSchedule 间隔循环
	GapSchedule ScheduleType = "gap"
)

type SlackNotifyType string

const (
	// SlackAll SlackNotifyType = "all"
	SlackOnChange  SlackNotifyType = "onchange"
	SlackOnfailure SlackNotifyType = "onfailure"
)

// Type pipeline type
type PipelineType string

const (
	// SingleType 单服务工作流
	SingleType PipelineType = "single"
	// WorkflowType 多服务工作流
	WorkflowType PipelineType = "workflow"
	// FreestyleType 自由编排工作流
	FreestyleType PipelineType = "freestyle"
	// TestType 测试
	TestType PipelineType = "test"
	// ServiceType 服务
	ServiceType PipelineType = "service"
)

type Status string

const (
	StatusDisabled   Status = "disabled"
	StatusCreated    Status = "created"
	StatusRunning    Status = "running"
	StatusPassed     Status = "passed"
	StatusSkipped    Status = "skipped"
	StatusFailed     Status = "failed"
	StatusTimeout    Status = "timeout"
	StatusCancelled  Status = "cancelled"
	StatusWaiting    Status = "waiting"
	StatusQueued     Status = "queued"
	StatusBlocked    Status = "blocked"
	QueueItemPending Status = "pending"
)

type TaskStatus string

const (
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusReady     TaskStatus = "ready"
	TaskStatusFailed    TaskStatus = "failed"
	TaskStatusTimeout   TaskStatus = "timeout"
	TaskStatusCancelled TaskStatus = "cancelled"
	TaskStatusPass      TaskStatus = "pass"
)

type TaskType string

const (
	TaskPipeline       TaskType = "pipeline"
	TaskBuild          TaskType = "buildv2"
	TaskJenkinsBuild   TaskType = "jenkins_build"
	TaskArtifact       TaskType = "artifact"
	TaskDeploy         TaskType = "deploy"
	TaskTestingV2      TaskType = "testingv2"
	TaskDistributeToS3 TaskType = "distribute2kodo"
	TaskReleaseImage   TaskType = "release_image"
	TaskJira           TaskType = "jira"
	TaskDockerBuild    TaskType = "docker_build"
	TaskSecurity       TaskType = "security"
	TaskResetImage     TaskType = "reset_image"
	TaskDistribute     TaskType = "distribute"
)

type DistributeType string

const (
	File  DistributeType = "file"
	Image DistributeType = "image"
)

type K8SClusterStatus string

const (
	Disconnected K8SClusterStatus = "disconnected"
	Pending      K8SClusterStatus = "pending"
)

type NotifyType int

var (
	Announcement   NotifyType = 1 // 公告
	PipelineStatus NotifyType = 2 // 提醒
	Message        NotifyType = 3 // 消息
)

// Validation constants
const (
	NameSpaceRegexString = "[^a-z0-9.-]"
)

// Request ...
type Request string

const (
	// HighRequest 16 CPU 32 G
	HighRequest = Request("high")
	// MediumRequest 8 CPU 16 G
	MediumRequest = Request("medium")
	// LowRequest 4 CPU 8 G
	LowRequest = Request("low")
	// MinRequest 2 CPU 2 G
	MinRequest = Request("min")
)

//ProductPermission ...
type ProductPermission string

// ProductAuthType ...
type ProductAuthType string

const (
	// ProductReadPermission ...
	ProductReadPermission = ProductPermission("read")
	// ProductWritePermission ...
	ProductWritePermission = ProductPermission("write")
)

const (
	// ProductAuthUser ...
	ProductAuthUser = ProductAuthType("user")
	// ProductAuthTeam ...
	ProductAuthTeam = ProductAuthType("team")
)

type HookEventType string

const (
	HookEventPush    = HookEventType("push")
	HookEventPr      = HookEventType("pull_request")
	HookEventUpdated = HookEventType("ref-updated")
)

const (
	KeyStateNew     = "new"
	KeyStateUnused  = "unused"
	KeyStatePresent = "present"
)
