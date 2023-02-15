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
	"time"

	"github.com/koderover/zadig/pkg/setting"
)

var (
	// RenderTemplateAlias ...
	RenderTemplateAlias = regexp.MustCompile(`{{\s?\.\w+\s?}}`)
	ServiceNameAlias    = regexp.MustCompile(`\$Service\$`)
	ProductNameAlias    = regexp.MustCompile(`\$Product\$`)
	NameSpaceRegex      = regexp.MustCompile(NameSpaceRegexString)
)

const (
	ServiceNameRegexString = "^[a-zA-Z0-9-_]+$"
	ConfigNameRegexString  = "^[a-zA-Z0-9-]+$"
	ImageRegexString       = "^[a-zA-Z0-9.:\\/-]+$"
	CVMNameRegexString     = "^[a-zA-Z_]\\w+$"

	EnvRecyclePolicyAlways     = "always"
	EnvRecyclePolicyTaskStatus = "success"
	EnvRecyclePolicyNever      = "never"

	// 定时器的所属job类型
	WorkflowCronjob   = "workflow"
	WorkflowV4Cronjob = "workflow_v4"
	TestingCronjob    = "test"
)

var (
	ServiceNameRegex = regexp.MustCompile(ServiceNameRegexString)
	ConfigNameRegex  = regexp.MustCompile(ConfigNameRegexString)
	ImageRegex       = regexp.MustCompile(ImageRegexString)
	CVMNameRegex     = regexp.MustCompile(CVMNameRegexString)
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
	// WorkflowTypeV3
	WorkflowTypeV3 PipelineType = "workflow_v3"
	// WorkflowTypeV4
	WorkflowTypeV4 PipelineType = "workflow_v4"
	// ArtifactPackageType package artifact
	ArtifactType PipelineType = "artifact"
	// ScanningType is the type for scanning
	ScanningType PipelineType = "scanning"
)

type Status string

const (
	StatusDisabled       Status = "disabled"
	StatusCreated        Status = "created"
	StatusRunning        Status = "running"
	StatusPassed         Status = "passed"
	StatusSkipped        Status = "skipped"
	StatusFailed         Status = "failed"
	StatusTimeout        Status = "timeout"
	StatusCancelled      Status = "cancelled"
	StatusWaiting        Status = "waiting"
	StatusQueued         Status = "queued"
	StatusBlocked        Status = "blocked"
	QueueItemPending     Status = "pending"
	StatusChanged        Status = "changed"
	StatusNotRun         Status = "notRun"
	StatusPrepare        Status = "prepare"
	StatusReject         Status = "reject"
	StatusWaitingApprove Status = "waitforapprove"
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
	TaskPipeline        TaskType = "pipeline"
	TaskBuild           TaskType = "buildv2"
	TaskBuildV3         TaskType = "buildv3"
	TaskJenkinsBuild    TaskType = "jenkins_build"
	TaskArtifact        TaskType = "artifact"
	TaskArtifactDeploy  TaskType = "artifact_deploy"
	TaskDeploy          TaskType = "deploy"
	TaskTestingV2       TaskType = "testingv2"
	TaskScanning        TaskType = "scanning"
	TaskDistributeToS3  TaskType = "distribute2kodo"
	TaskReleaseImage    TaskType = "release_image"
	TaskJira            TaskType = "jira"
	TaskDockerBuild     TaskType = "docker_build"
	TaskSecurity        TaskType = "security"
	TaskResetImage      TaskType = "reset_image"
	TaskDistribute      TaskType = "distribute"
	TaskTrigger         TaskType = "trigger"
	TaskExtension       TaskType = "extension"
	TaskArtifactPackage TaskType = "artifact_package"
)

type StepType string

const (
	StepTools             StepType = "tools"
	StepShell             StepType = "shell"
	StepGit               StepType = "git"
	StepDockerBuild       StepType = "docker_build"
	StepDeploy            StepType = "deploy"
	StepHelmDeploy        StepType = "helm_deploy"
	StepCustomDeploy      StepType = "custom_deploy"
	StepImageDistribute   StepType = "image_distribute"
	StepArchive           StepType = "archive"
	StepArchiveDistribute StepType = "archive_distribute"
	StepJunitReport       StepType = "junit_report"
	StepHtmlReport        StepType = "html_report"
	StepTarArchive        StepType = "tar_archive"
	StepSonarCheck        StepType = "sonar_check"
	StepDistributeImage   StepType = "distribute_image"
)

type JobType string

