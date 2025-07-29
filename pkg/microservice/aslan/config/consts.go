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
	"strings"
	"time"

	"github.com/koderover/zadig/v2/pkg/setting"
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
	CVMNameRegexString     = "^[a-zA-Z_]\\w*$"

	EnvRecyclePolicyAlways     = "always"
	EnvRecyclePolicyTaskStatus = "success"
	EnvRecyclePolicyNever      = "never"
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
	// UnixstampSchedule 时间戳
	UnixstampSchedule ScheduleType = "unix_stamp"
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
	// PipelineTypeService 服务
	PipelineTypeService PipelineType = "service"
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

func (s Status) ToLower() Status {
	return Status(strings.ToLower(string(s)))
}

const (
	StatusDisabled       Status = "disabled"
	StatusCreated        Status = "created"
	StatusRunning        Status = "running"
	StatusPassed         Status = "passed"
	StatusSkipped        Status = "skipped"
	StatusFailed         Status = "failed"
	StatusTimeout        Status = "timeout"
	StatusCancelled      Status = "cancelled"
	StatusPause          Status = "pause"
	StatusWaiting        Status = "waiting"
	StatusQueued         Status = "queued"
	StatusBlocked        Status = "blocked"
	QueueItemPending     Status = "pending"
	StatusChanged        Status = "changed"
	StatusNotRun         Status = "notRun"
	StatusPrepare        Status = "prepare"
	StatusReject         Status = "reject"
	StatusDistributed    Status = "distributed"
	StatusWaitingApprove Status = "wait_for_approval"
	StatusDebugBefore    Status = "debug_before"
	StatusDebugAfter     Status = "debug_after"
	StatusUnstable       Status = "unstable"
	StatusManualApproval Status = "wait_for_manual_error_handling"
)

func FailedStatus() []Status {
	return []Status{StatusFailed, StatusTimeout, StatusCancelled, StatusReject}
}

func InCompletedStatus() []Status {
	return []Status{StatusCreated, StatusRunning, StatusWaiting, StatusQueued, StatusBlocked, QueueItemPending, StatusPrepare, StatusWaitingApprove, ""}
}

func CompletedStatus() []Status {
	return []Status{StatusPassed, StatusFailed, StatusTimeout, StatusCancelled, StatusReject}
}

type CustomWorkflowTaskType string

const (
	WorkflowTaskTypeWorkflow CustomWorkflowTaskType = "workflow"
	WorkflowTaskTypeTesting  CustomWorkflowTaskType = "test"
	WorkflowTaskTypeScanning CustomWorkflowTaskType = "scan"
	WorkflowTaskTypeDelivery CustomWorkflowTaskType = "delivery"
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
	StepBatchFile         StepType = "batch_file"
	StepPowerShell        StepType = "powershell"
	StepGit               StepType = "git"
	StepPerforce          StepType = "perforce"
	StepDockerBuild       StepType = "docker_build"
	StepDeploy            StepType = "deploy"
	StepHelmDeploy        StepType = "helm_deploy"
	StepCustomDeploy      StepType = "custom_deploy"
	StepImageDistribute   StepType = "image_distribute"
	StepDownloadArchive   StepType = "download_archive"
	StepArchive           StepType = "archive"
	StepArchiveHtml       StepType = "archive_html"
	StepArchiveDistribute StepType = "archive_distribute"
	StepJunitReport       StepType = "junit_report"
	StepHtmlReport        StepType = "html_report"
	StepTarArchive        StepType = "tar_archive"
	StepSonarCheck        StepType = "sonar_check"
	StepSonarGetMetrics   StepType = "sonar_get_metrics"
	StepDistributeImage   StepType = "distribute_image"
	StepDebugBefore       StepType = "debug_before"
	StepDebugAfter        StepType = "debug_after"
)

type JobType string

