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
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/koderover/zadig/v2/pkg/config"
	"github.com/koderover/zadig/v2/pkg/microservice/user/core/repository"
	"github.com/koderover/zadig/v2/pkg/microservice/user/core/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/user/core/repository/orm"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/cache"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/types"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"k8s.io/apimachinery/pkg/util/sets"
)

const (
	RoleActionKeyFormat = "role_action_%d"

	UIDRoleKeyFormat  = "uid_role_%s"
	UIDRoleDataFormat = "%d++%s++%s"
	UIDRoleLock       = "lock_uid_role_%s"

	GIDRoleKeyFormat  = "gid_role_%s"
	GIDRoleDataFormat = "%d++%s++%s"
	GIDRoleLock       = "lock_gid_role_%s"
)

// ActionMap is the local cache for all the actions' ID, the key is the action name
// Note that there is no way to change action after the service start, the local cache won't
// have an expiration mechanism.
var ActionMap = make(map[string]uint)

type CreateRoleReq struct {
	Name      string   `json:"name"`
	Actions   []string `json:"actions"`
	Namespace string   `json:"namespace"`
	Desc      string   `json:"desc,omitempty"`
	Type      string   `json:"type,omitempty"`
}

// ListRoleByUID lists all roles by uid with cache.
// WARNING: this function only returns roleID and namespace, DO NOT use other fields.
func ListRoleByUID(uid string) ([]*types.Role, error) {
	uidRoleKey := fmt.Sprintf(UIDRoleKeyFormat, uid)
	roleCache := cache.NewRedisCache(config.RedisCommonCacheTokenDB())

	useCache := true
	response := make([]*types.Role, 0)
	// check if the cache has been set
	exists, err := roleCache.Exists(uidRoleKey)
	if err == nil && exists {
		resp, err2 := roleCache.ListSetMembers(uidRoleKey)
		if err2 == nil {
			// if we got the data from cache, simply return it\
			for _, roleInfo := range resp {
				roleInfos := strings.Split(roleInfo, "++")
				if len(roleInfos) != 3 {
					// if the data is corrupted, stop using it.
					useCache = false
					break
				}

				roleID, err := strconv.Atoi(roleInfos[0])
				if err != nil {
					log.Warnf("invalid role id: %s", roleInfos[0])
					useCache = false
					break
				}

				response = append(response, &types.Role{
					ID:        uint(roleID),
					Namespace: roleInfos[1],
					Name:      roleInfos[2],
				})
			}
		} else {
			useCache = false
		}
	} else {
		useCache = false
	}

	if useCache {
		return response, nil
	}

	// if we don't use cache, flush the data
	response = make([]*types.Role, 0)
	cacheData := make([]string, 0)

	roles, err := orm.ListRoleByUID(uid, repository.DB)
	if err != nil {
		return nil, err
	}

	for _, role := range roles {
		response = append(response, &types.Role{
			ID:        role.ID,
			Namespace: role.Namespace,
			Name:      role.Name,
		})
		cacheData = append(cacheData, fmt.Sprintf(UIDRoleDataFormat, role.ID, role.Namespace, role.Name))
	}

	err = roleCache.Delete(uidRoleKey)
	if err != nil {
		log.Warnf("failed to flush cache for key: %s, err: %s", uidRoleKey, err)
		return response, nil
	}

	err = roleCache.AddElementsToSet(uidRoleKey, cacheData, setting.CacheExpireTime)
	if err != nil {
		log.Warnf("failed to add cache to key: %s, err: %s", uidRoleKey, err)
	}

	return response, nil
}

