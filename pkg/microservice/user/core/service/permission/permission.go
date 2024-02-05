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
	"fmt"

	"go.uber.org/zap"
	"gorm.io/gorm"
	"k8s.io/apimachinery/pkg/util/sets"

	aslanmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/user/core/repository"
	"github.com/koderover/zadig/v2/pkg/microservice/user/core/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/user/core/repository/orm"
	"github.com/koderover/zadig/v2/pkg/microservice/user/core/service/user"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/types"
)

type GetUserRulesByProjectResp struct {
	IsSystemAdmin       bool                `json:"is_system_admin"`
	IsProjectAdmin      bool                `json:"is_project_admin"`
	ProjectVerbs        []string            `json:"project_verbs"`
	WorkflowVerbsMap    map[string][]string `json:"workflow_verbs_map"`
	EnvironmentVerbsMap map[string][]string `json:"environment_verbs_map"`
}

func GetUserPermissionByProject(uid, projectName string, log *zap.SugaredLogger) (*GetUserRulesByProjectResp, error) {
	projectVerbSet := sets.NewString()

	groupIDList := make([]string, 0)
	// find the user groups this uid belongs to, if none it is ok
	groups, err := user.GetUserGroupByUID(uid)
	if err != nil {
		log.Errorf("failed to find user group for user: %s, error: %s", uid, err)
		return nil, fmt.Errorf("failed to get user permission, cannot find the user group for user, error: %s", err)
	}

	for _, groupID := range groups {
		groupIDList = append(groupIDList, groupID)
	}

	allUserGroup, err := user.GetAllUserGroup()
	if err != nil || allUserGroup == "" {
		log.Errorf("failed to find user group for %s, error: %s", "所有用户", err)
		return nil, fmt.Errorf("failed to find user group for %s, error: %s", "所有用户", err)
	}

	groupIDList = append(groupIDList, allUserGroup)

	// first find if the user is the system admin
	isSystemAdmin, err := checkUserIsSystemAdmin(uid, repository.DB)
	if err != nil {
		return nil, err
	}

	if isSystemAdmin {
		return &GetUserRulesByProjectResp{
			IsSystemAdmin: true,
		}, nil
	}

	// TODO: add some powerful cache here
	roleActionMap := make(map[uint]sets.String)

	// deal with the roles next
	// firstly, user itself.
	roles, err := ListRoleByUID(uid)
	if err != nil {
		log.Errorf("failed to find role for user: %s in project: %s, error: %s", uid, projectName, err)
		return nil, fmt.Errorf("failed to find role for user: %s in project: %s, error: %s", uid, projectName, err)
	}

	for _, role := range roles {
		if role.Namespace != projectName {
			continue
		}
		// if the user has project admin role,
		if role.Name == ProjectAdminRole {
			return &GetUserRulesByProjectResp{
				IsProjectAdmin: true,
			}, nil
		}
		actions, err := ListActionByRole(role.ID)
		if err != nil {
			log.Errorf("failed to find action bindings for role: %s, error: %s", role.Name, err)
			return nil, fmt.Errorf("failed to find action bindings for role: %s, error: %s", role.Name, err)
		}
		if _, ok := roleActionMap[role.ID]; !ok {
			roleActionMap[role.ID] = sets.NewString()
		}

		for _, action := range actions {
			projectVerbSet.Insert(action)
			roleActionMap[role.ID].Insert(action)
		}
	}

	// after dealing with the user's groups role bindings
	groupRoleMap := make(map[uint]*types.Role)

	for _, gid := range groupIDList {
		groupRoles, err := ListRoleByGID(gid)
		if err != nil {
			log.Errorf("failed to find role for user's group for user: %s in project: %s, error: %s", uid, projectName, err)
			return nil, fmt.Errorf("failed to find role for user's group for user: %s in project: %s, error: %s", uid, projectName, err)
		}

		for _, role := range groupRoles {
			groupRoleMap[role.ID] = role
		}
	}

	for _, role := range groupRoleMap {
		// if the user's group has project admin role,
		if role.Name == ProjectAdminRole {
			return &GetUserRulesByProjectResp{
				IsProjectAdmin: true,
			}, nil
		}
		// if the role has been seen previously, it has already been processed
		if _, ok := roleActionMap[role.ID]; ok {
			continue
		}

		actions, err := orm.ListActionByRole(role.ID, repository.DB)
		if err != nil {
			log.Errorf("failed to find action bindings for role: %s, error: %s", role.Name, err)
			return nil, fmt.Errorf("failed to find action bindings for role: %s, error: %s", role.Name, err)
		}

		roleActionMap[role.ID] = sets.NewString()
		for _, action := range actions {
			projectVerbSet.Insert(action.Action)
			roleActionMap[role.ID].Insert(action.Action)
		}
	}

	// finally check the collaboration instance, set all the permission granted by collaboration instance to the corresponding map
	collaborationInstance, err := mongodb.NewCollaborationInstanceColl().FindInstance(uid, projectName)
	if err != nil {
		// if no collaboration mode is found, simple ignore it, it is a warn level log, no necessarily an error.
		log.Warnf("failed to find collaboration instance for user: %s, error: %s", uid, err)
		return &GetUserRulesByProjectResp{
			ProjectVerbs: projectVerbSet.List(),
		}, nil
	}

	workflowMap := make(map[string][]string)
	envMap := make(map[string][]string)

	// TODO: currently this map will have some problems when there is a naming conflict between product workflow and common workflow. fix it.
	for _, workflow := range collaborationInstance.Workflows {
		workflowVerbs := make([]string, 0)
		for _, verb := range workflow.Verbs {
			// special case: if the user have workflow view permission in collaboration mode, we add read workflow permission in the resp
			if verb == types.WorkflowActionView {
				projectVerbSet.Insert(types.WorkflowActionView)
			}
			workflowVerbs = append(workflowVerbs, verb)
		}
		workflowMap[workflow.Name] = workflowVerbs
	}

	for _, env := range collaborationInstance.Products {
		envVerbs := make([]string, 0)
		for _, verb := range env.Verbs {
			// special case: if the user have env view permission in collaboration mode, we add read env permission in the resp
			if verb == types.EnvActionView {
				projectVerbSet.Insert(types.EnvActionView)
			}
			if verb == types.ProductionEnvActionView {
				projectVerbSet.Insert(types.ProductionEnvActionView)
			}
			envVerbs = append(envVerbs, verb)
		}
		envMap[env.Name] = envVerbs
	}

	return &GetUserRulesByProjectResp{
		ProjectVerbs:        projectVerbSet.List(),
		WorkflowVerbsMap:    workflowMap,
		EnvironmentVerbsMap: envMap,
	}, nil
}