const (
	JobBuild                JobType = "build"
	JobDeploy               JobType = "deploy"
	JobZadigBuild           JobType = "zadig-build"
	JobZadigDistributeImage JobType = "zadig-distribute-image"
	JobZadigTesting         JobType = "zadig-test"
	JobZadigScanning        JobType = "zadig-scanning"
	JobCustomDeploy         JobType = "custom-deploy"
	JobZadigDeploy          JobType = "zadig-deploy"
	JobZadigHelmDeploy      JobType = "zadig-helm-deploy"
	JobFreestyle            JobType = "freestyle"
	JobPlugin               JobType = "plugin"
	JobK8sBlueGreenDeploy   JobType = "k8s-blue-green-deploy"
	JobK8sBlueGreenRelease  JobType = "k8s-blue-green-release"
	JobK8sCanaryDeploy      JobType = "k8s-canary-deploy"
	JobK8sCanaryRelease     JobType = "k8s-canary-release"
	JobK8sGrayRelease       JobType = "k8s-gray-release"
	JobK8sGrayRollback      JobType = "k8s-gray-rollback"
	JobK8sPatch             JobType = "k8s-resource-patch"
	JobIstioRelease         JobType = "istio-release"
	JobIstioRollback        JobType = "istio-rollback"
	JobJira                 JobType = "jira"
	JobNacos                JobType = "nacos"
	JobApollo               JobType = "apollo"
	JobMeegoTransition      JobType = "meego-transition"
)

const (
	ZadigIstioCopySuffix     = "zadig-copy"
	ZadigLastAppliedImage    = "last-applied-image"
	ZadigLastAppliedReplicas = "last-applied-replicas"
)

type ApprovalType string

const (
	NativeApproval ApprovalType = "native"
	LarkApproval   ApprovalType = "lark"
)

type ApproveOrReject string

const (
	Approve ApproveOrReject = "approve"
	Reject  ApproveOrReject = "reject"
)

type DeploySourceType string

const (
	SourceRuntime DeploySourceType = "runtime"
	SourceFromJob DeploySourceType = "fromjob"
)

type StageType string

const (
	StageCustom  StageType = "custom"
	StepApproval StageType = "approval"
)

type DistributeType string

const (
	File  DistributeType = "file"
	Image DistributeType = "image"
	Chart DistributeType = "chart"
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

// ProductPermission ...
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
	HookEventTag     = HookEventType("tag")
	HookEventUpdated = HookEventType("ref-updated")
)

const (
	KeyStateNew     = "new"
	KeyStateUnused  = "unused"
	KeyStatePresent = "present"
)

const (
	Day       = 7
	LatestDay = 10
	Date      = "2006-01-02"
)

const (
	RegistryTypeSWR = "swr"
	RegistryTypeAWS = "ecr"
)

const (
	ImageResourceType = "image"
	TarResourceType   = "tar"
)

const (
	RoleBindingNameEdit = setting.ProductName + "-edit"
	RoleBindingNameView = setting.ProductName + "-view"
)

type CommonEnvCfgType string

const (
	CommonEnvCfgTypeIngress   CommonEnvCfgType = "Ingress"
	CommonEnvCfgTypeConfigMap CommonEnvCfgType = "ConfigMap"
	CommonEnvCfgTypeSecret    CommonEnvCfgType = "Secret"
	CommonEnvCfgTypePvc       CommonEnvCfgType = "PVC"
)

// for custom blue-green release job
const (
	BlueGreenVerionLabelName = "zadig-blue-green-version"
	BlueServiceNameSuffix    = "-zadig-blue"
	OriginVersion            = "origin"
)

// for custom gray release job
const (
	GrayLabelKey               = "zadig-gray-release"
	GrayLabelValue             = "released-by-zadig"
	GrayImageAnnotationKey     = "zadig-gray-release-image"
	GrayContainerAnnotationKey = "zadig-gray-release-container"
	GrayReplicaAnnotationKey   = "zadig-gray-release-replica"
	GrayDeploymentSuffix       = "-zadig-gray"
)

type ProjectType string

const (
	ProjectTypeHelm   = "helm"
	ProjectTypeYaml   = "yaml"
	ProjectTypeVM     = "vm"
	ProjectTypeLoaded = "loaded"
)

type RegistryProvider string

const (
	RegistryProviderACR       = "acr"
	RegistryProviderSWR       = "swr"
	RegistryProviderTCR       = "tcr"
	RegistryProviderHarbor    = "harbor"
	RegistryProviderDockerhub = "dockerhub"
	RegistryProviderECR       = "ecr"
	RegistryProviderNative    = "native"
)

const (
	TestJobJunitReportStepName   = "junit-report-step"
	TestJobHTMLReportStepName    = "html-report-step"
	TestJobArchiveResultStepName = "archive-result-step"
)

type JobRunPolicy string

const (
	DefaultRun    JobRunPolicy = ""                // default run this job
	DefaultNotRun JobRunPolicy = "default_not_run" // default not run this job
	ForceRun      JobRunPolicy = "force_run"       // force run this job
)

const DefaultDeleteDeploymentTimeout = 10 * time.Minute