const (
	// deprecated
	JobBuild JobType = "build"
	// deprecated
	JobDeploy JobType = "deploy"

	JobZadigBuild           JobType = "zadig-build"
	JobZadigDistributeImage JobType = "zadig-distribute-image"
	JobZadigTesting         JobType = "zadig-test"
	JobZadigScanning        JobType = "zadig-scanning"
	JobCustomDeploy         JobType = "custom-deploy"
	JobZadigDeploy          JobType = "zadig-deploy"
	JobZadigVMDeploy        JobType = "zadig-vm-deploy"
	JobZadigHelmDeploy      JobType = "zadig-helm-deploy"
	JobZadigHelmChartDeploy JobType = "zadig-helm-chart-deploy"
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
	JobUpdateEnvIstioConfig JobType = "update-env-istio-config"
	JobJira                 JobType = "jira"
	JobNacos                JobType = "nacos"
	JobApollo               JobType = "apollo"
	JobPingCode             JobType = "pingcode"
	JobSQL                  JobType = "sql"
	JobJenkins              JobType = "jenkins"
	JobMeegoTransition      JobType = "meego-transition"
	JobWorkflowTrigger      JobType = "workflow-trigger"
	JobOfflineService       JobType = "offline-service"
	JobMseGrayRelease       JobType = "mse-gray-release"
	JobMseGrayOffline       JobType = "mse-gray-offline"
	JobGuanceyunCheck       JobType = "guanceyun-check"
	JobGrafana              JobType = "grafana"
	JobBlueKing             JobType = "blueking"
	JobApproval             JobType = "approval"
	JobNotification         JobType = "notification"
	JobSAEDeploy            JobType = "sae-deploy"
)

const (
	ZadigIstioCopySuffix     = "zadig-copy"
	ZadigLastAppliedImage    = "last-applied-image"
	ZadigLastAppliedReplicas = "last-applied-replicas"
)

type DBInstanceType string

const (
	DBInstanceTypeMySQL   DBInstanceType = "mysql"
	DBInstanceTypeMariaDB DBInstanceType = "mariadb"
)

type ObservabilityType string

const (
	ObservabilityTypeGrafana   ObservabilityType = "grafana"
	ObservabilityTypeGuanceyun ObservabilityType = "guanceyun"
)

type ApprovalType string

const (
	NativeApproval   ApprovalType = "native"
	LarkApproval     ApprovalType = "lark"
	DingTalkApproval ApprovalType = "dingtalk"
	WorkWXApproval   ApprovalType = "workwx"
)

type SAEUpdateStrategy string

const (
	SAEUpdateTypeGrayBatch = "GrayBatchUpdate"
	SAEUpdateTypeBatch     = "BatchUpdate"
)

type SAEBatchReleaseType string

const (
	SAEBatchReleaseTypeAuto   = "auto"
	SAEBatchReleaseTypeManual = "manual"
)

type ApprovalStatus string

const (
	ApprovalStatusPending  ApprovalStatus = ""
	ApprovalStatusApprove  ApprovalStatus = "approve"
	ApprovalStatusReject   ApprovalStatus = "reject"
	ApprovalStatusRedirect ApprovalStatus = "redirect"
	ApprovalStatusDone     ApprovalStatus = "done"
)

type DeploySourceType string

const (
	SourceRuntime DeploySourceType = "runtime"
	SourceFixed   DeploySourceType = "fixed"
	SourceFromJob DeploySourceType = "fromjob"
)

type TriggerWorkflowSourceType string

const (
	TriggerWorkflowSourceRuntime TriggerWorkflowSourceType = "runtime"
	TriggerWorkflowSourceFromJob TriggerWorkflowSourceType = "fromjob"
)

type EnvType string

const (
	EnvTypeZadig EnvType = "zadig"
	EnvTypeSae   EnvType = "sae"
)

type EnvOperation string

const (
	EnvOperationDefault  EnvOperation = "default"
	EnvOperationRollback EnvOperation = "rollback"
)

type EnvOperationType string

const (
	EnvOperationTypeZadig          EnvOperationType = "zadig"
	EnvOperationTypeSae            EnvOperationType = "sae"
	EnvOperationTypeSaeChangeOrder EnvOperationType = "sae_change_order"
)

type ServiceType string

const (
	ServiceTypeHelm      ServiceType = "helm"
	ServiceTypeHelmChart ServiceType = "helm_chart"
	ServiceTypeK8S       ServiceType = "k8s"
	ServiceTypeSae       ServiceType = "sae"
	ServiceTypeVM        ServiceType = "vm"
)

type TestModuleType string

const (
	ProductTestType TestModuleType = ""
	ServiceTestType TestModuleType = "service_test"
)

type ScanningModuleType string

const (
	NormalScanningType  ScanningModuleType = ""
	ServiceScanningType ScanningModuleType = "service_scanning"
)

type FreeStyleJobType string

const (
	NormalFreeStyleJobType  FreeStyleJobType = ""
	ServiceFreeStyleJobType FreeStyleJobType = "service_freestyle"
)

type DeployContent string

