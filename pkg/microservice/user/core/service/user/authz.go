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
					// since every rule is only associated with one resource, we just read the first one
					switch rule.Resources[0]{
					case
					}
				}
			}
		}
	}

	return nil, nil
}

func generateAdminRoleResource() *AuthorizedResources {
	return &AuthorizedResources{
		IsSystemAdmin:      true,
		ProjectAuthInfo:    nil,
		AdditionalResource: nil,
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
			Debug:      false,
		},
		ProductionEnv: &ProductionEnvActions{
			View:       false,
			Create:     false,
			EditConfig: false,
			ManagePods: false,
			Delete:     false,
			Debug:      false,
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
