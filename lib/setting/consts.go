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

package setting

const LocalConfig = "local.env"

// envs
const (
	// common
	ENVCollieAPIAddress        = "COLLIE_API_ADDRESS"
	ENVNsqLookupAddrs          = "NSQLOOKUP_ADDRS"
	ENVMongoDBConnectionString = "MONGODB_CONNECTION_STRING"
	ENVAslanDBName             = "ASLAN_DB"
	ENVAslanAPIBase            = "ASLAN_API_BASE"
	ENVHubServerAddr           = "HUB_SERVER_ADDR"
	ENVPoetryAPIServer         = "POETRY_API_SERVER"
	ENVPoetryAPIRootKey        = "POETRY_API_ROOT_KEY"

	// Aslan
	ENVHookSecret           = "HOOK_SECRET"
	ENVPodName              = "BE_POD_NAME"
	ENVNamespace            = "BE_POD_NAMESPACE"
	ENVLogLevel             = "LOG_LEVEL"
	ENVAslanURL             = "ASLAN_URL"
	ENVWarpdriveService     = "WARPDRIVE_SERVICE"
	ENVServiceStartTimeout  = "SERVICE_START_TIMEOUT"
	ENVDefaultEnvRecycleDay = "DEFAULT_ENV_RECYCLE_DAY"
	ENVDefaultIngressClass  = "DEFAULT_INGRESS_CLASS"
	ENVAslanRegAddress      = "DEFAULT_REGISTRY"
	ENVAslanRegAccessKey    = "DEFAULT_REGISTRY_AK"
	ENVAslanRegSecretKey    = "DEFAULT_REGISTRY_SK"
	ENVAslanRegNamespace    = "DEFAULT_REGISTRY_NAMESPACE"

	ENVGithubSSHKey    = "GITHUB_SSH_KEY"
	ENVGithubKnownHost = "GITHUB_KNOWN_HOST"

	ENVReaperImage      = "REAPER_IMAGE"
	ENVReaperBinaryFile = "REAPER_BINARY_FILE"
	ENVPredatorImage    = "PREDATOR_IMAGE"

	ENVDockerHosts = "DOCKER_HOSTS"

	ENVUseClassicBuild       = "USE_CLASSIC_BUILD"
	ENVCustomDNSNotSupported = "CUSTOM_DNS_NOT_SUPPORTED"

	ENVOldEnvSupported = "OLD_ENV_SUPPORTED"

	ENVS3StorageAK       = "S3STORAGE_AK"
	ENVS3StorageSK       = "S3STORAGE_SK"
	ENVS3StorageEndpoint = "S3STORAGE_ENDPOINT"
	ENVS3StorageBucket   = "S3STORAGE_BUCKET"
	ENVS3StorageProtocol = "S3STORAGE_PROTOCOL"
	ENVS3StoragePath     = "S3STORAGE_PATH"
	ENVKubeServerAddr    = "KUBE_SERVER_ADDR"

	EnvSonarAddr = "SONAR_ADDR"

	// cron
	ENVAslanAPI        = "ASLAN_API"
	ENVRootToken       = "ROOT_TOKEN"
	ENVAslanAddr       = "ASLAN_ADDRESS"
	ENVNsqdAddr        = "NSQD_ADDR"
	ENVPodIP           = "POD_IP"
	ENVPoetryAPIAddr   = "POETRY_API_ADDRESS"
	ENVClairClientAddr = "CLAIR_CLIENT_ADDRESS"

	ENVKodespaceVersion = "KODESPACE_VERSION"

	// plutus customer
	PlutusMgoAddr    = "PLUTUS_MGO_ADDR"
	PlutusMgoDB      = "PLUTUS_MGO_DB"
	PlutusMgoMode    = "PLUTUS_MGO_MODE"
	PlutusPassword   = "PLUTUS_PASSWORD"
	VendorServer     = "VENDOR_SERVER"
	VendorAPIRootKey = "VENDOR_API_ROOT_KEY"
	CustomerIP       = "CUSTOMER_IP"
	KubeCfg          = "KUBE_CFG"
	Namespace        = "NAMESPACE"

	// plutus vendor
	//PoetryAPIServer  = "POETRY_API_SERVER"
	//PoetryAPIRootKey = "POETRY_API_ROOT_KEY"
	PlutusBaseURL  = "PLUTUS_BASE_URL"
	PlutusLogLevel = "PLUTUS_LOG_LEVEL"

	// hubagent
	HubAgentToken         = "HUB_AGENT_TOKEN"
	HubServerBaseAddr     = "HUB_SERVER_BASE_ADDR"
	KubernetesServiceHost = "KUBERNETES_SERVICE_HOST"
	KubernetesServicePort = "KUBERNETES_SERVICE_PORT"
	Token                 = "X-API-Tunnel-Token"
	Params                = "X-API-Tunnel-Params"

	// warpdrive
	WarpDrivePodName    = "WD_POD_NAME"
	ReleaseImageTimeout = "RELEASE_IMAGE_TIMEOUT"
	DefaultRegistryAddr = "DEFAULT_REG_ADDRESS"
	DefaultRegistryAK   = "DEFAULT_REG_ACCESS_KEY"
	DefaultRegistrySK   = "DEFAULT_REG_SECRET_KEY"

	// reaper
	Home          = "HOME"
	PkgFile       = "PKG_FILE"
	JobConfigFile = "JOB_CONFIG_FILE"
	DockerAuthDir = "DOCKER_AUTH_DIR"
	Path          = "PATH"
	DockerHost    = "DOCKER_HOST"
	BuildURL      = "BUILD_URL"

	// jenkins
	JenkinsBuildImage = "JENKINS_BUILD_IMAGE"
)

