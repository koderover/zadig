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

package webhooknotify

import (
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commontypes "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/types"
	"github.com/koderover/zadig/v2/pkg/types"
)

const (
	TokenHeader       = "X-Zadig-Token"
	InstanceHeader    = "X-Zadig-Instance"
	EventHeader       = "X-Zadig-Event"
	EventUUIDHeader   = "X-Zadig-Event-UUID"
	WebhookUUIDHeader = "X-Zadig-Webhook-UUID"

	TimeoutSeconds = 60
)

type WebHookNotifyEvent string

const (
	WebHookNotifyEventWorkflow    WebHookNotifyEvent = "workflow"
	WebHookNotifyEventReleasePlan WebHookNotifyEvent = "release_plan"
)

type WebHookNotifyObjectKind string

const (
	WebHookNotifyObjectKindWorkflow    WebHookNotifyObjectKind = "workflow"
	WebHookNotifyObjectKindReleasePlan WebHookNotifyObjectKind = "release_plan"
)

type WebHookNotify struct {
	ObjectKind  WebHookNotifyObjectKind `json:"object_kind"`
	Event       WebHookNotifyEvent      `json:"event"`
	Workflow    *WorkflowNotify         `json:"workflow"`
	ReleasePlan *ReleasePlanHookBody    `json:"release_plan"`
}

type WorkflowNotify struct {
	TaskID              int64                         `json:"task_id"`
	ProjectName         string                        `json:"project_name"`
	ProjectDisplayName  string                        `json:"project_display_name"`
	WorkflowName        string                        `json:"workflow_name"`
	WorkflowDisplayName string                        `json:"workflow_display_name"`
	Status              config.Status                 `json:"status"`
	Remark              string                        `json:"remark"`
	DetailURL           string                        `json:"detail_url"`
	Error               string                        `json:"error"`
	CreateTime          int64                         `json:"create_time"`
	StartTime           int64                         `json:"start_time"`
	EndTime             int64                         `json:"end_time"`
	Stages              []*WorkflowNotifyStage        `json:"stages"`
	TaskCreator         string                        `json:"task_creator"`
	TaskCreatorID       string                        `json:"task_creator_id"`
	TaskCreatorPhone    string                        `json:"task_creator_phone"`
	TaskCreatorEmail    string                        `json:"task_creator_email"`
	TaskType            config.CustomWorkflowTaskType `json:"task_type"`
}

type WorkflowNotifyStage struct {
	Name      string                   `json:"name"`
	Status    config.Status            `json:"status"`
	StartTime int64                    `json:"start_time"`
	EndTime   int64                    `json:"end_time"`
	Jobs      []*WorkflowNotifyJobTask `json:"jobs"`
	Error     string                   `json:"error"`
}

type WorkflowNotifyJobTask struct {
	Name        string        `json:"name"`
	DisplayName string        `json:"display_name"`
	JobType     string        `json:"type"`
	Status      config.Status `json:"status"`
	StartTime   int64         `json:"start_time"`
	EndTime     int64         `json:"end_time"`
	Error       string        `json:"error"`
	Spec        interface{}   `json:"spec"`
}

type WorkflowNotifyJobTaskBuildSpec struct {
	Repositories []*WorkflowNotifyRepository `json:"repositories"`
	Image        string                      `json:"image"`
}

type WorkflowNotifyJobTaskDeploySpec struct {
	Env            string                               `json:"env"`
	ServiceName    string                               `json:"service_name"`
	ServiceModules []*WorkflowNotifyDeployServiceModule `json:"service_modules"`
}

type WorkflowNotifyDeployServiceModule struct {
	ServiceModule string `json:"service_module"`
	Image         string `json:"image"`
}

