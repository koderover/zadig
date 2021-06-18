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

type NotifyType int

var (
	// Announcement ...
	Announcement NotifyType = 1 // 公告
	// PipelineStatus ...
	PipelineStatus NotifyType = 2 // 提醒
	// Message ...
	Message NotifyType = 3 // 消息
)

const (
	// CIStatusSuccess ...
	CIStatusSuccess = CIStatus("success")
	// CIStatusFailure ...
	CIStatusFailure = CIStatus("failure")
	// CIStatusNeutral ...
	CIStatusNeutral = CIStatus("neutral")
	// CIStatusCancelled ...
	CIStatusCancelled = CIStatus("cancelled")
	// CIStatusTimeout ...
	CIStatusTimeout = CIStatus("timed_out")
)

// CIStatus ...
type CIStatus string

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

const (
	ImageFromKoderover = "koderover"
	ImageFromCustom    = "custom"

	APIServer = "https://api.github.com/"
)