// k8s resources
const (
	Secret             = "Secret"
	ConfigMap          = "ConfigMap"
	Ingress            = "Ingress"
	Service            = "Service"
	Deployment         = "Deployment"
	StatefulSet        = "StatefulSet"
	Pod                = "Pod"
	ReplicaSet         = "ReplicaSet"
	Job                = "Job"
	CronJob            = "CronJob"
	ClusterRoleBinding = "ClusterRoleBinding"
	ServiceAccount     = "ServiceAccount"
	ClusterRole        = "ClusterRole"
	Role               = "Role"
	RoleBinding        = "RoleBinding"

	// labels
	TaskLabel                       = "s-task"
	TypeLabel                       = "s-type"
	PipelineTypeLable               = "p-type"
	ProductLabel                    = "s-product"
	GroupLabel                      = "s-group"
	ServiceLabel                    = "s-service"
	ConfigBackupLabel               = "config-backup"
	EnvNameLabel                    = "s-env"
	UpdateBy                        = "update-by"
	UpdateById                      = "update-by-id"
	UpdateTime                      = "update-time"
	UpdatedByLabel                  = "updated-by-koderover"
	IngressClassLabel               = "kubernetes.io/ingress.class"
	IngressProxyConnectTimeoutLabel = "nginx.ingress.kubernetes.io/proxy-connect-timeout"
	IngressProxySendTimeoutLabel    = "nginx.ingress.kubernetes.io/proxy-send-timeout"
	IngressProxyReadTimeoutLabel    = "nginx.ingress.kubernetes.io/proxy-read-timeout"
	ServiceNameLabel                = "app.kubernetes.io/name"

	// Pod status
	PodRunning    = "Running"
	PodError      = "Error"
	PodUnstable   = "Unstable"
	PodCreating   = "Creating"
	PodCreated    = "created"
	PodUpdating   = "Updating"
	PodDeleting   = "Deleting"
	PodSucceeded  = "Succeeded"
	PodFailed     = "Failed"
	PodPending    = "Pending"
	PodNonStarted = "Unstart"
	PodCompleted  = "Completed"

	// cluster status
	ClusterUnknown      = "Unknown"
	ClusterNotFound     = "NotFound"
	ClusterDisconnected = "Disconnected"

	EnvCreatedBy              = "createdBy"
	EnvCreator                = "koderover"
	PodReady                  = "ready"
	JobReady                  = "Completed"
	PodNotReady               = "not ready"
	HelmReleaseStatusPending  = "Pending"
	HelmReleaseStatusDeployed = "Deployed"

	APIVersionAppsV1 = "apps/v1"

	DefaultCandidateImagePullSecret = "default-candidate-registry-secret"

	BuildType   = "build"
	DeployType  = "deploy"
	TestType    = "test"
	PublishType = "publish"

	FunctionTestType = "function"

	GitHubAPIServer = "https://api.github.com/"
)

