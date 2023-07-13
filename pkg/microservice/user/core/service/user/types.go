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

package user

const (
	GeneralNamespace = "*"

	AdminRole = "admin"
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
)

type AuthorizedResources struct {
	IsSystemAdmin   bool
	ProjectAuthInfo map[string]*ProjectActions
	//AdditionalResource *AdditionalResources
	//SystemAuthInfo
}

type ProjectActions struct {
	Workflow          *WorkflowActions
	Env               *EnvActions
	ProductionEnv     *ProductionEnvActions
	Service           *ServiceActions
	ProductionService *ProductionServiceActions
	Build             *BuildActions
	Test              *TestActions
	Scanning          *ScanningActions
	Version           *VersionActions
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