type WorkflowNotifyRepository struct {
	Source        string `json:"source"`
	RepoOwner     string `json:"repo_owner"`
	RepoNamespace string `json:"repo_namespace"`
	RepoName      string `json:"repo_name"`
	Branch        string `json:"branch"`
	PRs           []int  `json:"prs"`
	Tag           string `json:"tag"`
	AuthorName    string `json:"author_name"`
	CommitID      string `json:"commit_id"`
	CommitURL     string `json:"commit_url"`
	CommitMessage string `json:"commit_message"`
}

type ReleasePlanHookBody struct {
	// 发布计划 ID
	ID primitive.ObjectID `json:"id"`
	// 发布计划索引
	Index int64 `json:"index"`
	// 事件
	EventName commonmodels.ReleasePlanHookEvent `json:"event_name"`
	// 发布计划名称
	Name string `json:"name"`
	// 负责人
	Manager string `json:"manager"`
	// 负责人 ID
	ManagerID string `json:"manager_id"`
	// 开始时间
	StartTime int64 `json:"start_time"`
	// 结束时间
	EndTime int64 `json:"end_time"`
	// 定时执行时间
	ScheduleExecuteTime int64 `json:"schedule_execute_time"`
	// 需求关联
	Description string `json:"description"`
	// 创建者
	CreatedBy string `json:"created_by"`
	// 创建时间
	CreateTime int64 `json:"create_time"`
	// 更新者
	UpdatedBy string `json:"updated_by"`
	// 更新时间
	UpdateTime int64 `json:"update_time"`
	// 实例代码
	InstanceCode string `json:"instance_code"`

	// 发布任务列表
	Jobs []*ReleasePlanHookJob `json:"jobs"`

	// 状态
	Status config.ReleasePlanStatus `json:"status"`

	// 规划时间
	PlanningTime int64 `json:"planning_time"`
	// 审批时间
	ApprovalTime int64 `json:"approval_time"`
	// 执行时间
	ExecutingTime int64 `json:"executing_time"`
	// 成功时间
	SuccessTime int64 `json:"success_time"`
}

type ReleasePlanHookJob struct {
	// 发布任务 ID
	ID string `json:"id"`
	// 发布任务名称
	Name string `json:"name"`
	// 发布任务类型
	Type config.ReleasePlanJobType `json:"type"`
	// 发布任务规格
	Spec interface{} `json:"spec"`

	ReleasePlanHookJobRuntime `json:",inline"`
}

type ReleasePlanHookJobRuntime struct {
	// 状态
	Status config.ReleasePlanJobStatus `json:"status"`
	// 执行者
	ExecutedBy string `json:"executed_by"`
	// 执行时间
	ExecutedTime int64 `json:"executed_time"`
}

type ReleasePlanHookTextJobSpec struct {
	// 内容
	Content string `json:"content"`
	// 备注
	Remark string `json:"remark"`
}

type ReleasePlanHookWorkflowJobSpec struct {
	// 工作流
	Workflow *OpenAPIWorkflowV4 `json:"workflow"`
	// 状态
	Status config.Status `json:"status"`
	// 任务 ID
	TaskID int64 `json:"task_id"`
}

type OpenAPIWorkflowV4 struct {
	// 工作流标识
	Name string `json:"name"`
	// 工作流名称
	DisplayName string `json:"display_name"`
	// 是否禁用
	Disabled bool `json:"disabled"`
	// 全局变量
	Params []*OpenAPIWorkflowParam `json:"params"`
	// 阶段
	Stages []*OpenAPIWorkflowStage `json:"stages"`
	// 项目
	Project string `json:"project"`
	// 描述
	Description string `json:"description"`
	// 创建者
	CreatedBy string `json:"created_by"`
	// 创建时间
	CreateTime int64 `json:"create_time"`
	// 更新者
	UpdatedBy string `json:"updated_by"`
	// 更新时间
	UpdateTime int64 `json:"update_time"`
	// 备注
	Remark string `json:"remark"`
	// 是否启用审批单
	EnableApprovalTicket bool `json:"enable_approval_ticket"`
	// 审批单 ID
	ApprovalTicketID string `json:"approval_ticket_id"`
}