// ListRoleByGID lists all roles by gid with cache.
// WARNING: this function only returns roleID and namespace, DO NOT use other fields.
func ListRoleByGID(gid string) ([]*types.Role, error) {
	gidRoleKey := fmt.Sprintf(GIDRoleKeyFormat, gid)
	roleCache := cache.NewRedisCache(config.RedisCommonCacheTokenDB())

	useCache := true
	response := make([]*types.Role, 0)
	// check if the cache has been set
	exists, err := roleCache.Exists(gidRoleKey)
	if err == nil && exists {
		resp, err2 := roleCache.ListSetMembers(gidRoleKey)
		if err2 == nil {
			// if we got the data from cache, simply return it\
			for _, roleInfo := range resp {
				roleInfos := strings.Split(roleInfo, "++")
				if len(roleInfos) != 3 {
					// if the data is corrupted, stop using it.
					useCache = false
					break
				}

				roleID, err := strconv.Atoi(roleInfos[0])
				if err != nil {
					log.Warnf("invalid role id: %s", roleInfos[0])
					useCache = false
					break
				}

				response = append(response, &types.Role{
					ID:        uint(roleID),
					Namespace: roleInfos[1],
					Name:      roleInfos[2],
				})
			}
		} else {
			useCache = false
		}
	} else {
		useCache = false
	}

	if useCache {
		return response, nil
	}

	// if we don't use cache, flush the data
	response = make([]*types.Role, 0)
	cacheData := make([]string, 0)

	roles, err := orm.ListRoleByGroupIDs([]string{gid}, repository.DB)
	if err != nil {
		return nil, err
	}

	for _, role := range roles {
		response = append(response, &types.Role{
			ID:        role.ID,
			Namespace: role.Namespace,
			Name:      role.Name,
		})
		cacheData = append(cacheData, fmt.Sprintf(GIDRoleDataFormat, role.ID, role.Namespace, role.Name))
	}

	err = roleCache.Delete(gidRoleKey)
	if err != nil {
		log.Warnf("failed to flush cache for key: %s, err: %s", gidRoleKey, err)
		return response, nil
	}

	err = roleCache.AddElementsToSet(gidRoleKey, cacheData, setting.CacheExpireTime)
	if err != nil {
		log.Warnf("failed to add cache to key: %s, err: %s", gidRoleKey, err)
	}

	return response, nil
}

// ListActionByRole list all actions permitted by a role ID with cache.
// note: since now global action and projected action are mutually exclusive in a role, we use this function
// change this function if necessary.
func ListActionByRole(roleID uint) ([]string, error) {
	roleActionKey := fmt.Sprintf(RoleActionKeyFormat, roleID)
	actionCache := cache.NewRedisCache(config.RedisCommonCacheTokenDB())

	// check if the cache has been set
	exists, err := actionCache.Exists(roleActionKey)
	if err == nil && exists {
		resp, err2 := actionCache.ListSetMembers(roleActionKey)
		if err2 == nil {
			// if we got the data from cache, simply return it
			return resp, nil
		}
	}

	roleTemplateBind, err := orm.GetRoleTemplateBindingByRoleID(roleID, repository.DB)
	if err != nil {
		return nil, fmt.Errorf("failed to find role template binding for role: %d, error: %s", roleID, err)
	}
	var actions []*models.Action
	if roleTemplateBind != nil {
		actions, err = orm.ListActionByRoleTemplate(roleTemplateBind.RoleTemplateID, repository.DB)
	} else {
		actions, err = orm.ListActionByRole(roleID, repository.DB)
	}

	if err != nil {
		log.Errorf("failed to list actions by role id: %d from database, error: %s", roleID, err)
		return nil, fmt.Errorf("failed to list actions by role id: %d from database, error: %s", roleID, err)
	}

	resp := make([]string, 0)
	for _, action := range actions {
		resp = append(resp, action.Action)
	}

	err = actionCache.AddElementsToSet(roleActionKey, resp, setting.CacheExpireTime)
	if err != nil {
		// nothing should be returned since setting data into cache does not affect final result
		log.Warnf("failed to add actions into role-action cache, error: %s", err)
	}

	return resp, nil
}