const (
	ProtocolHTTP  string = "http"
	ProtocolHTTPS string = "https"
	ProtocolTCP   string = "tcp"

	DefaultIngressClass = "koderover-nginx"

	// K8SDeployType 容器化部署方式
	K8SDeployType = "k8s"
	// helm 部署
	HelmDeployType = "helm"

	// SourceFromZadig 配置来源由平台管理
	SourceFromZadig = "spock"
	// SourceFromGitlab 配置来源为gitlab
	SourceFromGitlab = "gitlab"
	// SourceFromGithub 配置来源为github
	SourceFromGithub = "github"
	// SourceFromGitlab 配置来源为gerrit
	SourceFromGerrit = "gerrit"
	// SourceFromGUI 配置来源为gui
	SourceFromGUI = "gui"
	//SourceFromHelm
	SourceFromHelm = "helm"
	//SourceFromExternal
	SourceFromExternal = "external"

	ProdENV = "prod"
	TestENV = "test"

	// action type
	TypeEnableCronjob  = "enable"
	TypeDisableCronjob = "disable"

	PUBLICSERVICE = "public"

	// onboarding流程第二步
	OnboardingStatusSecond = 2

	Unset            = "UNSET"
	CleanSkippedList = "CLEAN_WHITE_LIST"
	PerPage          = 20
)

// auth
const (
	SessionUsername = "Username"

	AuthPrefix  = "X-ROOT-API-KEY "
	Auth        = "Authorization"
	SessionUser = "User"
	USERAPIKEY  = "X-API-KEY"
	ROOTAPIKEY  = "X-ROOT-API-KEY"
	TIMERAPIKEY = "X-TIMER-API-KEY"
	HEADERAUTH  = "Authorization"
)

// Pod Status
const (
	StatusRunning   = "Running"
	StatusSucceeded = "Succeeded"
)

//build image consts
const (
	// BuildImageJob ...
	BuildImageJob = "docker-build"
	// ReleaseImageJob ...
	ReleaseImageJob = "docker-release"
)

const (
	JenkinsBuildJob = "jenkins-build"
)

// counter prefix
const (
	PipelineTaskFmt = "PipelineTask:%s"
	WorkflowTaskFmt = "WorkflowTask:%s"
	TestTaskFmt     = "TestTask:%s"
	ServiceTaskFmt  = "ServiceTask:%s"
)

// Product Status
const (
	ProductStatusSuccess  = "success"
	ProductStatusFailed   = "failed"
	ProductStatusCreating = "creating"
	ProductStatusUpdating = "updating"
	ProductStatusDeleting = "deleting"
	ProductStatusUnknown  = "unknown"
	ProductStatusUnstable = "Unstable"
)

const (
	NormalModeProduct = "normal"
)

// roles
const (
	RoleOwnerID = 3
	RoleUserID  = 4

	RoleUser        = "user"        // 普通用户
	RoleOwner       = "owner"       // 项目管理员
	RoleAdmin       = "admin"       // 超级管理员
	RoleContributor = "contributor" //开源项目贡献者
	SystemUser      = "system"

	GuestAccount = "guest2019"
)

