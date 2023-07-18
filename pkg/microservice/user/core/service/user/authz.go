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

import (
	"github.com/koderover/zadig/pkg/microservice/user/core/repository/mongodb"
	"go.uber.org/zap"
)

func GetUserAuthInfo(uid string, logger *zap.SugaredLogger) (*AuthorizedResources, error) {
	userRoleBindingList, err := mongodb.NewRoleBindingColl().ListUserRoleBinding(uid)
	if err != nil {
		logger.Errorf("failed to list user role binding, error: %s", err)
		return nil, err
	}
	// generate a corresponding role list for each namespace(project)
	namespacedRoleMap := make(map[string][]string)

	for _, roleBinding := range userRoleBindingList {
		namespacedRoleMap[roleBinding.Namespace] = append(namespacedRoleMap[roleBinding.Namespace], roleBinding.RoleRef.Name)
	}

	// first check if the user is a system admin, if it is, just return with the admin flags
	for _, role := range namespacedRoleMap[GeneralNamespace] {
		if role == AdminRole {
			return generateAdminRoleResource(), nil
		}
	}

	// otherwise we generate a map of namespaced(project) permission
	projectActionMap := make(map[string]*ProjectActions)
	for project, roles := range namespacedRoleMap {
		// set every permission to false if
		if _, ok := projectActionMap[project]; !ok {
			projectActionMap[project] = generateDefaultProjectActions()
		}

		for _, role := range roles {
			roleDetailInfo, found, err := mongodb.NewRoleColl().Get(project, role)
			if err != nil {
				return nil, err
			}
			if found {
				for _, rule := range roleDetailInfo.Rules {
					// resources field is no longer required, the verb itself is sufficient to explain the authorization
					for _, verb := range rule.Verbs {
						modifyUserAuth(projectActionMap[project], verb)
					}
				}
			}
			// TODO: this might be compromised if there is a role called project admin
			// special case for project admins
			if role == ProjectAdminRole {
				projectActionMap[project].IsProjectAdmin = true
			}
		}
	}

	// TODO: deal with individual resources later

	resp := &AuthorizedResources{
		IsSystemAdmin:   false,
		ProjectAuthInfo: projectActionMap,
	}

	return resp, nil
}

func generateAdminRoleResource() *AuthorizedResources {
	return &AuthorizedResources{
		IsSystemAdmin:   true,
		ProjectAuthInfo: nil,
		//AdditionalResource: nil,
	}
}

// generateDefaultProjectActions generate an ProjectActions without any authorization info.
func generateDefaultProjectActions() *ProjectActions {
	return &ProjectActions{
		Workflow: &WorkflowActions{
			View:    false,
			Create:  false,
			Edit:    false,
			Delete:  false,
			Execute: false,
		},
		Env: &EnvActions{
			View:       false,
			Create:     false,
			EditConfig: false,
			ManagePods: false,
			Delete:     false,
			DebugPod:   false,
		},
		ProductionEnv: &ProductionEnvActions{
			View:       false,
			Create:     false,
			EditConfig: false,
			ManagePods: false,
			Delete:     false,
			DebugPod:   false,
		},
		Service: &ServiceActions{
			View:   false,
			Create: false,
			Edit:   false,
			Delete: false,
		},
		ProductionService: &ProductionServiceActions{
			View:   false,
			Create: false,
			Edit:   false,
			Delete: false,
		},
		Build: &BuildActions{
			View:   false,
			Create: false,
			Edit:   false,
			Delete: false,
		},
		Test: &TestActions{
			View:    false,
			Create:  false,
			Edit:    false,
			Delete:  false,
			Execute: false,
		},
		Scanning: &ScanningActions{
			View:    false,
			Create:  false,
			Edit:    false,
			Delete:  false,
			Execute: false,
		},
		Version: &VersionActions{
			View:   false,
			Create: false,
			Delete: false,
		},
	}
}

func modifyUserAuth(userAuthInfo *ProjectActions, verb string) {
	switch verb {
	case VerbCreateDelivery:
		userAuthInfo.Version.Create = true
	case VerbDeleteDelivery:
		userAuthInfo.Version.Delete = true
	case VerbGetDelivery:
		userAuthInfo.Version.View = true
	case VerbGetTest:
		userAuthInfo.Test.View = true
	case VerbCreateTest:
		userAuthInfo.Test.Create = true
	case VerbDeleteTest:
		userAuthInfo.Test.Delete = true
	case VerbEditTest:
		userAuthInfo.Test.Edit = true
	case VerbRunTest:
		userAuthInfo.Test.Execute = true
	case VerbCreateService:
		userAuthInfo.Service.Create = true
	case VerbEditService:
		userAuthInfo.Service.Edit = true
	case VerbDeleteService:
		userAuthInfo.Service.Delete = true
	case VerbGetService:
		userAuthInfo.Service.View = true
	case VerbCreateProductionService:
		userAuthInfo.ProductionService.Create = true
	case VerbEditProductionService:
		userAuthInfo.ProductionService.Edit = true
	case VerbDeleteProductionService:
		userAuthInfo.ProductionService.Delete = true
	case VerbGetProductionService:
		userAuthInfo.ProductionService.View = true
	case VerbGetBuild:
		userAuthInfo.Build.View = true
	case VerbEditBuild:
		userAuthInfo.Build.Edit = true
	case VerbDeleteBuild:
		userAuthInfo.Build.Delete = true
	case VerbCreateBuild:
		userAuthInfo.Build.Create = true
	case VerbCreateWorkflow:
		userAuthInfo.Workflow.Create = true
	case VerbEditWorkflow:
		userAuthInfo.Workflow.Edit = true
	case VerbDeleteWorkflow:
		userAuthInfo.Workflow.Delete = true
	case VerbGetWorkflow:
		userAuthInfo.Workflow.View = true
	case VerbRunWorkflow:
		userAuthInfo.Workflow.Execute = true
	case VerbDebugWorkflow:
		userAuthInfo.Workflow.Debug = true
	case VerbGetEnvironment:
		userAuthInfo.Env.View = true
	case VerbCreateEnvironment:
		userAuthInfo.Env.Create = true
	case VerbConfigEnvironment:
		userAuthInfo.Env.EditConfig = true
	case VerbManageEnvironment:
		userAuthInfo.Env.ManagePods = true
	case VerbDeleteEnvironment:
		userAuthInfo.Env.Delete = true
	case VerbDebugEnvironmentPod:
		userAuthInfo.Env.DebugPod = true
	case VerbEnvironmentSSHPM:
		userAuthInfo.Env.SSH = true
	case VerbGetProductionEnv:
		userAuthInfo.ProductionEnv.View = true
	case VerbCreateProductionEnv:
		userAuthInfo.ProductionEnv.Create = true
	case VerbConfigProductionEnv:
		userAuthInfo.ProductionEnv.EditConfig = true
	case VerbEditProductionEnv:
		userAuthInfo.ProductionEnv.ManagePods = true
	case VerbDeleteProductionEnv:
		userAuthInfo.ProductionEnv.Delete = true
	case VerbDebugProductionEnvPod:
		userAuthInfo.ProductionEnv.DebugPod = true
	case VerbGetScan:
		userAuthInfo.Scanning.View = true
	case VerbCreateScan:
		userAuthInfo.Scanning.Create = true
	case VerbEditScan:
		userAuthInfo.Scanning.Edit = true
	case VerbDeleteScan:
		userAuthInfo.Scanning.Delete = true
	case VerbRunScan:
		userAuthInfo.Scanning.Execute = true
	}
}