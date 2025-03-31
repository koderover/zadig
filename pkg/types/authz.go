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

package types

// collaboration mode resource type
const (
	ResourceTypeWorkflow    = "workflow"
	ResourceTypeEnvironment = "environment"
	ResourceTypeTest        = "test"

	WorkflowTypeCustomeWorkflow = "common_workflow"
)

// collaboration mode resource actions
const (
	// workflow actions for collaboration
	WorkflowActionView  = "get_workflow"
	WorkflowActionEdit  = "edit_workflow"
	WorkflowActionRun   = "run_workflow"
	WorkflowActionDebug = "debug_workflow"
	// env actions for collaboration
	EnvActionView       = "get_environment"
	EnvActionEditConfig = "config_environment"
	EnvActionManagePod  = "manage_environment"
	EnvActionDebug      = "debug_pod"
	EnvActionSSH        = "ssh_pm"
	// production env actions
	ProductionEnvActionView       = "get_production_environment"
	ProductionEnvActionEditConfig = "config_production_environment"
	ProductionEnvActionManagePod  = "edit_production_environment"
	ProductionEnvActionDebug      = "production_debug_pod"
	// test actions
	TestActionView = "get_test"
	// scan actions
	ScanActionView = "get_scan"
	// service actions
	ServiceActionView = "get_service"
	// build actions
	BuildActionView = "get_build"
	// delivery actions
	DeliveryActionView = "get_delivery"
)

type RequestBodyType string

const (
	RequestBodyTypeJSON RequestBodyType = "json"
	RequestBodyTypeYAML RequestBodyType = "yaml"
)

type CheckCollaborationModePermissionReq struct {
	UID          string `json:"uid" form:"uid"`
	ProjectKey   string `json:"project_key" form:"project_key"`
	Resource     string `json:"resource" form:"resource"`
	ResourceName string `json:"resource_name" form:"resource_name"`
	Action       string `json:"action" form:"action"`
}

type CheckCollaborationModePermissionResp struct {
	HasPermission bool   `json:"has_permission"`
	Error         string `json:"error"`
}

type ListAuthorizedProjectResp struct {
	ProjectList []string `json:"project_list"`
	Found       bool     `json:"found"`
	Error       string   `json:"error"`
}

type ListAuthorizedWorkflowsReq struct {
	UID        string `json:"uid" form:"uid"`
	ProjectKey string `json:"project_key" form:"project_key"`
}

type ListAuthorizedEnvsReq struct {
	UID        string `json:"uid" form:"uid"`
	ProjectKey string `json:"project_key" form:"project_key"`
}

type ListAuthorizedWorkflowsResp struct {
	WorkflowList       []string `json:"workflow_list"`
	CustomWorkflowList []string `json:"custom_workflow_list"`
	Error              string   `json:"error"`
}

type CollaborationEnvPermission struct {
	Error       string   `json:"error"`
	ReadEnvList []string `json:"read_env_list"`
	EditEnvList []string `json:"edit_env_list"`
}