func CreateRole(ns string, req *CreateRoleReq, log *zap.SugaredLogger) error {
	tx := repository.DB.Begin()

	role := &models.NewRole{
		Name:        req.Name,
		Description: req.Desc,
		Namespace:   ns,
	}

	if req.Type == string(setting.ResourceTypeSystem) {
		role.Type = int64(setting.RoleTypeSystem)
	} else {
		role.Type = int64(setting.RoleTypeCustom)
	}

	if ns != "*" {
		roleTemplate, err := orm.GetRoleTemplate(req.Name, tx)
		if err == nil && roleTemplate.ID > 0 {
			tx.Rollback()
			return fmt.Errorf("can't create role with the same name as golbal role: %s", req.Name)
		}
	}

	err := orm.CreateRole(role, tx)
	if err != nil {
		log.Errorf("failed to create role, error: %s", err)
		tx.Rollback()
		return fmt.Errorf("failed to create role, error: %s", err)
	}

	actionIDList := make([]uint, 0)
	actionList := make([]string, 0)
	for _, action := range req.Actions {
		// if the action is not in the action cache, get one.
		if _, ok := ActionMap[action]; !ok {
			act, err := orm.GetActionByVerb(action, repository.DB)
			if err != nil {
				log.Errorf("failed to find verb: %s in request, action might not exist.", action)
				tx.Rollback()
				return fmt.Errorf("failed to find verb: %s in request, action might not exist", action)
			}
			ActionMap[action] = act.ID
		}
		actionIDList = append(actionIDList, ActionMap[action])
		actionList = append(actionList, action)
	}

	err = orm.BulkCreateRoleActionBindings(role.ID, actionIDList, tx)
	if err != nil {
		log.Errorf("failed to create action binding for role: %s in namespace: %s, the error is: %s", role.Name, role.Namespace, err)
		tx.Rollback()
		return fmt.Errorf("failed to create action binding for role: %s in namespace: %s, the error is: %s", role.Name, role.Namespace, err)
	}

	tx.Commit()

	// after committing to db, save it to the cache if possible
	roleActionKey := fmt.Sprintf(RoleActionKeyFormat, role.ID)
	actionCache := cache.NewRedisCache(config.RedisCommonCacheTokenDB())

	err = actionCache.Delete(roleActionKey)
	if err != nil {
		log.Warnf("failed to flush role-action cache, key: %s, error: %s", roleActionKey, err)
	}

	go func(key string, redisCache *cache.RedisCache) {
		time.Sleep(2 * time.Second)
		redisCache.Delete(key)
	}(roleActionKey, actionCache)

	return nil
}

// UpdateRole updates the role and its action binding.
func UpdateRole(ns string, req *CreateRoleReq, log *zap.SugaredLogger) error {
	tx := repository.DB.Begin()

	// Doing a tricky thing here: removing the whole role-action binding, then re-adding them.
	roleInfo, err := orm.GetRole(req.Name, ns, repository.DB)
	if err != nil {
		log.Errorf("failed to find role: [%s] in namespace [%s], error: %s", req.Namespace, ns, err)
		tx.Rollback()
		return fmt.Errorf("failed to find role: [%s] in namespace [%s], error: %s", req.Namespace, ns, err)
	}

	if roleInfo.Type == int64(setting.RoleTypeSystem) {
		tx.Rollback()
		return fmt.Errorf("can't update system role: %s", req.Name)
	}

	err = orm.DeleteRoleActionBindingByRole(roleInfo.ID, tx)
	if err != nil {
		log.Errorf("failed to delete role-action binding for role: %s, error: %s", roleInfo.Name, err)
		tx.Rollback()
		return fmt.Errorf("update role-action binding failed, error: %s", err)
	}

	actionIDList := make([]uint, 0)
	actionList := make([]string, 0)
	for _, action := range req.Actions {
		// if the action is not in the action cache, get one.
		if _, ok := ActionMap[action]; !ok {
			act, err := orm.GetActionByVerb(action, repository.DB)
			if err != nil {
				log.Errorf("failed to find verb: %s in request, action might not exist.", action)
				tx.Rollback()
				return fmt.Errorf("failed to find verb: %s in request, action might not exist", action)
			}
			ActionMap[action] = act.ID
		}
		actionIDList = append(actionIDList, ActionMap[action])
		actionList = append(actionList, action)
	}

	err = orm.BulkCreateRoleActionBindings(roleInfo.ID, actionIDList, tx)
	if err != nil {
		log.Errorf("failed to create action binding for role: %s in namespace: %s, the error is: %s", roleInfo.Name, roleInfo.Namespace, err)
		tx.Rollback()
		return fmt.Errorf("failed to create action binding for role: %s in namespace: %s, the error is: %s", roleInfo.Name, roleInfo.Namespace, err)
	}

	// so the only field capable of changing is the description....
	err = orm.UpdateRoleInfo(roleInfo.ID, &models.NewRole{
		Description: req.Desc,
	}, tx)

	tx.Commit()

	// after committing to db, save it to the cache if possible
	roleActionKey := fmt.Sprintf(RoleActionKeyFormat, roleInfo.ID)
	actionCache := cache.NewRedisCache(config.RedisCommonCacheTokenDB())

	err = actionCache.Delete(roleActionKey)
	if err != nil {
		log.Warnf("failed to flush actions from role-action cache, error: %s", err)
	}

	go func(key string, redisCache *cache.RedisCache) {
		time.Sleep(2 * time.Second)
		redisCache.Delete(key)
	}(roleActionKey, actionCache)

	return nil
}