type OpenAPIWorkflowParam struct {
	// 变量名称
	Name string `json:"name"`
	// 变量描述
	Description string `json:"description"`
	// 变量类型，支持 string/text/choice/repo 类型
	ParamsType string `json:"type"`
	// 变量值
	Value string `json:"value"`
	// 仓库
	Repo *OpenAPIWorkflowRepository `json:"repo"`
	// 选项
	ChoiceOption []string `json:"choice_option"`
	// 选项值
	ChoiceValue []string `json:"choice_value"`
	// 默认值
	Default string `json:"default"`
	// 是否为敏感变量
	IsCredential bool `json:"is_credential"`
	// 变量来源
	Source config.ParamSourceType `json:"source"`
}

type OpenAPIWorkflowStage struct {
	// 阶段名称
	Name string `json:"name"`
	// 阶段任务列表
	Jobs []*OpenAPIWorkflowJob `json:"jobs"`
}

type OpenAPIWorkflowJob struct {
	// 任务名称
	Name string `json:"name"`
	// 任务类型
	JobType config.JobType `json:"type"`
	// 任务规格
	Spec interface{} `json:"spec"`
	// 运行策略
	RunPolicy config.JobRunPolicy `json:"run_policy"`
}

// BuildJobSpec

type OpenAPIWorkflowBuildJobSpec struct {
	// 来源
	Source config.DeploySourceType `json:"source"`
	// 引用的任务名称，当来源为 fromjob 时使用
	JobName string `json:"job_name"`
	// 是否引用所选任务代码信息
	RefRepos bool `json:"ref_repos"`
	// 服务和构建列表
	ServiceAndBuilds []*OpenAPIWorkflowServiceAndBuild `json:"service_and_builds"`
	// OpenAPIWorkflowServiceWithModule `json:",inline"`
}

type OpenAPIWorkflowServiceWithModule struct {
	// 服务名称
	ServiceName string `json:"service_name"`
	// 服务组件
	ServiceModule string `json:"service_module"`
}

type OpenAPIWorkflowServiceAndBuild struct {
	// 服务名称
	ServiceName string `json:"service_name"`
	// 服务组件
	ServiceModule string `json:"service_module"`
	// 构建名称
	BuildName string `json:"build_name"`
	// 镜像
	Image string `json:"image"`
	// 二进制包
	Package string `json:"package"`
	// 镜像名称
	ImageName string `json:"image_name"`
	// 变量列表
	KeyVals commonmodels.RuntimeKeyValList `json:"key_vals"`
	// 仓库列表
	Repos []*OpenAPIWorkflowRepository `json:"repos"`
}

type OpenAPIWorkflowRepository struct {
	// 来源
	Source string `json:"source"`
	// 仓库所有者
	RepoOwner string `json:"repo_owner"`
	// 仓库命名空间
	RepoNamespace string `json:"repo_namespace"`
	// 仓库名称
	RepoName string `json:"repo_name"`
	// 远程名称
	RemoteName string `json:"remote_name"`
	// 分支
	Branch string `json:"branch"`
	// PR 编号列表
	PRs []int `json:"prs"`
	// 标签
	Tag string `json:"tag"`
	// 提交 ID
	CommitID string `json:"commit_id"`
	// 提交消息
	CommitMessage string `json:"commit_message"`
	// 检出路径
	CheckoutPath string `json:"checkout_path"`
	// 代码源 ID
	CodehostID int `json:"codehost_id"`
	// 地址
	Address string `json:"address"`
}

// DeployJobSpec