// events
const (
	CreateProductEvent        = "CreateProduct"
	UpdateProductEvent        = "UpdateProduct"
	DeleteProductEvent        = "DeleteProduct"
	UpdateContainerImageEvent = "UpdateContainerImage"
)

// Service Related
const (
	// PrivateVisibility ...
	PrivateVisibility = "private"
	// PublicAccess ...
	PublicAccess = "public"
	// ChartYaml
	ChartYaml = "Chart.yaml"
	// ValuesYaml
	ValuesYaml = "values.yaml"
	// TemplatesDir
	TemplatesDir = "templates"
	// ServiceTemplateCounterName 服务模板counter name
	ServiceTemplateCounterName = "service:%s&type:%s"
	// GerritDefaultOwner
	GerritDefaultOwner = "dafault"
	// YamlFileSeperator ...
	YamlFileSeperator = "\n---\n"
)

const MaskValue = "********"

// proxy
const (
	ProxyHTTPSAddr  = "PROXY_HTTPS_ADDR"
	ProxyHTTPAddr   = "PROXY_HTTP_ADDR"
	ProxySocks5Addr = "PROXY_SOCKS_ADDR"

	EnableGitCheck = "ENABLE_GIT_CHECK"
)

const (
	// WebhookTaskCreator ...
	WebhookTaskCreator = "webhook"
	// CronTaskCreator ...
	CronTaskCreator = "timer"
	// DefaultTaskRevoker ...
	DefaultTaskRevoker = "system" // default task revoker
)

const (
	// DefaultMaxFailures ...
	DefaultMaxFailures = 10

	// FrequencySeconds ...
	FrequencySeconds = "seconds"
	// FrequencyMinutes ...
	FrequencyMinutes = "minutes"
	// FrequencyHour ...
	FrequencyHour = "hour"
	// FrequencyHours ...
	FrequencyHours = "hours"
	// FrequencyDay ...
	FrequencyDay = "day"
	// FrequencyDays ...
	FrequencyDays = "days"
	// FrequencyMondy ...
	FrequencyMondy = "monday"
	// FrequencyTuesday ...
	FrequencyTuesday = "tuesday"
	// FrequencyWednesday ...
	FrequencyWednesday = "wednesday"
	// FrequencyThursday ...
	FrequencyThursday = "thursday"
	// FrequencyFriday ...
	FrequencyFriday = "friday"
	// FrequencySaturday ...
	FrequencySaturday = "saturday"
	// FrequencySunday ...
	FrequencySunday = "sunday"
)

const (
	// FunctionTest 功能测试
	FunctionTest = "function"
	// PerformanceTest 性能测试
	PerformanceTest = "performance"
)

const (
	// UbuntuPrecis ...
	UbuntuPrecis = "precise"
	// UbuntuTrusty ...
	UbuntuTrusty = "trusty"
	// UbuntuXenial ...
	UbuntuXenial = "xenial"
	// UbuntuBionic ...
	UbuntuBionic = "bionic"
	// TestOnly ...
	TestOnly = "test"
)

const (
	Version = "stable"

	EnvRecyclePolicyAlways     = "always"
	EnvRecyclePolicyTaskStatus = "success"
	EnvRecyclePolicyNever      = "never"
)

const (
	ImageFromCustom     = "custom"
	FixedDayTimeCronjob = "timing"
	FixedGapCronjob     = "gap"
	CrontabCronjob      = "crontab"

	WorkflowCronjob = "workflow"
	TestingCronjob  = "test"

	TopicProcess      = "task.process"
	TopicCancel       = "task.cancel"
	TopicAck          = "task.ack"
	TopicItReport     = "task.it.report"
	TopicNotification = "task.notification"
	TopicCronjob      = "cronjob"
)

const MaxTries = 1