// ListRolesByNamespace list roles
// For roles in projects, system roles will be returned as lazy initialization
func ListRolesByNamespace(projectName string, log *zap.SugaredLogger) ([]*types.Role, error) {
	roles, err := orm.ListRoleByNamespace(projectName, repository.DB)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		log.Errorf("failed to list roles in project: %s, error: %s", projectName, err)
		return nil, fmt.Errorf("failed to list roles in project: %s, error: %s", projectName, err)
	}

	roles, err = lazySyncRoleTemplates(projectName, roles)
	if err != nil {
		log.Errorf("failed to sync role templates, error: %s", err)
		return nil, fmt.Errorf("failed to sync role templates, error: %s", err)
	}

	sort.Slice(roles, func(i, j int) bool {
		if roles[i].Type == int64(setting.RoleTypeCustom) && roles[j].Type == int64(setting.RoleTypeSystem) {
			return true
		}
		if roles[i].Type == int64(setting.RoleTypeSystem) && roles[j].Type == int64(setting.RoleTypeCustom) {
			return false
		}
		return roles[i].ID < roles[j].ID
	})

	resp := make([]*types.Role, 0)
	for _, role := range roles {
		resp = append(resp, &types.Role{
			ID:          role.ID,
			Name:        role.Name,
			Namespace:   role.Namespace,
			Description: role.Description,
			Type:        convertDBRoleType(role.Type),
		})
	}

	return resp, nil
}

func lazySyncRoleTemplates(namespace string, roles []*models.NewRole) ([]*models.NewRole, error) {
	if namespace == "*" {
		return roles, nil
	}

	roleSet := sets.NewString()
	for _, role := range roles {
		roleSet.Insert(role.Name)
	}

	ret := make([]*models.NewRole, 0)
	for _, role := range roles {
		ret = append(ret, role)
	}

	tx := repository.DB.Begin()

	rolesNeedCreate := make([]*models.NewRole, 0)
	roleTemplates, err := orm.ListRoleTemplates(tx)
	if err != nil {
		log.Errorf("failed to list role templates, error: %s", err)
		return nil, fmt.Errorf("failed to list role templates, error: %s", err)
	}
	roleTemplateMap := make(map[string]uint)

	for _, roleTemplate := range roleTemplates {
		if roleSet.Has(roleTemplate.Name) {
			continue
		}
		roleTemplateMap[roleTemplate.Name] = roleTemplate.ID
		rolesNeedCreate = append(rolesNeedCreate, &models.NewRole{
			Name:        roleTemplate.Name,
			Description: roleTemplate.Description,
			Namespace:   namespace,
			Type:        int64(setting.RoleTypeSystem),
		})
	}

	if len(rolesNeedCreate) == 0 {
		tx.Rollback()
		return roles, nil
	}

	err = orm.BulkCreateRole(rolesNeedCreate, tx)
	if err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to create roles from template, error: %s", err)
	}

	templateRoleBinds := make([]*models.RoleTemplateBinding, 0)
	for _, role := range rolesNeedCreate {
		templateRoleBinds = append(templateRoleBinds, &models.RoleTemplateBinding{
			RoleID:         role.ID,
			RoleTemplateID: roleTemplateMap[role.Name],
		})
		ret = append(ret, role)
	}

	err = orm.BulkCreateRoleTemplateBindings(templateRoleBinds, tx)
	if err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to create role-template bindings, error: %s", err)
	}

	tx.Commit()

	return ret, nil
}

func ListRolesByNamespaceAndUserID(projectName, uid string, log *zap.SugaredLogger) ([]*types.Role, error) {
	roles, err := orm.ListRoleByUIDAndNamespace(uid, projectName, repository.DB)
	if err != nil && err != gorm.ErrRecordNotFound {
		log.Errorf("failed to list roles in project: %s, error: %s", projectName, err)
		return nil, fmt.Errorf("failed to list roles in project: %s, error: %s", projectName, err)
	}

	resp := make([]*types.Role, 0)
	for _, role := range roles {
		resp = append(resp, &types.Role{
			ID:          role.ID,
			Name:        role.Name,
			Namespace:   role.Namespace,
			Description: role.Description,
			Type:        convertDBRoleType(role.Type),
		})
	}

	return resp, nil
}

