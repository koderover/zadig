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
	"time"

	"github.com/koderover/zadig/v2/pkg/config"
	"github.com/koderover/zadig/v2/pkg/tool/cache"
	"github.com/koderover/zadig/v2/pkg/types"
	"go.uber.org/zap"

	"github.com/koderover/zadig/v2/pkg/microservice/user/core/repository"
	"github.com/koderover/zadig/v2/pkg/microservice/user/core/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/user/core/repository/orm"
	"github.com/koderover/zadig/v2/pkg/setting"
)

var readOnlyAction = []string{
	VerbGetDelivery,
	VerbGetTest,
	VerbGetService,
	VerbGetProductionService,
	VerbGetBuild,
	VerbGetWorkflow,
	VerbGetEnvironment,
	VerbGetProductionEnv,
	VerbGetScan,
}

func InitializeProjectAuthorization(namespace string, isPublic bool, admins []string, log *zap.SugaredLogger) error {
	tx := repository.DB.Begin()
	// First, create default roles
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

	err := orm.BulkCreateRole([]*models.NewRole{projectAdminRole, readOnlyRole, readProjectOnlyRole}, tx)
	if err != nil {
		tx.Rollback()
		log.Errorf("failed to create system default role for project: %s, error: %s", namespace, err)
		return fmt.Errorf("failed to create system default role for project: %s, error: %s", namespace, err)
	}

	actionIDList := make([]uint, 0)
	for _, verb := range readOnlyAction {
		actionDetail, err := orm.GetActionByVerb(verb, tx)
		if err != nil || actionDetail.ID == 0 {
			tx.Rollback()
			log.Errorf("failed to find action %s for read-only, error: %s", verb, err)
			return fmt.Errorf("failed to find action %s for read-only, error: %s", verb, err)
		}

		actionIDList = append(actionIDList, actionDetail.ID)
	}

	err = orm.BulkCreateRoleActionBindings(readOnlyRole.ID, actionIDList, tx)
	if err != nil {
		tx.Rollback()
		log.Errorf("failed to create role action binding for read-only role, err: %s", err)
		return fmt.Errorf("failed to create role action binding for read-only role, err: %s", err)
	}

	// then, create role bindings if the project is public
	if isPublic {
		group, err := orm.GetUserGroupByName(types.AllUserGroupName, tx)
		if err != nil {
			tx.Rollback()
			log.Errorf("failed to find all-user group, error: %s", err)
			return fmt.Errorf("failed to find all-user group, error: %s", err)
		}

		err = orm.CreateGroupRoleBinding(&models.GroupRoleBinding{
			GroupID: group.GroupID,
			RoleID:  readOnlyRole.ID,
		}, tx)

		if err != nil {
			tx.Rollback()
			log.Errorf("failed to bind read-only role to all-users, error: %s", err)
			return fmt.Errorf("failed to bind read-only role to all-users, error: %s", err)
		}
	}

	// finally create role bindings
	err = orm.BulkCreateRoleBindingForRole(projectAdminRole.ID, admins, tx)
	if err != nil {
		tx.Rollback()
		log.Errorf("failed to bind project-admin role to given user list, error: %s", err)
		return fmt.Errorf("failed to bind project-admin role to given user list, error: %s", err)
	}

	roleCache := cache.NewRedisCache(config.RedisCommonCacheTokenDB())
	// flush cache for every identity that is affected
	for _, uid := range admins {
		uidRoleKey := fmt.Sprintf(UIDRoleKeyFormat, uid)
		err = roleCache.Delete(uidRoleKey)
		if err != nil {
			log.Warnf("failed to flush user-role cache for key: %s, error: %s", uidRoleKey, err)
		}
	}

	tx.Commit()

	go func(uids []string, redisCache *cache.RedisCache) {
		time.Sleep(2 * time.Second)

		for _, uid := range uids {
			uidRoleKey := fmt.Sprintf(UIDRoleKeyFormat, uid)
			err = roleCache.Delete(uidRoleKey)
		}
	}(admins, roleCache)

	return nil
}

func SetProjectVisibility(namespace string, visible bool, log *zap.SugaredLogger) error {
	tx := repository.DB.Begin()

	log.Infof("change namespace: %s to visibility: %+v", namespace, visible)

	group, err := orm.GetUserGroupByName(types.AllUserGroupName, tx)
	if err != nil {
		tx.Rollback()
		log.Errorf("failed to find all-user group, error: %s", err)
		return fmt.Errorf("failed to find all-user group, error: %s", err)
	}

	role, err := orm.GetRole("read-only", namespace, tx)
	if err != nil {
		tx.Rollback()
		log.Errorf("failed to find read-only role, error: %s", err)
		return fmt.Errorf("failed to find read-only role, error: %s", err)
	}

	if role.ID == 0 || len(group.GroupID) == 0 {
		tx.Rollback()
		log.Errorf("failed to find role or group, error: %s", err)
		return fmt.Errorf("failed to find role or group, error: %s", err)
	}

	groupRoleBinding, err := orm.GetGroupRoleBinding(group.GroupID, role.ID, tx)
	if err != nil {
		tx.Rollback()
		log.Errorf("failed to find group-role-binding, error: %s", err)
		return fmt.Errorf("failed to find group-role-binding, error: %s", err)
	}

	if visible {
		log.Infof("grb id: %d", groupRoleBinding.ID)
		if groupRoleBinding.ID != 0 {
			return nil
		}
		log.Infof("creating group role binding on group: %s, role: %d", group.GroupID, role.ID)
		err = orm.CreateGroupRoleBinding(&models.GroupRoleBinding{
			GroupID: group.GroupID,
			RoleID:  role.ID,
		}, tx)
		if err != nil {
			tx.Rollback()
			log.Errorf("failed to make project public by setting read-only role to all-users for project %s, error: %s", namespace, err)
			return fmt.Errorf("failed to make project public by setting read-only role to all-users for project %s, error: %s", namespace, err)
		}
	} else {
		log.Infof("grb id 2: %d", groupRoleBinding.ID)
		if groupRoleBinding.ID == 0 {
			return nil
		}
		err = orm.DeleteGroupRoleBinding(&models.GroupRoleBinding{
			GroupID: group.GroupID,
			RoleID:  role.ID,
		}, tx)
		if err != nil {
			tx.Rollback()
			log.Errorf("failed to make project private by deleteing read-only role from all-users for project %s, error: %s", namespace, err)
			return fmt.Errorf("failed to make project private by deleting read-only role from all-users for project %s, error: %s", namespace, err)
		}
	}

	return nil
}