type GetUserRulesResp struct {
	IsSystemAdmin    bool     `json:"is_system_admin"`
	ProjectAdminList []string `json:"project_admin_list"`
	SystemVerbs      []string `json:"system_verbs"`
}

func GetUserRules(uid string, log *zap.SugaredLogger) (*GetUserRulesResp, error) {
	var isSystemAdmin bool
	projectAdminList := make([]string, 0)
	// find the user groups this uid belongs to, if none it is ok
	groupIDList, err := user.GetUserGroupByUID(uid)
	if err != nil {
		log.Errorf("cannot find user group by uid: %s, error: %s", uid, err)
		return nil, err
	}

	allUserGroupID, err := user.GetAllUserGroup()
	if err != nil {
		log.Errorf("failed to find user group for %s, error: %s", "所有用户", err)
		return nil, fmt.Errorf("failed to find user group for %s, error: %s", "所有用户", err)
	}

	groupIDList = append(groupIDList, allUserGroupID)

	systemVerbs := make([]string, 0)
	roleActionMap := make(map[uint]sets.String)

	roles, err := ListRoleByUID(uid)
	if err != nil {
		log.Errorf("failed to list roles for uid: %s, error: %s", uid, err)
		return nil, fmt.Errorf("failed to list roles for uid: %s, error: %s", uid, err)
	}

	for _, role := range roles {
		// system admins
		if role.Namespace == GeneralNamespace && role.Name == AdminRole {
			isSystemAdmin = true
		}
		// project admins
		if role.Namespace != GeneralNamespace && role.Name == ProjectAdminRole {
			projectAdminList = append(projectAdminList, role.Namespace)
		}

		actions, err := ListActionByRole(role.ID)
		if err != nil {
			log.Errorf("failed to list action for role: %s in namespace %s, error: %s", role.Name, role.Namespace, err)
			return nil, err
		}

		if _, ok := roleActionMap[role.ID]; !ok {
			roleActionMap[role.ID] = sets.NewString()
		}

		for _, action := range actions {
			roleActionMap[role.ID].Insert(action)
		}

		switch role.Namespace {
		case GeneralNamespace:
			for _, action := range actions {
				systemVerbs = append(systemVerbs, action)
			}
		}
	}

	groupRoleMap := make(map[uint]*types.Role)

	for _, gid := range groupIDList {
		groupRoles, err := ListRoleByGID(gid)
		if err != nil {
			log.Errorf("failed to list user roles by group list for user: %s in group: %s, error: %s", uid, gid, err)
			return nil, fmt.Errorf("failed to list user roles by group list for user: %s in group: %s, error: %s", uid, gid, err)
		}

		for _, role := range groupRoles {
			groupRoleMap[role.ID] = role
		}
	}

	for _, role := range groupRoleMap {
		// system admins
		if role.Namespace == GeneralNamespace && role.Name == AdminRole {
			isSystemAdmin = true
			continue
		}
		// project admins
		if role.Namespace != GeneralNamespace && role.Name == ProjectAdminRole {
			projectAdminList = append(projectAdminList, role.Namespace)
			continue
		}

		// if the role has been seen previously, it has already been processed in user
		if _, ok := roleActionMap[role.ID]; ok {
			continue
		}

		actions, err := ListActionByRole(role.ID)
		if err != nil {
			log.Errorf("failed to find action bindings for role: %s, error: %s", role.Name, err)
			return nil, fmt.Errorf("failed to find action bindings for role: %s, error: %s", role.Name, err)
		}

		switch role.Namespace {
		case GeneralNamespace:
			for _, action := range actions {
				systemVerbs = append(systemVerbs, action)
			}
		}
	}

	return &GetUserRulesResp{
		IsSystemAdmin:    isSystemAdmin,
		SystemVerbs:      systemVerbs,
		ProjectAdminList: projectAdminList,
	}, nil
}