func GetRole(ns, name string, log *zap.SugaredLogger) (*types.DetailedRole, error) {
	role, err := orm.GetRole(name, ns, repository.DB)
	if err != nil {
		log.Errorf("failed to find role: %s under namespace: %s, error: %s", name, ns, err)
		return nil, fmt.Errorf("failed to find role: %s under namespace: %s, error: %s", name, ns, err)
	}

	actionList, err := orm.ListActionByRole(role.ID, repository.DB)
	if err != nil {
		log.Errorf("failed to find action for role: %s under namespace: %s, error: %s", name, ns, err)
		return nil, fmt.Errorf("failed to find action for role: %s under namespace: %s, error: %s", name, ns, err)
	}

	actionMap := make(map[string]sets.String)

	for _, action := range actionList {
		if _, ok := actionMap[action.Resource]; !ok {
			actionMap[action.Resource] = sets.NewString()
		}

		actionMap[action.Resource].Insert(action.Action)
	}

	resourceActionList := make([]*types.ResourceAction, 0)
	for resource, actionSet := range actionMap {
		resourceActionList = append(resourceActionList, &types.ResourceAction{
			Resource: resource,
			Verbs:    actionSet.List(),
		})
	}

	resp := &types.DetailedRole{
		ID:              role.ID,
		Name:            role.Name,
		Namespace:       role.Namespace,
		Description:     role.Description,
		Type:            convertDBRoleType(role.Type),
		ResourceActions: resourceActionList,
	}

	return resp, nil
}

func DeleteRole(name string, projectName string, log *zap.SugaredLogger) error {
	role, err := orm.GetRole(name, projectName, repository.DB)
	if err != nil {
		log.Errorf("failed to get role: %s under namespace %s, error: %s", name, projectName, err)
		return fmt.Errorf("failed to get role: %s under namespace %s, error: %s", name, projectName, err)
	}

	err = orm.DeleteRoleByName(name, projectName, repository.DB)
	if err != nil {
		log.Errorf("failed to delete role: %s under namespace %s, error: %s", name, projectName, err)
		return fmt.Errorf("failed to delete role: %s under namespace %s, error: %s", name, projectName, err)
	}

	roleActionKey := fmt.Sprintf(RoleActionKeyFormat, role.ID)
	actionCache := cache.NewRedisCache(config.RedisCommonCacheTokenDB())

	// check if the cache has been set
	err = actionCache.Delete(roleActionKey)
	if err != nil {
		log.Warnf("failed to flush actions from role-action cache, error: %s", err)
	}

	go func(key string, redisCache *cache.RedisCache) {
		time.Sleep(2 * time.Second)
		redisCache.Delete(key)
	}(roleActionKey, actionCache)

	return nil
}

func BatchDeleteRole(roles []*models.NewRole, db *gorm.DB, log *zap.SugaredLogger) error {
	// if nothing to delete, just return
	if len(roles) == 0 {
		return nil
	}
	err := orm.DeleteRoleByIDList(roles, db)
	if err != nil {
		log.Errorf("failed to batch delete roles, error: %s", err)
		return fmt.Errorf("failed to batch delete roles, error: %s", err)
	}
	return nil
}

func CreateDefaultRolesForNamespace(namespace string, log *zap.SugaredLogger) error {
	projectAdminRole := &models.NewRole{
		Name:        "project-admin",
		Description: "拥有指定项目中任何操作的权限",
		Type:        int64(setting.RoleTypeSystem),
		Namespace:   namespace,
	}
	readOnlyRole := &models.NewRole{
		Name:        "read-only",
		Description: "拥有指定项目中所有资源的读权限",
		Type:        int64(setting.RoleTypeSystem),
		Namespace:   namespace,
	}
	readProjectOnlyRole := &models.NewRole{
		Name:        "read-project-only",
		Description: "拥有指定项目本身的读权限，无权限查看和操作项目内资源",
		Type:        int64(setting.RoleTypeSystem),
		Namespace:   namespace,
	}

	err := orm.BulkCreateRole([]*models.NewRole{projectAdminRole, readOnlyRole, readProjectOnlyRole}, repository.DB)
	if err != nil {
		log.Errorf("failed to create system default role for project: %s, error: %s", namespace, err)
		return fmt.Errorf("failed to create system default role for project: %s, error: %s", namespace, err)
	}

	return nil
}

func DeleteAllRolesInNamespace(namespace string, log *zap.SugaredLogger) error {
	err := orm.DeleteRoleByNameSpace(namespace, repository.DB)
	if err != nil {
		log.Errorf("failed to delete role under namespace %s, error: %s", namespace, err)
		return fmt.Errorf("failed to delete role under namespace %s, error: %s", namespace, err)
	}
	return nil
}

func convertDBRoleType(tid int64) string {
	if tid == int64(setting.RoleTypeSystem) {
		return string(setting.ResourceTypeSystem)
	}
	return string(setting.ResourceTypeCustom)
}
