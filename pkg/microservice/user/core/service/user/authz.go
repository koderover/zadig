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
	"fmt"

	"github.com/koderover/zadig/pkg/microservice/user/core/repository/models"
	"github.com/koderover/zadig/pkg/microservice/user/core/repository/mongodb"
	"github.com/koderover/zadig/pkg/types"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/sets"
)

func GetUserAuthInfo(uid string, logger *zap.SugaredLogger) (*AuthorizedResources, error) {
	// system calls
	if uid == "" {
		return generateAdminRoleResource(), nil
	}

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

	// generate system actions for user
	systemActions := generateDefaultSystemActions()

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
						if project != GeneralNamespace {
							modifyUserProjectAuth(projectActionMap[project], verb)
						} else {
							modifySystemAction(systemActions, verb)
						}

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

	for _, role := range namespacedRoleMap[GeneralNamespace] {
		roleDetailInfo, found, err := mongodb.NewRoleColl().Get(GeneralNamespace, role)
		if err != nil {
			return nil, err
		}
		if found {
			for _, rule := range roleDetailInfo.Rules {
				for _, verb := range rule.Verbs {
					modifySystemAction(systemActions, verb)
				}
			}
		}
	}

	projectInfo := make(map[string]ProjectActions)
	for proj, actions := range projectActionMap {
		projectInfo[proj] = *actions
	}

	resp := &AuthorizedResources{
		IsSystemAdmin:   false,
		ProjectAuthInfo: projectInfo,
		SystemActions:   systemActions,
	}

	return resp, nil
}

func CheckCollaborationModePermission(uid, projectKey, resource, resourceName, action string) (hasPermission bool, err error) {
	hasPermission = false
	collabInstance, findErr := mongodb.NewCollaborationInstanceColl().FindInstance(uid, projectKey)
	if findErr != nil {
		err = findErr
		return
	}

	switch resource {
	case types.ResourceTypeWorkflow:
		hasPermission = checkWorkflowPermission(collabInstance.Workflows, resourceName, action)
	case types.ResourceTypeEnvironment:
		hasPermission = checkEnvPermission(collabInstance.Products, resourceName, action)
	default:
		return
	}
	return
}

func ListAuthorizedProject(uid string, logger *zap.SugaredLogger) ([]string, error) {
	respSet := sets.NewString()

	userRoleBindingList, err := mongodb.NewRoleBindingColl().ListUserRoleBinding(uid)
	if err != nil {
		logger.Errorf("failed to list user role binding, error: %s", err)
		return nil, fmt.Errorf("failed to list user role binding, error: %s", err)
	}

	// generate a corresponding role list for each namespace(project)
	namespacedRoleMap := make(map[string][]string)

	for _, roleBinding := range userRoleBindingList {
		namespacedRoleMap[roleBinding.Namespace] = append(namespacedRoleMap[roleBinding.Namespace], roleBinding.RoleRef.Name)
	}

	for project, _ := range namespacedRoleMap {
		respSet.Insert(project)
	}

	collaborationModeList, err := mongodb.NewCollaborationModeColl().ListUserCollaborationMode(uid)
	if err != nil {
		logger.Errorf("failed to find user collaboration mode, error: %s", err)
		return nil, fmt.Errorf("failed to find user collaboration mode, error: %s", err)
	}

	// if user have collaboration mode, they must have access to this project.
	for _, collabMode := range collaborationModeList {
		respSet.Insert(collabMode.ProjectName)
	}

	return respSet.List(), nil
}

func checkWorkflowPermission(list []models.WorkflowCIItem, workflowName, action string) bool {
	for _, workflow := range list {
		if workflow.Name == workflowName {
			for _, verb := range workflow.Verbs {
				if verb == action {
					return true
				}
			}
		}
	}
	return false
}

func checkEnvPermission(list []models.ProductCIItem, envName, action string) bool {
	for _, env := range list {
		if env.Name == envName {
			for _, verb := range env.Verbs {
				if verb == action {
					return true
				}
			}
		}
	}
	return false
}

func generateAdminRoleResource() *AuthorizedResources {
	return &AuthorizedResources{
		IsSystemAdmin:   true,
		ProjectAuthInfo: nil,
		SystemActions:   nil,
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

func generateDefaultSystemActions() *SystemActions {
	return &SystemActions{
		Project: &SystemProjectActions{
			Create: false,
			Delete: false,
		},
		Template: &TemplateActions{
			Create: false,
			View:   false,
			Edit:   false,
			Delete: false,
		},
		TestCenter: &TestCenterActions{
			View: false,
		},
		ReleaseCenter: &ReleaseCenterActions{
			View: false,
		},
		DeliveryCenter: &DeliveryCenterActions{
			ViewArtifact: false,
			ViewVersion:  false,
		},
		DataCenter: &DataCenterActions{
			ViewOverView:      false,
			ViewInsight:       false,
			EditInsightConfig: false,
		},
	}
}

func modifyUserProjectAuth(userAuthInfo *ProjectActions, verb string) {
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

func modifySystemAction(systemActions *SystemActions, verb string) {
	switch verb {
	case VerbCreateProject:
		systemActions.Project.Create = true
	case VerbDeleteProject:
		systemActions.Project.Delete = true
	case VerbCreateTemplate:
		systemActions.Template.Create = true
	case VerbGetTemplate:
		systemActions.Template.View = true
	case VerbEditTemplate:
		systemActions.Template.Edit = true
	case VerbDeleteTemplate:
		systemActions.Template.Delete = true
	case VerbViewTestCenter:
		systemActions.TestCenter.View = true
	case VerbViewReleaseCenter:
		systemActions.ReleaseCenter.View = true
	case VerbDeliveryCenterGetVersions:
		systemActions.DeliveryCenter.ViewVersion = true
	case VerbDeliveryCenterGetArtifact:
		systemActions.DeliveryCenter.ViewArtifact = true
	case VerbGetDataCenterOverview:
		systemActions.DataCenter.ViewOverView = true
	case VerbGetDataCenterInsight:
		systemActions.DataCenter.ViewInsight = true
	case VerbEditDataCenterInsightConfig:
		systemActions.DataCenter.EditInsightConfig = true
	}
}
