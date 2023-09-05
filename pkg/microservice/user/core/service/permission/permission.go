package permission

import (
	"fmt"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/koderover/zadig/pkg/microservice/user/core/repository"
	"github.com/koderover/zadig/pkg/microservice/user/core/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/user/core/repository/orm"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/koderover/zadig/pkg/types"
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

	tx := repository.DB.Begin()
	groupIDList := make([]string, 0)
	// find the user groups this uid belongs to, if none it is ok
	groups, err := orm.ListUserGroupByUID(uid, tx)
	if err != nil {
		log.Errorf("failed to find user group for user: %s, error: %s", uid, err)
		return nil, fmt.Errorf("failed to get user permission, cannot find the user group for user, error: %s", err)
	}

	for _, group := range groups {
		groupIDList = append(groupIDList, group.GroupID)
	}

	// first find if the user is the system admin
	isSystemAdmin, err := checkUserIsSystemAdmin(uid)
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
	roles, err := orm.ListRoleByUIDAndNamespace(uid, projectName, tx)
	if err != nil {
		log.Errorf("failed to find role for user: %s in project: %s, error: %s", uid, projectName, err)
		return nil, fmt.Errorf("failed to find role for user: %s in project: %s, error: %s", uid, projectName, err)
	}

	for _, role := range roles {
		// if the user has project admin role,
		if role.Name == ProjectAdminRole {
			return &GetUserRulesByProjectResp{
				IsProjectAdmin: true,
			}, nil
		}
		actions, err := orm.ListActionByRole(role.ID, repository.DB)
		if err != nil {
			log.Errorf("failed to find action bindings for role: %s, error: %s", role.Name, err)
			return nil, fmt.Errorf("failed to find action bindings for role: %s, error: %s", role.Name, err)
		}
		if _, ok := roleActionMap[role.ID]; !ok {
			roleActionMap[role.ID] = sets.NewString()
		}

		for _, action := range actions {
			projectVerbSet.Insert(action.Action)
			roleActionMap[role.ID].Insert(action.Action)
		}
	}

	// after dealing with the user's groups role bindings
	groupRoles, err := orm.ListRoleByGroupIDsAndNamespace(groupIDList, projectName, tx)
	if err != nil {
		log.Errorf("failed to find role for user's group for user: %s in project: %s, error: %s", uid, projectName, err)
		return nil, fmt.Errorf("failed to find role for user's group for user: %s in project: %s, error: %s", uid, projectName, err)
	}

	for _, role := range groupRoles {
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

	tx.Commit()

	return &GetUserRulesByProjectResp{
		ProjectVerbs:        projectVerbSet.List(),
		WorkflowVerbsMap:    workflowMap,
		EnvironmentVerbsMap: envMap,
	}, nil
}

type GetUserRulesResp struct {
	IsSystemAdmin    bool                `json:"is_system_admin"`
	ProjectAdminList []string            `json:"project_admin_list"`
	ProjectVerbMap   map[string][]string `json:"project_verb_map"`
	SystemVerbs      []string            `json:"system_verbs"`
}

func GetUserRules(uid string, log *zap.SugaredLogger) (*GetUserRulesResp, error) {
	var isSystemAdmin bool
	projectAdminList := make([]string, 0)
	tx := repository.DB.Begin()
	groupIDList := make([]string, 0)
	// find the user groups this uid belongs to, if none it is ok
	groups, err := orm.ListUserGroupByUID(uid, tx)
	if err != nil {
		log.Errorf("failed to find user group for user: %s, error: %s", uid, err)
		return nil, fmt.Errorf("failed to get user permission, cannot find the user group for user, error: %s", err)
	}

	for _, group := range groups {
		groupIDList = append(groupIDList, group.GroupID)
	}

	projectVerbMap := make(map[string][]string)
	systemVerbs := make([]string, 0)
	// TODO: add some powerful cache here
	roleActionMap := make(map[uint]sets.String)

	roles, err := orm.ListRoleByUID(uid, tx)
	if err != nil {
		tx.Rollback()
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

		// TODO: improve this with cache
		actions, err := orm.ListActionByRole(role.ID, tx)
		if err != nil {
			tx.Rollback()
			log.Errorf("failed to list action for role: %s in namespace %s, error: %s", role.Name, role.Namespace, err)
			return nil, err
		}

		if _, ok := roleActionMap[role.ID]; !ok {
			roleActionMap[role.ID] = sets.NewString()
		}

		for _, action := range actions {
			roleActionMap[role.ID].Insert(action.Action)
		}

		switch role.Namespace {
		case GeneralNamespace:
			for _, action := range actions {
				systemVerbs = append(systemVerbs, action.Action)
			}
		default:
			if _, ok := projectVerbMap[role.Namespace]; !ok {
				projectVerbMap[role.Namespace] = make([]string, 0)
			}
			for _, action := range actions {
				projectVerbMap[role.Namespace] = append(projectVerbMap[role.Namespace], action.Action)
			}
		}
	}

	groupRoles, err := orm.ListRoleByGroupIDs(groupIDList, tx)
	if err != nil {
		tx.Rollback()
		log.Errorf("failed to list user roles by group list for user: %s, error: %s", uid, err)
		return nil, fmt.Errorf("failed to list user roles by group list for user: %s, error: %s", uid, err)
	}

	for _, role := range groupRoles {
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

		actions, err := orm.ListActionByRole(role.ID, repository.DB)
		if err != nil {
			tx.Rollback()
			log.Errorf("failed to find action bindings for role: %s, error: %s", role.Name, err)
			return nil, fmt.Errorf("failed to find action bindings for role: %s, error: %s", role.Name, err)
		}

		switch role.Namespace {
		case GeneralNamespace:
			for _, action := range actions {
				systemVerbs = append(systemVerbs, action.Action)
			}
		default:
			if _, ok := projectVerbMap[role.Namespace]; !ok {
				projectVerbMap[role.Namespace] = make([]string, 0)
			}
			for _, action := range actions {
				projectVerbMap[role.Namespace] = append(projectVerbMap[role.Namespace], action.Action)
			}
		}
	}

	for project := range projectVerbMap {
		// collaboration mode is a special that does not have rule and verbs, we manually check if the user is permitted to
		// get workflow and environment
		workflowReadPermission, err := internalhandler.CheckPermissionGivenByCollaborationMode(uid, project, types.ResourceTypeWorkflow, types.WorkflowActionView)
		if err != nil {
			// there are cases where the users do not have any collaboration modes, hence no instances found
			// in these cases we just ignore the error, and set permission to false
			//log.Warnf("failed to read collaboration permission for project: %s, error: %s", project, err)
			workflowReadPermission = false
		}
		if workflowReadPermission {
			projectVerbMap[project] = append(projectVerbMap[project], types.WorkflowActionView)
		}

		envReadPermission, err := internalhandler.CheckPermissionGivenByCollaborationMode(uid, project, types.ResourceTypeEnvironment, types.EnvActionView)
		if err != nil {
			// there are cases where the users do not have any collaboration modes, hence no instances found
			// in these cases we just ignore the error, and set permission to false
			//log.Warnf("failed to read collaboration permission for project: %s, error: %s", project, err)
			envReadPermission = false
		}
		if envReadPermission {
			projectVerbMap[project] = append(projectVerbMap[project], types.EnvActionView)
		}
	}

	tx.Commit()
	return &GetUserRulesResp{
		IsSystemAdmin:    isSystemAdmin,
		ProjectVerbMap:   projectVerbMap,
		SystemVerbs:      systemVerbs,
		ProjectAdminList: projectAdminList,
	}, nil
}

// checkUserIsSystemAdmin return if the user, or its user group, is a system admin
func checkUserIsSystemAdmin(uid string) (bool, error) {
	tx := repository.DB.Begin()
	groupIDList := make([]string, 0)
	// find the user groups this uid belongs to, if none it is ok
	groups, err := orm.ListUserGroupByUID(uid, tx)
	if err != nil {
		tx.Commit()
		log.Errorf("failed to find user group for user: %s, error: %s", uid, err)
		return false, fmt.Errorf("failed to get user permission, cannot find the user group for user, error: %s", err)
	}

	for _, group := range groups {
		groupIDList = append(groupIDList, group.GroupID)
	}

	// first find if the user is the system admin
	systemAdminRole, err := orm.FindSystemAdminRole(tx)
	if err != nil || systemAdminRole.ID == 0 {
		tx.Commit()
		log.Errorf("No system admin role found, error: %s", err)
		return false, fmt.Errorf("get user permission error: %s", err)
	}

	// find if user has a role binding on system admin
	rb, err := orm.GetRoleBinding(systemAdminRole.ID, uid, tx)
	if err != nil {
		tx.Commit()
		log.Errorf("failed to query mysql to find if the user is admin, error: %s", err)
		return false, fmt.Errorf("failed to get user permission, cannot determine if the user is system admin, error: %s", err)
	}

	if rb.ID != 0 {
		tx.Commit()
		return true, nil
	}

	// find if the user's group have a binding on system admin
	groupRBs, err := orm.ListGroupRoleBindingsByGroupsAndRoles(systemAdminRole.ID, groupIDList, tx)
	if err != nil {
		tx.Commit()
		log.Errorf("failed to query mysql to find if the user's group is bound to system admin, error: %s", err)
		return false, fmt.Errorf("failed to get user permission, cannot determine if the user's group is system admin, error: %s", err)
	}

	tx.Commit()
	return len(groupRBs) > 0, nil
}