type TestingOpt struct {
	Name        string                  `json:"name"`
	ProductName string                  `json:"product_name"`
	Desc        string                  `json:"desc"`
	UpdateTime  int64                   `json:"update_time"`
	UpdateBy    string                  `json:"update_by"`
	TestCaseNum int                     `json:"test_case_num,omitempty"`
	ExecuteNum  int                     `json:"execute_num,omitempty"`
	PassRate    float64                 `json:"pass_rate,omitempty"`
	AvgDuration float64                 `json:"avg_duration,omitempty"`
	Workflows   []*aslanmodels.Workflow `json:"workflows,omitempty"`
	Verbs       []string                `json:"verbs"`
}

// checkUserIsSystemAdmin return if the user, or its user group, is a system admin.
func checkUserIsSystemAdmin(uid string, tx *gorm.DB) (bool, error) {
	groupIDList := make([]string, 0)
	// find the user groups this uid belongs to, if none it is ok
	groups, err := orm.ListUserGroupByUID(uid, tx)
	if err != nil {
		log.Errorf("failed to find user group for user: %s, error: %s", uid, err)
		return false, fmt.Errorf("failed to get user permission, cannot find the user group for user, error: %s", err)
	}

	for _, group := range groups {
		groupIDList = append(groupIDList, group.GroupID)
	}

	// first find if the user is the system admin
	systemAdminRole, err := orm.FindSystemAdminRole(tx)
	if err != nil || systemAdminRole.ID == 0 {
		log.Errorf("No system admin role found, error: %s", err)
		return false, fmt.Errorf("get user permission error: %s", err)
	}

	// find if user has a role binding on system admin
	rb, err := orm.GetRoleBinding(systemAdminRole.ID, uid, tx)
	if err != nil {
		log.Errorf("failed to query mysql to find if the user is admin, error: %s", err)
		return false, fmt.Errorf("failed to get user permission, cannot determine if the user is system admin, error: %s", err)
	}

	if rb.ID != 0 {
		return true, nil
	}

	// find if the user's group have a binding on system admin
	groupRBs, err := orm.ListGroupRoleBindingsByGroupsAndRoles(systemAdminRole.ID, groupIDList, tx)
	if err != nil {
		log.Errorf("failed to query mysql to find if the user's group is bound to system admin, error: %s", err)
		return false, fmt.Errorf("failed to get user permission, cannot determine if the user's group is system admin, error: %s", err)
	}

	return len(groupRBs) > 0, nil
}