const (
	DeployImage  DeployContent = "image"
	DeployVars   DeployContent = "vars"
	DeployConfig DeployContent = "config"
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
	Announcement       NotifyType = 1 // 公告
	PipelineStatus     NotifyType = 2 // 提醒
	Message            NotifyType = 3 // 消息
	WorkflowTaskStatus NotifyType = 4 // 工作流任务状态
)

// Validation constants
const (
	NameSpaceRegexString = "[^a-z0-9.-]"
)

type HookEventType string

const (
	HookEventPush    = HookEventType("push")
	HookEventPr      = HookEventType("pull_request")
	HookEventTag     = HookEventType("tag")
	HookEventUpdated = HookEventType("ref-updated")
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
	BlueGreenVersionLabelName = "zadig-blue-green-version"
	BlueServiceNameSuffix     = "-zadig-blue"
	OriginVersion             = "origin"
	BlueVersion               = "blue"
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

type WorkflowTriggerType string

const (
	WorkflowTriggerTypeCommon WorkflowTriggerType = "common"
	WorkflowTriggerTypeFixed  WorkflowTriggerType = "fixed"
)

type ProjectType string

const (
	ProjectTypeHelm   = "helm"
	ProjectTypeYaml   = "yaml"
	ProjectTypeVM     = "vm"
	ProjectTypeLoaded = "loaded"
)

type ParamType string

const (
	ParamTypeString = "string"
	ParamTypeBool   = "bool"
	ParamTypeChoice = "choice"
)

type WorkflowParamType string

const (
	WorkflowParamTypeString WorkflowParamType = "string"
	WorkflowParamTypeText   WorkflowParamType = "text"
	WorkflowParamTypeChoice WorkflowParamType = "choice"
	WorkflowParamTypeRepo   WorkflowParamType = "repo"
)

type ParamSourceType string

const (
	ParamSourceRuntime   = "runtime"
	ParamSourceFixed     = "fixed"
	ParamSourceReference = "reference"
)

type RegistryProvider string

const (
	RegistryProviderACR           = "acr"
	RegistryProviderACREnterprise = "acr-enterprise"
	RegistryProviderSWR           = "swr"
	RegistryProviderTCR           = "tcr"
	RegistryProviderTCREnterprise = "tcr-enterprise"
	RegistryProviderHarbor        = "harbor"
	RegistryProviderDockerhub     = "dockerhub"
	RegistryProviderECR           = "ecr"
	RegistryProviderJFrog         = "jfrog"
	RegistryProviderNative        = "native"
)

type S3StorageProvider int

const (
	S3StorageProviderAmazonS3 = 5
)

type ClusterProvider int

const (
	ClusterProviderAmazonEKS     = 4
	ClusterProviderTKEServerless = 5
)

type VMProvider int

const (
	VMProviderAmazon = 4
)

const (
	TestJobJunitReportStepName       = "junit-report-step"
	TestJobHTMLReportStepName        = "html-report-step"
	TestJobHTMLReportArchiveStepName = "html-report-archive-step"
	TestJobArchiveResultStepName     = "archive-result-step"
	TestJobObjectStorageStepName     = "object-storage-step"
)

const (
	ScanningJobArchiveResultStepName = "archive-result-step"
)

type JobRunPolicy string

const (
	DefaultRun    JobRunPolicy = ""                // default run this job
	DefaultNotRun JobRunPolicy = "default_not_run" // default not run this job
	ForceRun      JobRunPolicy = "force_run"       // force run this job
	SkipRun       JobRunPolicy = "skip"
)

type JobErrorPolicy string

const (
	JobErrorPolicyStop        JobErrorPolicy = "stop"
	JobErrorPolicyIgnoreError JobErrorPolicy = "ignore_error"
	JobErrorPolicyManualCheck JobErrorPolicy = "manual_check"
	JobErrorPolicyRetry       JobErrorPolicy = "retry"
)

const DefaultDeleteDeploymentTimeout = 10 * time.Minute

// Service creation source for openAPI
const (
	SourceFromTemplate = "template"
	SourceFromYaml     = "yaml"
)

type JiraAuthType string

const (
	JiraBasicAuth           JiraAuthType = "password_or_token"
	JiraPersonalAccessToken JiraAuthType = "personal_access_token"
)

// statistics dashboard id enum
const (
	DashboardDataTypeTestPassRate           = "test_pass_rate"
	DashboardDataTypeTestAverageDuration    = "test_average_duration"
	DashboardDataTypeBuildSuccessRate       = "build_success_rate"
	DashboardDataTypeBuildAverageDuration   = "build_average_duration"
	DashboardDataTypeBuildFrequency         = "build_frequency"
	DashboardDataTypeDeploySuccessRate      = "deploy_success_rate"
	DashboardDataTypeDeployAverageDuration  = "deploy_average_duration"
	DashboardDataTypeDeployFrequency        = "deploy_frequency"
	DashboardDataTypeReleaseSuccessRate     = "release_success_rate"
	DashboardDataTypeReleaseAverageDuration = "release_average_duration"
	DashboardDataTypeReleaseFrequency       = "release_frequency"

	DashboardDataSourceZadig = "zadig"
	DashboardDataSourceApi   = "api"

	DashboardDataCategoryQuality    = "quality"
	DashboardDataCategoryEfficiency = "efficiency"
	DashboardDataCategorySchedule   = "schedule"

	DashboardFunctionBuildAverageDuration = "5400/(x+540)"
	DashboardFunctionBuildSuccessRate     = "-2400/(x-120)-20"
	DashboardFunctionDeploySuccessRate    = "-2400/(x-120)-20"
	DashboardFunctionDeployFrequency      = "100-40000/(3*x+400)"
	DashboardFunctionTestPassRate         = "(x**2)/80-x/4+1.25"
	DashboardFunctionTestAverageDuration  = "90000/(x+900)"
	DashboardFunctionReleaseFrequency     = "100-200/(x+2)"
)

const (
	CUSTOME_THEME = "custom"
)

type ReleasePlanStatus string

const (
	StatusPlanning         ReleasePlanStatus = "planning"
	StatusWaitForApprove   ReleasePlanStatus = "wait_for_approval"
	StatusExecuting        ReleasePlanStatus = "executing"
	StatusApprovalDenied   ReleasePlanStatus = "denied"
	StatusTimeoutForWindow ReleasePlanStatus = "timeout"
	StatusSuccess          ReleasePlanStatus = "success"
	StatusCancel           ReleasePlanStatus = "cancel"
)

// ReleasePlanStatusMap is a map of status and its available next status
var ReleasePlanStatusMap = map[ReleasePlanStatus][]ReleasePlanStatus{
	StatusPlanning:         {StatusWaitForApprove, StatusExecuting},
	StatusWaitForApprove:   {StatusPlanning, StatusExecuting},
	StatusExecuting:        {StatusPlanning, StatusSuccess, StatusCancel},
	StatusTimeoutForWindow: {StatusPlanning},
	StatusApprovalDenied:   {StatusPlanning},
}

type ReleasePlanJobType string

const (
	JobText     ReleasePlanJobType = "text"
	JobWorkflow ReleasePlanJobType = "workflow"
)

type ReleasePlanJobStatus string

const (
	ReleasePlanJobStatusTodo    ReleasePlanJobStatus = "todo"
	ReleasePlanJobStatusDone    ReleasePlanJobStatus = "done"
	ReleasePlanJobStatusSkipped ReleasePlanJobStatus = "skipped"
	ReleasePlanJobStatusFailed  ReleasePlanJobStatus = "failed"
	ReleasePlanJobStatusRunning ReleasePlanJobStatus = "running"
)

// WorkWX Related constants
const (
	DefaultWorkWXApprovalControlType = "Textarea"
	DefaultWorkWXApprovalControlID   = "Textarea-1"
)

type ProductionType string

const (
	Production = "production"
	Testing    = "testing"
	Both       = "both"
)

type DistributeImageMethod string

const (
	DistributeImageMethodImagePush DistributeImageMethod = "image_push"
	DistributeImageMethodCloudSync DistributeImageMethod = "cloud_sync"
)

const (
	AgentTypeZadigDefaultServiceAccountName      = "koderover-agent"
	KubeConfigTypeZadigDefaultServiceAccountName = "zadig-workflow-sa"
)

const (
	VariableRegEx             = `{{\.([\p{L}\d-]+(\.[\p{L}\d-]+)*.)}}`
	VariableOutputRegEx       = `{{\.([\p{L}\d-]+(\.[\p{L}\d-]+)*).output.[\p{L}\d-]+}}`
	ReplacedTempVariableRegEx = `TEMP_PLACEHOLDER_([\p{L}\d-]+(\.[\p{L}\d-]+)*)`
)

type ValueMergeStrategy string

const (
	ValueMergeStrategyReuseValue = "reuse-values"
	ValueMergeStrategyOverride   = "override"
)

type SystemLanguage string

const (
	SystemLanguageZhCN SystemLanguage = "zh-cn"
	SystemLanguageEnUS SystemLanguage = "en-us"
)