type OpenAPIWorkflowDeployJobSpec struct {
	// 环境名称
	Env string `json:"env"`
	// 是否生产环境
	Production bool `json:"production"`
	// 部署类型
	DeployType string `json:"deploy_type"`
	// 来源，fromjob/runtime, runtime 表示运行时输入，fromjob 表示从上游构建任务中获取
	Source config.DeploySourceType `json:"source"`
	// 环境来源
	EnvSource config.ParamSourceType `json:"env_source"`
	// 部署内容
	DeployContents []config.DeployContent `json:"deploy_contents"`
	// 引用的任务名称，当来源为 fromjob 时使用
	JobName string `json:"job_name"`
	// 版本名称
	VersionName string `json:"version_name"`
	// 服务列表
	Services []*OpenAPIWorkflowDeployServiceInfo `json:"services"`
}

type OpenAPIWorkflowDeployServiceInfo struct {
	OpenAPIWorkflowDeployBasicInfo `json:",inline"`

	OpenAPIWorkflowDeployVariableInfo `json:",inline"`
}

type OpenAPIWorkflowDeployBasicInfo struct {
	// 服务名称
	ServiceName string `json:"service_name"`
	// 服务组件列表
	Modules []*OpenAPIWorkflowDeployModuleInfo `json:"modules"`
	// 是否已部署
	Deployed bool `json:"deployed"`
	// 是否自动同步
	AutoSync bool `json:"auto_sync"`
	// 是否更新配置
	UpdateConfig bool `json:"update_config"`
	// 是否可更新
	Updatable bool `json:"updatable"`
}

type OpenAPIWorkflowDeployModuleInfo struct {
	// 服务组件
	ServiceModule string `json:"service_module"`
	// 镜像
	Image string `json:"image"`
	// 镜像名称
	ImageName string `json:"image_name"`
}

type OpenAPIWorkflowDeployVariableInfo struct {
	// 变量列表，用于 K8S Yaml 服务
	VariableKVs []*commontypes.RenderVariableKV `json:"variable_kvs"`
	// 最终的变量 Yaml，用于 Helm 和 K8S Yaml 服务
	VariableYaml string `json:"variable_yaml"`

	// Values 合并策略，用于 Helm 服务
	ValueMergeStrategy config.ValueMergeStrategy `json:"value_merge_strategy"`
	// 键值对变量，用于 Helm 服务，json 编码的键值对值
	OverrideKVs string `json:"override_kvs"`
}

// ZadigVMDeployJobSpec

type OpenAPIWorkflowVMDeployJobSpec struct {
	// 环境名称
	Env string `json:"env"`
	// 是否生产环境
	Production bool `json:"production"`
	// 环境别名
	EnvAlias string `json:"env_alias"`
	// 环境来源
	EnvSource config.ParamSourceType `json:"env_source"`
	// 是否引用所选任务代码信息
	RefRepos bool `json:"ref_repos"`
	// 服务和 VM 部署列表
	ServiceAndVMDeploys []*OpenAPIWorkflowServiceAndVMDeploy `json:"service_and_vm_deploys"`
	// 来源，fromjob/runtime, runtime 表示运行时输入，fromjob 表示从上游构建任务中获取
	Source config.DeploySourceType `json:"source"`
	// 引用的任务名称，当来源为 fromjob 时使用
	JobName string `json:"job_name"`
}

type OpenAPIWorkflowServiceAndVMDeploy struct {
	// 仓库列表
	Repos []*OpenAPIWorkflowRepository `json:"repos"`
	// 服务名称
	ServiceName string `json:"service_name"`
	// 服务组件
	ServiceModule string `json:"service_module"`
	// 部署名称
	DeployName string `json:"deploy_name"`
	// 部署交付物类型
	DeployArtifactType types.VMDeployArtifactType `json:"deploy_artifact_type"`
	// 部署交付物 URL
	ArtifactURL string `json:"artifact_url"`
	// 部署交付物文件名
	FileName string `json:"file_name"`
	// 镜像
	Image string `json:"image"`
	// 变量列表
	KeyVals commonmodels.RuntimeKeyValList `json:"key_vals"`
	// TaskID             int                            `json:"task_id"`
	// WorkflowType       config.PipelineType            `json:"workflow_type"`
	// WorkflowName       string                         `json:"workflow_name"`
	// JobTaskName        string                         `json:"job_task_name"`
}

