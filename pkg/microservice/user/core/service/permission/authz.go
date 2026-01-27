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

import (
	"database/sql"
	"fmt"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/koderover/zadig/v2/pkg/microservice/user/core/repository"
	"github.com/koderover/zadig/v2/pkg/microservice/user/core/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/user/core/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/user/core/repository/orm"
	"github.com/koderover/zadig/v2/pkg/microservice/user/core/service/common"
	"github.com/koderover/zadig/v2/pkg/shared/client/user"
	"github.com/koderover/zadig/v2/pkg/types"
)

func GetUserAuthInfo(uid string, logger *zap.SugaredLogger) (*AuthorizedResources, error) {
	// system calls
	if uid == "" {
		return generateAdminRoleResource(), nil
	}

	isSystemAdmin, err := checkUserIsSystemAdmin(uid, repository.DB)
	if err != nil {
		logger.Errorf("failed to check if the user is system admin for uid: %s, error: %s", uid, err)
		return nil, fmt.Errorf("failed to check if the user is system admin for uid: %s, error: %s", uid, err)
	}

	if isSystemAdmin {
		return generateAdminRoleResource(), nil
	}

	// find the user groups this uid belongs to, if none it is ok
	groupIDList, err := common.GetUserGroupByUID(uid)
	if err != nil {
		logger.Errorf("failed to find user group for user: %s, error: %s", uid, err)
		return nil, fmt.Errorf("failed to get user permission, cannot find the user group for user, error: %s", err)
	}

	allUserGroup, err := common.GetAllUserGroup()
	if err != nil || allUserGroup == "" {
		logger.Errorf("failed to find user group for %s, error: %s", "所有用户", err)
		return nil, fmt.Errorf("failed to find user group for %s, error: %s", "所有用户", err)
	}

	groupIDList = append(groupIDList, allUserGroup)

	// generate system actions for user
	systemActions := generateDefaultSystemActions()
	// we generate a map of namespaced(project) permission
	projectActionMap := make(map[string]*ProjectActions)

	roles, err := ListRoleByUID(uid)
	if err != nil {
		logger.Errorf("failed to list roles for uid: %s, error: %s", uid, err)
		return nil, fmt.Errorf("failed to list roles for uid: %s, error: %s", uid, err)
	}

	for _, role := range roles {
		if role.Namespace != GeneralNamespace {
			if _, ok := projectActionMap[role.Namespace]; !ok {
				projectActionMap[role.Namespace] = generateDefaultProjectActions()
			}
		}

		// project admin does not have any bindings, it is special
		if role.Name == ProjectAdminRole {
			projectActionMap[role.Namespace].IsProjectAdmin = true
			continue
		}

		// first get actions from the roles
		actions, err := ListActionByRole(role.ID)
		if err != nil {
			logger.Errorf("failed to list action for role: %s in namespace %s, error: %s", role.Name, role.Namespace, err)
			return nil, err
		}

		for _, action := range actions {
			switch role.Namespace {
			case GeneralNamespace:
				modifySystemAction(systemActions, action)
			default:
				modifyUserProjectAuth(projectActionMap[role.Namespace], action)
			}
		}
	}

	groupRoleMap := make(map[uint]*types.Role)

	for _, gid := range groupIDList {
		groupRoles, err := ListRoleByGID(gid)
		if err != nil {
			logger.Errorf("failed to list user roles by group list for user: %s in group: %s, error: %s", uid, gid, err)
			return nil, fmt.Errorf("failed to list user roles by group list for user: %s in group: %s, error: %s", uid, gid, err)
		}

		for _, role := range groupRoles {
			groupRoleMap[role.ID] = role
		}
	}

	for _, role := range groupRoleMap {
		if role.Namespace != GeneralNamespace {
			if _, ok := projectActionMap[role.Namespace]; !ok {
				projectActionMap[role.Namespace] = generateDefaultProjectActions()
			}
		}

		if role.Name == ProjectAdminRole {
			projectActionMap[role.Namespace].IsProjectAdmin = true
			continue
		}

		// first get actions from the roles
		actions, err := ListActionByRole(role.ID)
		if err != nil {
			logger.Errorf("failed to list action for role: %s in namespace %s, error: %s", role.Name, role.Namespace, err)
			return nil, err
		}

		for _, action := range actions {
			switch role.Namespace {
			case GeneralNamespace:
				modifySystemAction(systemActions, action)
			default:
				modifyUserProjectAuth(projectActionMap[role.Namespace], action)
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
	collabInstances, findErr := mongodb.NewCollaborationInstanceColl().FindInstance(uid, projectKey)
	if findErr != nil {
		err = findErr
		return
	}

	workflows := make([]models.WorkflowCIItem, 0)
	envs := make([]models.ProductCIItem, 0)

	for _, collabInstance := range collabInstances {
		workflows = append(workflows, collabInstance.Workflows...)
		envs = append(envs, collabInstance.Products...)
	}

	switch resource {
	case types.ResourceTypeWorkflow:
		hasPermission = checkWorkflowPermission(workflows, resourceName, action)
	case types.ResourceTypeEnvironment:
		hasPermission = checkEnvPermission(envs, resourceName, action)
	default:
		return
	}
	return
}

func CheckPermissionGivenByCollaborationMode(uid, projectKey, resource, action string) (hasPermission bool, err error) {
	hasPermission = false
	collabInstances, findErr := mongodb.NewCollaborationInstanceColl().FindInstance(uid, projectKey)
	if findErr != nil {
		err = findErr
		return
	}

	for _, collabInstance := range collabInstances {
		if resource == types.ResourceTypeWorkflow {
			for _, workflow := range collabInstance.Workflows {
				for _, verb := range workflow.Verbs {
					if action == verb {
						hasPermission = true
						return
					}
				}
			}
		} else if resource == types.ResourceTypeEnvironment {
			for _, env := range collabInstance.Products {
				for _, verb := range env.Verbs {
					if action == verb {
						hasPermission = true
						return
					}
				}
			}
		}
	}

	return
}

func ListAuthorizedProject(uid string, logger *zap.SugaredLogger) ([]string, error) {
	tx := repository.DB.Begin(&sql.TxOptions{ReadOnly: true})

	respSet := sets.NewString()

	isSystemAdmin, err := checkUserIsSystemAdmin(uid, tx)
	if err != nil {
		tx.Rollback()
		logger.Errorf("failed to check if the user is system admin, error: %s", err)
		return nil, fmt.Errorf("failed to check if the user is system admin, error: %s", err)
	}

	if isSystemAdmin {
		projectList, err := mongodb.NewProjectColl().List()
		if err != nil {
			tx.Rollback()
			logger.Errorf("failed to list project for project admin to return authorized projects, error: %s", err)
			return nil, fmt.Errorf("failed to list project for project admin to return authorized projects, error: %s", err)
		}
		for _, project := range projectList {
			respSet.Insert(project.ProductName)
		}
		tx.Commit()
		return respSet.List(), nil
	}

	groupIDList := make([]string, 0)
	// find the user groups this uid belongs to, if none it is ok
	groups, err := orm.ListUserGroupByUID(uid, tx)
	if err != nil {
		tx.Rollback()
		logger.Errorf("failed to find user group for user: %s, error: %s", uid, err)
		return nil, fmt.Errorf("failed to get user permission, cannot find the user group for user, error: %s", err)
	}

	for _, group := range groups {
		groupIDList = append(groupIDList, group.GroupID)
	}

	allUserGroup, err := orm.GetAllUserGroup(tx)
	if err != nil || allUserGroup.GroupID == "" {
		tx.Rollback()
		logger.Errorf("failed to find user group for %s, error: %s", "所有用户", err)
		return nil, fmt.Errorf("failed to find user group for %s, error: %s", "所有用户", err)
	}

	groupIDList = append(groupIDList, allUserGroup.GroupID)

	roles, err := orm.ListRoleByUID(uid, tx)
	if err != nil {
		tx.Rollback()
		logger.Errorf("failed to list roles for uid: %s, error: %s", uid, err)
		return nil, fmt.Errorf("failed to list roles for uid: %s, error: %s", uid, err)
	}

	for _, role := range roles {
		if role.Namespace != GeneralNamespace {
			respSet.Insert(role.Namespace)
		}
	}

	groupRoles, err := orm.ListRoleByGroupIDs(groupIDList, tx)
	if err != nil {
		tx.Rollback()
		logger.Errorf("failed to find role for user's group for user: %s, error: %s", uid, err)
		return nil, fmt.Errorf("failed to find role for user's group for user: %s, error: %s", uid, err)
	}

	for _, role := range groupRoles {
		if role.Namespace != GeneralNamespace {
			respSet.Insert(role.Namespace)
		}
	}

	// TODO: add user group support for collaboration mode
	collaborationModeList, err := mongodb.NewCollaborationModeColl().ListUserCollaborationMode(uid)
	if err != nil {
		// this case is special, since the user might not have collaboration mode, we simply return the project list
		// given by the role.
		tx.Commit()
		logger.Warnf("failed to find user collaboration mode, error: %s", err)
		return respSet.List(), nil
	}

	// if user have collaboration mode, they must have access to this project.
	for _, collabMode := range collaborationModeList {
		respSet.Insert(collabMode.ProjectName)
	}

	tx.Commit()
	return respSet.List(), nil
}

func ListAuthorizedProjectByVerb(uid, resource, verb string, logger *zap.SugaredLogger) ([]string, error) {
	respSet := sets.NewString()

	tx := repository.DB.Begin(&sql.TxOptions{ReadOnly: true})

	isSystemAdmin, err := checkUserIsSystemAdmin(uid, tx)
	if err != nil {
		tx.Rollback()
		logger.Errorf("failed to check if the user is system admin, error: %s", err)
		return nil, fmt.Errorf("failed to check if the user is system admin, error: %s", err)
	}

	if isSystemAdmin {
		projectList, err := mongodb.NewProjectColl().List()
		if err != nil {
			tx.Rollback()
			logger.Errorf("failed to list project for project admin to return authorized projects, error: %s", err)
			return nil, fmt.Errorf("failed to list project for project admin to return authorized projects, error: %s", err)
		}
		for _, project := range projectList {
			respSet.Insert(project.ProductName)
		}
		tx.Commit()
		return respSet.List(), nil
	}

	groupIDList := make([]string, 0)
	// find the user groups this uid belongs to, if none it is ok
	groups, err := orm.ListUserGroupByUID(uid, tx)
	if err != nil {
		tx.Rollback()
		logger.Errorf("failed to find user group for user: %s, error: %s", uid, err)
		return nil, fmt.Errorf("failed to get user permission, cannot find the user group for user, error: %s", err)
	}

	for _, group := range groups {
		groupIDList = append(groupIDList, group.GroupID)
	}

	allUserGroup, err := orm.GetAllUserGroup(tx)
	if err != nil || allUserGroup.GroupID == "" {
		tx.Rollback()
		logger.Errorf("failed to find user group for %s, error: %s", "所有用户", err)
		return nil, fmt.Errorf("failed to find user group for %s, error: %s", "所有用户", err)
	}

	groupIDList = append(groupIDList, allUserGroup.GroupID)

	roles, err := orm.ListRoleByUIDAndVerb(uid, verb, tx)
	if err != nil {
		tx.Rollback()
		logger.Errorf("failed to list roles for uid: %s, error: %s", uid, err)
		return nil, fmt.Errorf("failed to list roles for uid: %s, error: %s", uid, err)
	}

	for _, role := range roles {
		if role.Namespace != GeneralNamespace {
			respSet.Insert(role.Namespace)
		}
	}

	adminRoles, err := orm.ListProjectAdminRoleByUID(uid, tx)
	if err != nil {
		tx.Rollback()
		logger.Errorf("failed to list roles for uid: %s, error: %s", uid, err)
		return nil, fmt.Errorf("failed to list roles for uid: %s, error: %s", uid, err)
	}

	for _, role := range adminRoles {
		if role.Namespace != GeneralNamespace {
			respSet.Insert(role.Namespace)
		}
	}

	groupRoles, err := orm.ListRoleByGroupIDsAndVerb(groupIDList, verb, tx)
	if err != nil {
		tx.Rollback()
		logger.Errorf("failed to list roles for groupid: %+v, error: %s", groupIDList, err)
		return nil, fmt.Errorf("failed to list roles for groupid: %+v, error: %s", groupIDList, err)
	}

	for _, role := range groupRoles {
		if role.Namespace != GeneralNamespace {
			respSet.Insert(role.Namespace)
		}
	}

	groupAdminRoles, err := orm.ListProjectAdminRoleByGroupIDs(groupIDList, tx)
	if err != nil {
		tx.Rollback()
		logger.Errorf("failed to list roles for uid: %s, error: %s", uid, err)
		return nil, fmt.Errorf("failed to list roles for uid: %s, error: %s", uid, err)
	}

	for _, role := range groupAdminRoles {
		if role.Namespace != GeneralNamespace {
			respSet.Insert(role.Namespace)
		}
	}

	if resource == types.ResourceTypeWorkflow || resource == types.ResourceTypeEnvironment {
		// TODO: after role-based permission is implemented, we should check for collaboration mode after, but just for workflow and envs.
	}

	tx.Commit()
	return respSet.List(), nil
}

// ListAuthorizedWorkflow lists all workflows authorized by collaboration mode
func ListAuthorizedWorkflow(uid, projectKey string, logger *zap.SugaredLogger) ([]string, []string, error) {
	collaborationInstances, err := mongodb.NewCollaborationInstanceColl().FindInstance(uid, projectKey)
	if err != nil {
		logger.Errorf("failed to find user collaboration mode, error: %s", err)
		return nil, nil, fmt.Errorf("failed to find user collaboration mode, error: %s", err)
	}

	authorizedWorkflows := make([]string, 0)
	authorizedCustomWorkflows := make([]string, 0)

	for _, collaborationInstance := range collaborationInstances {
		for _, workflow := range collaborationInstance.Workflows {
			for _, verb := range workflow.Verbs {
				// if the user actually has view permission
				if verb == types.WorkflowActionView {
					switch workflow.WorkflowType {
					case types.WorkflowTypeCustomeWorkflow:
						authorizedCustomWorkflows = append(authorizedCustomWorkflows, workflow.Name)
					default:
						// if a workflow does not have a type, it is a product workflow.
						authorizedWorkflows = append(authorizedWorkflows, workflow.Name)
					}
				}
			}
		}

	}

	return authorizedWorkflows, authorizedCustomWorkflows, nil
}

type ListAuthorizedWorkflowWithVerbResp struct {
	WorkflowList []string `json:"workflow_list"`
}

type AuthorizedWorkflowWithVerb struct {
	WorkflowName string   `json:"workflow_name"`
	Verbs        []string `json:"verbs"`
}

func ListAuthorizedWorkflowWithVerb(uid, projectKey string, logger *zap.SugaredLogger) (*user.CollModeAuthorizedWorkflowWithVerb, error) {
	collaborationInstances, err := mongodb.NewCollaborationInstanceColl().FindInstance(uid, projectKey)
	if err != nil {
		fmtErr := fmt.Errorf("failed to find user collaboration mode, error: %s", err)
		logger.Error(fmtErr)
		return &user.CollModeAuthorizedWorkflowWithVerb{
			ProjectWorkflowActionsMap: make(map[string]map[string]*user.WorkflowActions),
			Error:                     fmtErr.Error(),
		}, nil
	}

	resp := &user.CollModeAuthorizedWorkflowWithVerb{
		ProjectWorkflowActionsMap: make(map[string]map[string]*user.WorkflowActions),
	}

	for _, collaborationInstance := range collaborationInstances {
		for _, workflow := range collaborationInstance.Workflows {
			workflowActions := &user.WorkflowActions{}
			for _, verb := range workflow.Verbs {
				switch workflow.WorkflowType {
				case types.WorkflowTypeCustomeWorkflow:
					if verb == types.WorkflowActionView {
						workflowActions.View = true
					}
					if verb == types.WorkflowActionEdit {
						workflowActions.Edit = true
					}
					if verb == types.WorkflowActionRun {
						workflowActions.Execute = true
					}
					if verb == types.WorkflowActionDebug {
						workflowActions.Debug = true
					}
				}
				if _, ok := resp.ProjectWorkflowActionsMap[collaborationInstance.ProjectName]; !ok {
					resp.ProjectWorkflowActionsMap[collaborationInstance.ProjectName] = make(map[string]*user.WorkflowActions)
				}
				resp.ProjectWorkflowActionsMap[collaborationInstance.ProjectName][workflow.Name] = workflowActions
			}
		}
	}

	return resp, nil
}

func ListAuthorizedEnvs(uid, projectKey string, logger *zap.SugaredLogger) (readEnvList, editEnvList []string, err error) {
	readEnvList = make([]string, 0)
	editEnvList = make([]string, 0)

	readEnvSet := sets.NewString()
	editEnvSet := sets.NewString()
	collaborationInstances, findErr := mongodb.NewCollaborationInstanceColl().FindInstance(uid, projectKey)
	if findErr != nil {
		logger.Errorf("failed to find user collaboration mode, error: %s", findErr)
		err = fmt.Errorf("failed to find user collaboration mode, error: %s", findErr)
		return
	}

	for _, collaborationInstance := range collaborationInstances {
		for _, env := range collaborationInstance.Products {
			for _, verb := range env.Verbs {
				if verb == types.EnvActionView || verb == types.ProductionEnvActionView {
					readEnvSet.Insert(env.Name)
				}
				if verb == types.EnvActionEditConfig || verb == types.ProductionEnvActionEditConfig {
					editEnvSet.Insert(env.Name)
				}
			}
		}
	}

	readEnvList = readEnvSet.List()
	editEnvList = editEnvSet.List()
	return
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
		SprintTemplate: &SprintTemplateActions{
			Edit: false,
		},
		Sprint: &SprintActions{
			View:   false,
			Create: false,
			Edit:   false,
			Delete: false,
		},
		SprintWorkItem: &SprintWorkItemActions{
			Create: false,
			Edit:   false,
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
		ReleasePlan: &ReleasePlanActions{
			Create:       false,
			View:         false,
			Delete:       false,
			EditConfig:   false,
			EditMetadata: false,
			EditApproval: false,
			EditSubtasks: false,
		},
		BusinessDirectory: &BusinessDirectoryActions{
			View: false,
		},
		ClusterManagement: &ClusterManagementActions{
			Create: false,
			View:   false,
			Edit:   false,
			Delete: false,
		},
		VMManagement: &VMManagementActions{
			Create: false,
			View:   false,
			Edit:   false,
			Delete: false,
		},
		RegistryManagement: &RegistryManagementActions{
			Create: false,
			View:   false,
			Edit:   false,
			Delete: false,
		},
		S3StorageManagement: &S3StorageManagementActions{
			Create: false,
			View:   false,
			Edit:   false,
			Delete: false,
		},
		HelmRepoManagement: &HelmRepoManagementActions{
			Create: false,
			View:   false,
			Edit:   false,
			Delete: false,
		},
		DBInstanceManagement: &DBInstanceManagementActions{
			Create: false,
			View:   false,
			Edit:   false,
			Delete: false,
		},
		LabelManagement: &LabelManagementActions{
			Create: false,
			Edit:   false,
			Delete: false,
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
	case VerbEditSprintTemplate:
		userAuthInfo.SprintTemplate.Edit = true
	case VerbGetSprint:
		userAuthInfo.Sprint.View = true
	case VerbCreateSprint:
		userAuthInfo.Sprint.Create = true
	case VerbEditSprint:
		userAuthInfo.Sprint.Edit = true
	case VerbDeleteSprint:
		userAuthInfo.Sprint.Delete = true
	case VerbCreateSprintWorkItem:
		userAuthInfo.SprintWorkItem.Create = true
	case VerbEditSprintWorkItem:
		userAuthInfo.SprintWorkItem.Edit = true
	case VerbDeleteSprintWorkItem:
		userAuthInfo.SprintWorkItem.Delete = true
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
	case VerbCreateReleasePlan:
		systemActions.ReleasePlan.Create = true
	case VerbEditReleasePlanMetadata:
		systemActions.ReleasePlan.EditMetadata = true
	case VerbEditReleasePlanApproval:
		systemActions.ReleasePlan.EditApproval = true
	case VerbEditReleasePlanSubtasks:
		systemActions.ReleasePlan.EditSubtasks = true
	case VerbDeleteReleasePlan:
		systemActions.ReleasePlan.Delete = true
	case VerbGetReleasePlan:
		systemActions.ReleasePlan.View = true
	case VerbEditConfigReleasePlan:
		systemActions.ReleasePlan.EditConfig = true
	case VerbGetBusinessDirectory:
		systemActions.BusinessDirectory.View = true
	case VerbGetClusterManagement:
		systemActions.ClusterManagement.View = true
	case VerbCreateClusterManagement:
		systemActions.ClusterManagement.Create = true
	case VerbEditClusterManagement:
		systemActions.ClusterManagement.Edit = true
	case VerbDeleteClusterManagement:
		systemActions.ClusterManagement.Delete = true
	case VerbGetVMManagement:
		systemActions.VMManagement.View = true
	case VerbCreateVMManagement:
		systemActions.VMManagement.Create = true
	case VerbEditVMManagement:
		systemActions.VMManagement.Edit = true
	case VerbDeleteVMManagement:
		systemActions.VMManagement.Delete = true
	case VerbGetRegistryManagement:
		systemActions.RegistryManagement.View = true
	case VerbCreateRegistryManagement:
		systemActions.RegistryManagement.Create = true
	case VerbEditRegistryManagement:
		systemActions.RegistryManagement.Edit = true
	case VerbDeleteRegistryManagement:
		systemActions.RegistryManagement.Delete = true
	case VerbGetS3StorageManagement:
		systemActions.S3StorageManagement.View = true
	case VerbCreateS3StorageManagement:
		systemActions.S3StorageManagement.Create = true
	case VerbEditS3StorageManagement:
		systemActions.S3StorageManagement.Edit = true
	case VerbDeleteS3StorageManagement:
		systemActions.S3StorageManagement.Delete = true
	case VerbGetHelmRepoManagement:
		systemActions.HelmRepoManagement.View = true
	case VerbCreateHelmRepoManagement:
		systemActions.HelmRepoManagement.Create = true
	case VerbEditHelmRepoManagement:
		systemActions.HelmRepoManagement.Edit = true
	case VerbDeleteHelmRepoManagement:
		systemActions.HelmRepoManagement.Delete = true
	case VerbGetDBInstanceManagement:
		systemActions.DBInstanceManagement.View = true
	case VerbCreateDBInstanceManagement:
		systemActions.DBInstanceManagement.Create = true
	case VerbEditDBInstanceManagement:
		systemActions.DBInstanceManagement.Edit = true
	case VerbDeleteDBInstanceManagement:
		systemActions.DBInstanceManagement.Delete = true
	case VerbCreateLabelSetting:
		systemActions.LabelManagement.Create = true
	case VerbEditLabelSetting:
		systemActions.LabelManagement.Edit = true
	case VerbDeleteLabelSetting:
		systemActions.LabelManagement.Delete = true
	}
}
