/*
Copyright 2023 The KodeRover Authors.

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

package permission

const (
	GeneralNamespace = "*"

	AdminRole        = "admin"
	ProjectAdminRole = "project-admin"
	ReadOnlyRole     = "read-only"
)

// Namespaced Resources Actions
const (
	// delivery
	VerbGetDelivery    = "get_delivery"
	VerbCreateDelivery = "create_delivery"
	VerbDeleteDelivery = "delete_delivery"
	// test
	VerbGetTest    = "get_test"
	VerbCreateTest = "create_test"
	VerbEditTest   = "edit_test"
	VerbDeleteTest = "delete_test"
	VerbRunTest    = "run_test"
	// service
	VerbGetService    = "get_service"
	VerbCreateService = "create_service"
	VerbEditService   = "edit_service"
	VerbDeleteService = "delete_service"
	// production service
	VerbGetProductionService    = "get_production_service"
	VerbCreateProductionService = "create_production_service"
	VerbEditProductionService   = "edit_production_service"
	VerbDeleteProductionService = "delete_production_service"
	// build
	VerbGetBuild    = "get_build"
	VerbCreateBuild = "create_build"
	VerbEditBuild   = "edit_build"
	VerbDeleteBuild = "delete_build"
	// Workflow
	VerbGetWorkflow    = "get_workflow"
	VerbCreateWorkflow = "create_workflow"
	VerbEditWorkflow   = "edit_workflow"
	VerbDeleteWorkflow = "delete_workflow"
	VerbRunWorkflow    = "run_workflow"
	VerbDebugWorkflow  = "debug_workflow"
	// Environment
	VerbGetEnvironment      = "get_environment"
	VerbCreateEnvironment   = "create_environment"
	VerbConfigEnvironment   = "config_environment"
	VerbManageEnvironment   = "manage_environment"
	VerbDeleteEnvironment   = "delete_environment"
	VerbDebugEnvironmentPod = "debug_pod"
	VerbEnvironmentSSHPM    = "ssh_pm"
	// Production Environment
	VerbGetProductionEnv      = "get_production_environment"
	VerbCreateProductionEnv   = "create_production_environment"
	VerbConfigProductionEnv   = "config_production_environment"
	VerbEditProductionEnv     = "edit_production_environment"
	VerbDeleteProductionEnv   = "delete_production_environment"
	VerbDebugProductionEnvPod = "production_debug_pod"
	// Scanning
	VerbGetScan    = "get_scan"
	VerbCreateScan = "create_scan"
	VerbEditScan   = "edit_scan"
	VerbDeleteScan = "delete_scan"
	VerbRunScan    = "run_scan"
	// Sprint Template
	VerbEditSprintTemplate = "edit_sprint_template"
	// Sprint
	VerbGetSprint    = "get_sprint"
	VerbCreateSprint = "create_sprint"
	VerbEditSprint   = "edit_sprint"
	VerbDeleteSprint = "delete_sprint"
	// Sprint WorkItem
	VerbCreateSprintWorkItem = "create_sprint_workitem"
	VerbEditSprintWorkItem   = "edit_sprint_workitem"
	VerbDeleteSprintWorkItem = "delete_sprint_workitem"
)

// system level authorization actions
const (
	// project
	VerbCreateProject = "create_project"
	VerbDeleteProject = "delete_project"
	// template store
	VerbCreateTemplate = "create_template"
	VerbGetTemplate    = "get_template"
	VerbEditTemplate   = "edit_template"
	VerbDeleteTemplate = "delete_template"
	// Quality center
	VerbViewTestCenter = "get_test_detail"
	// release center, deprecated after 1.7.0
	VerbViewReleaseCenter = "get_release"
	// delivery center
	VerbDeliveryCenterGetVersions = "release_get"
	VerbDeliveryCenterGetArtifact = "delivery_get"
	// data center
	VerbGetDataCenterOverview       = "data_over"
	VerbGetDataCenterInsight        = "efficiency_over"
	VerbEditDataCenterInsightConfig = "edit_dashboard_config"
	// release plan
	VerbGetReleasePlan          = "get_release_plan"
	VerbCreateReleasePlan       = "create_release_plan"
	VerbEditReleasePlanMetadata = "edit_release_plan_metadata"
	VerbEditReleasePlanApproval = "edit_release_plan_approval"
	VerbEditReleasePlanSubtasks = "edit_release_plan_subtasks"
	VerbDeleteReleasePlan       = "delete_release_plan"
	VerbEditConfigReleasePlan   = "edit_config_release_plan"
	// cluster management
	VerbGetClusterManagement    = "get_cluster_management"
	VerbCreateClusterManagement = "create_cluster_management"
	VerbEditClusterManagement   = "edit_cluster_management"
	VerbDeleteClusterManagement = "delete_cluster_management"
	// vm management
	VerbGetVMManagement    = "get_vm_management"
	VerbCreateVMManagement = "create_vm_management"
	VerbEditVMManagement   = "edit_vm_management"
	VerbDeleteVMManagement = "delete_vm_management"
	// registry management
	VerbGetRegistryManagement    = "get_registry_management"
	VerbCreateRegistryManagement = "create_registry_management"
	VerbEditRegistryManagement   = "edit_registry_management"
	VerbDeleteRegistryManagement = "delete_registry_management"
	// s3storage management
	VerbGetS3StorageManagement    = "get_s3storage_management"
	VerbCreateS3StorageManagement = "create_s3storage_management"
	VerbEditS3StorageManagement   = "edit_s3storage_management"
	VerbDeleteS3StorageManagement = "delete_s3storage_management"
	// helmrepo management
	VerbGetHelmRepoManagement    = "get_helmrepo_management"
	VerbCreateHelmRepoManagement = "create_helmrepo_management"
	VerbEditHelmRepoManagement   = "edit_helmrepo_management"
	VerbDeleteHelmRepoManagement = "delete_helmrepo_management"
	// business directory
	VerbGetBusinessDirectory = "get_business_directory"
	// dbinstance management
	VerbGetDBInstanceManagement    = "get_dbinstance_management"
	VerbCreateDBInstanceManagement = "create_dbinstance_management"
	VerbEditDBInstanceManagement   = "edit_dbinstance_management"
	VerbDeleteDBInstanceManagement = "delete_dbinstance_management"
	// label management
	VerbCreateLabelSetting = "create_label_setting"
	VerbEditLabelSetting   = "edit_label_setting"
	VerbDeleteLabelSetting = "delete_label_setting"
)

type AuthorizedResources struct {
	IsSystemAdmin   bool                      `json:"is_system_admin"`
	ProjectAuthInfo map[string]ProjectActions `json:"project_auth_info"`
	SystemActions   *SystemActions            `json:"system_actions"`
}

type ProjectActions struct {
	IsProjectAdmin    bool                      `json:"is_system_admin"`
	Workflow          *WorkflowActions          `json:"workflow"`
	Env               *EnvActions               `json:"env"`
	ProductionEnv     *ProductionEnvActions     `json:"production_env"`
	Service           *ServiceActions           `json:"service"`
	ProductionService *ProductionServiceActions `json:"production_service"`
	Build             *BuildActions             `json:"build"`
	Test              *TestActions              `json:"test"`
	Scanning          *ScanningActions          `json:"scanning"`
	Version           *VersionActions           `json:"version"`
	SprintTemplate    *SprintTemplateActions    `json:"sprint_template"`
	Sprint            *SprintActions            `json:"sprint"`
	SprintWorkItem    *SprintWorkItemActions    `json:"sprint_workitem"`
}

type SystemActions struct {
	Project              *SystemProjectActions        `json:"project"`
	Template             *TemplateActions             `json:"template"`
	TestCenter           *TestCenterActions           `json:"test_center"`
	ReleaseCenter        *ReleaseCenterActions        `json:"release_center"`
	DeliveryCenter       *DeliveryCenterActions       `json:"delivery_center"`
	DataCenter           *DataCenterActions           `json:"data_center"`
	ReleasePlan          *ReleasePlanActions          `json:"release_plan"`
	BusinessDirectory    *BusinessDirectoryActions    `json:"business_directory"`
	ClusterManagement    *ClusterManagementActions    `json:"cluster_management"`
	VMManagement         *VMManagementActions         `json:"vm_management"`
	RegistryManagement   *RegistryManagementActions   `json:"registry_management"`
	S3StorageManagement  *S3StorageManagementActions  `json:"s3storage_management"`
	HelmRepoManagement   *HelmRepoManagementActions   `json:"helmrepo_management"`
	DBInstanceManagement *DBInstanceManagementActions `json:"dbinstance_management"`
	LabelManagement      *LabelManagementActions      `json:"label_management"`
}

type WorkflowActions struct {
	View    bool
	Create  bool
	Edit    bool
	Delete  bool
	Execute bool
	Debug   bool
}

type EnvActions struct {
	View   bool
	Create bool
	// 配置
	EditConfig bool
	// 管理服务实例
	ManagePods bool
	Delete     bool
	DebugPod   bool
	// 主机登录
	SSH bool
}

type ProductionEnvActions struct {
	View   bool
	Create bool
	// 配置
	EditConfig bool
	// 管理服务实例
	ManagePods bool
	Delete     bool
	DebugPod   bool
}

type ServiceActions struct {
	View   bool
	Create bool
	Edit   bool
	Delete bool
}

type ProductionServiceActions struct {
	View   bool
	Create bool
	Edit   bool
	Delete bool
}

type BuildActions struct {
	View   bool
	Create bool
	Edit   bool
	Delete bool
}

type TestActions struct {
	View    bool
	Create  bool
	Edit    bool
	Delete  bool
	Execute bool
}

type ScanningActions struct {
	View    bool
	Create  bool
	Edit    bool
	Delete  bool
	Execute bool
}

type VersionActions struct {
	View   bool
	Create bool
	Delete bool
}

type SprintTemplateActions struct {
	Edit bool
}

type SprintActions struct {
	Create bool
	View   bool
	Edit   bool
	Delete bool
}

type SprintWorkItemActions struct {
	Create bool
	Edit   bool
	Delete bool
}

type SystemProjectActions struct {
	Create bool
	Delete bool
}

type TemplateActions struct {
	Create bool
	View   bool
	Edit   bool
	Delete bool
}

type TestCenterActions struct {
	View bool
}

type ReleaseCenterActions struct {
	View bool
}

type DeliveryCenterActions struct {
	ViewArtifact bool
	ViewVersion  bool
}

type DataCenterActions struct {
	ViewOverView      bool
	ViewInsight       bool
	EditInsightConfig bool
}

type ReleasePlanActions struct {
	Create     bool
	View       bool
	EditMetadata bool
	EditApproval bool
	EditSubtasks bool
	Delete     bool
	// edit config is the ability to edit the global config for release plan
	EditConfig bool
}

type BusinessDirectoryActions struct {
	View bool
}

type ClusterManagementActions struct {
	Create bool
	View   bool
	Edit   bool
	Delete bool
}

type VMManagementActions struct {
	Create bool
	View   bool
	Edit   bool
	Delete bool
}

type RegistryManagementActions struct {
	Create bool
	View   bool
	Edit   bool
	Delete bool
}

type S3StorageManagementActions struct {
	Create bool
	View   bool
	Edit   bool
	Delete bool
}

type HelmRepoManagementActions struct {
	Create bool
	View   bool
	Edit   bool
	Delete bool
}

type DBInstanceManagementActions struct {
	Create bool
	View   bool
	Edit   bool
	Delete bool
}

type LabelManagementActions struct {
	Create bool
	Edit   bool
	Delete bool
}