// FreestyleJobSpec

type OpenAPIWorkflowFreestyleJobSpec struct {
	// 通用任务任务类型
	FreestyleJobType config.FreeStyleJobType `json:"freestyle_type"`
	// 服务来源
	ServiceSource config.DeploySourceType `json:"source"`
	// 引用的任务名称，当来源为 fromjob 时使用
	JobName string `json:"job_name"`

	// 是否引用所选任务代码信息
	RefRepos bool `json:"ref_repos"`
	// 仓库列表
	Repos []*OpenAPIWorkflowRepository `json:"repos"`
	// 变量列表
	Envs commonmodels.RuntimeKeyValList `json:"envs"`

	// 服务列表
	Services []*OpenAPIWorkflowFreeStyleServiceInfo `json:"services"`
}

type OpenAPIWorkflowFreeStyleServiceInfo struct {
	OpenAPIWorkflowServiceWithModule `json:",inline"`
	// 仓库列表
	Repos []*OpenAPIWorkflowRepository `json:"repos"`
	// 变量列表
	KeyVals commonmodels.RuntimeKeyValList `json:"key_vals"`
}

// TestingJobSpec

type OpenAPIWorkflowTestingJobSpec struct {
	// 测试类型
	TestType config.TestModuleType `json:"test_type"`
	// 来源
	Source config.DeploySourceType `json:"source"`
	// 引用的任务名称，当来源为 fromjob 时使用
	JobName string `json:"job_name"`
	// 是否引用所选任务代码信息
	RefRepos bool `json:"ref_repos"`
	// 服务列表，测试类型为服务测试时使用
	ServiceAndTests []*OpenAPIWorkflowServiceAndTest `json:"service_and_tests"`
	// 测试模块列表，测试类型为产品测试时使用
	TestModules []*OpenAPIWorkflowTestModule `json:"test_modules"`
}

type OpenAPIWorkflowServiceAndTest struct {
	// 服务名称
	ServiceName string `json:"service_name"`
	// 服务组件
	ServiceModule string `json:"service_module"`
	// 测试模块
	*OpenAPIWorkflowTestModule `json:",inline"`
}

type OpenAPIWorkflowTestModule struct {
	// 测试模块名称
	Name string `json:"name"`
	// 变量列表
	KeyVals commonmodels.RuntimeKeyValList `json:"key_vals"`
	// 仓库列表
	Repos []*OpenAPIWorkflowRepository `json:"repos"`
}

// ScanningJobSpec

type OpenAPIWorkflowScanningJobSpec struct {
	// 扫描类型
	ScanningType config.ScanningModuleType `bson:"scanning_type"    yaml:"scanning_type"    json:"scanning_type"`
	// 来源
	Source config.DeploySourceType `bson:"source"           yaml:"source"           json:"source"`
	// 引用的任务名称，当来源为 fromjob 时使用
	JobName string `bson:"job_name"         yaml:"job_name"         json:"job_name"`
	// 是否引用所选任务代码信息
	RefRepos bool `bson:"ref_repos"        yaml:"ref_repos"        json:"ref_repos"`
	// 扫描列表，扫描类型为产品扫描时使用
	Scannings []*OpenAPIWorkflowScanningModule `bson:"scannings"        yaml:"scannings"        json:"scannings"`
	// 服务和扫描模块列表，扫描类型为服务扫描时使用
	ServiceAndScannings []*OpenAPIWorkflowServiceAndScannings `bson:"service_and_scannings"    yaml:"service_and_scannings"    json:"service_and_scannings"`
}

type OpenAPIWorkflowServiceAndScannings struct {
	// 服务名称
	ServiceName string `json:"service_name"`
	// 服务组件
	ServiceModule                  string `json:"service_module"`
	*OpenAPIWorkflowScanningModule `json:",inline"`
}

type OpenAPIWorkflowScanningModule struct {
	// 扫描模块名称
	Name string `json:"name"`
	// 仓库列表
	Repos []*OpenAPIWorkflowRepository `json:"repos"`
	// 变量列表
	KeyVals commonmodels.RuntimeKeyValList `json:"key_vals"`
}

// NacosJobSpec

type OpenAPIWorkflowNacosJobSpec struct {
	// Nacos ID
	NacosID string `bson:"nacos_id"            json:"nacos_id"            yaml:"nacos_id"`
	// 命名空间 ID
	NamespaceID string `bson:"namespace_id"        json:"namespace_id"        yaml:"namespace_id"`
	// 命名空间来源
	Source config.ParamSourceType `bson:"source"              json:"source"              yaml:"source"`
	// Nacos 数据列表
	NacosDatas []*types.NacosConfig `bson:"nacos_datas"         json:"nacos_datas"         yaml:"nacos_datas"`
}

// ApolloJobSpec

type OpenAPIWorkflowApolloJobSpec struct {
	// Apollo ID
	ApolloID string `json:"apolloID"`
	// 命名空间列表
	NamespaceList []*OpenAPIWorkflowApolloNamespace `json:"namespaceList"`
}

type OpenAPIWorkflowApolloNamespace struct {
	// 应用 ID
	AppID string `json:"appID"`
	// 集群 ID
	ClusterID string `json:"clusterID"`
	// 环境
	Env string `json:"env"`
	// 命名空间
	Namespace string `json:"namespace"`
	// 类型
	Type string `json:"type"`
	// 原始配置
	OriginalConfig []*OpenAPIWorkflowApolloKV `json:"original_config,omitempty"`
	// 键值对列表
	KeyValList []*OpenAPIWorkflowApolloKV `json:"kv"`
}

type OpenAPIWorkflowApolloKV struct {
	// 键
	Key string `json:"key"`
	// 值
	Val string `json:"val"`
}

// SQLJobSpec

type OpenAPIWorkflowSQLJobSpec struct {
	// 数据库实例 ID
	ID string `json:"id"`
	// 数据库实例类型
	Type config.DBInstanceType `json:"type"`
	// SQL 语句
	SQL string `json:"sql"`
	// 来源
	Source string `json:"source"`
}

// DistributeImageJobSpec

type OpenAPIWorkflowDistributeImageJobSpec struct {
	// 来源
	Source config.DeploySourceType `json:"source"`
	// 引用的任务名称，当来源为 fromjob 时使用
	JobName string `json:"job_name"`
	// 分发方法
	DistributeMethod config.DistributeImageMethod `json:"distribute_method"`
	// 镜像分发目标列表
	Targets []*OpenAPIWorkflowDistributeTarget `json:"targets"`
	// 是否启用镜像版本规则
	EnableTargetImageTagRule bool `json:"enable_target_image_tag_rule"`
	// 镜像版本规则
	TargetImageTagRule string `json:"target_image_tag_rule"`
}

type OpenAPIWorkflowDistributeTarget struct {
	// 服务名称
	ServiceName string `json:"service_name"`
	// 服务组件
	ServiceModule string `json:"service_module"`
	// 源标签
	SourceTag string `json:"source_tag,omitempty"`
	// 目标标签
	TargetTag string `json:"target_tag,omitempty"`
	// 镜像名称
	ImageName string `json:"image_name,omitempty"`
	// 源镜像
	SourceImage string `json:"source_image,omitempty"`
	// 目标镜像
	TargetImage string `json:"target_image,omitempty"`
	// 如果 UpdateTag 为 false，则使用源标签作为目标标签
	UpdateTag bool `json:"update_tag"`
}
