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

	"github.com/google/uuid"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"go.uber.org/zap"

	"github.com/koderover/zadig/v2/pkg/config"
	userconfig "github.com/koderover/zadig/v2/pkg/microservice/user/config"
	"github.com/koderover/zadig/v2/pkg/microservice/user/core/repository"
	"github.com/koderover/zadig/v2/pkg/microservice/user/core/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/user/core/repository/orm"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/cache"
	"github.com/koderover/zadig/v2/pkg/types"
)

func CreateUserGroup(groupName, desc string, uids []string, logger *zap.SugaredLogger) (*models.UserGroup, error) {
	tx := repository.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	gid, _ := uuid.NewUUID()

	userGroup := &models.UserGroup{
		GroupID:     gid.String(),
		GroupName:   groupName,
		Description: desc,
		Type:        int64(setting.RoleTypeCustom),
	}

	err := orm.CreateUserGroup(userGroup, tx)
	if err != nil {
		tx.Rollback()
		logger.Errorf("failed to create usergroup: %s, error: %s", groupName, err)
		return nil, err
	}

	err = orm.BulkCreateGroupBindings(gid.String(), uids, tx)
	if err != nil {
		tx.Rollback()
		logger.Errorf("failed to create group binding for user group: %s, error: %s", gid.String(), err)
		return nil, err
	}

	tx.Commit()

	return userGroup, nil
}

type UserGroupResp struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	SystemRoleNames []string `json:"system_role_names"`
	Description     string   `json:"description"`
	Type            string   `json:"type"`
	UserTotal       int64    `json:"user_total"`
}

func ListUserGroupsByUid(uid string, logger *zap.SugaredLogger) ([]*UserGroupResp, int64, error) {
	tx := repository.DB.Begin()
	groups, err := orm.ListUserGroupByUID(uid, tx)
	if err != nil {
		tx.Rollback()
		logger.Errorf("failed to list user groups by uid: %s, error: %s", uid, err)
		return nil, 0, err
	}

	resp := make([]*UserGroupResp, 0)
	for _, group := range groups {
		respItem := &UserGroupResp{
			ID:          group.GroupID,
			Name:        group.GroupName,
			Description: group.Description,
		}

		roleList, err := orm.ListSystemRoleByGroupID(group.GroupID, tx)
		if err != nil {
			tx.Rollback()
			logger.Errorf("failed to list role for user group: %s, error: %s", group.GroupName, err)
			return nil, 0, err
		}

		for _, role := range roleList {
			respItem.SystemRoleNames = append(respItem.SystemRoleNames, role.Name)
		}

		if group.Type == int64(setting.RoleTypeSystem) {
			respItem.Type = string(setting.ResourceTypeSystem)
		} else {
			respItem.Type = string(setting.ResourceTypeCustom)
		}
		resp = append(resp, respItem)
	}

	tx.Commit()
	return resp, int64(len(resp)), nil
}

func ListUserGroups(queryName string, pageNum, pageSize int, logger *zap.SugaredLogger) ([]*UserGroupResp, int64, error) {
	resp := make([]*UserGroupResp, 0)
	tx := repository.DB.Begin()

	groups, count, err := orm.ListUserGroups(queryName, pageNum, pageSize, tx)
	if err != nil {
		tx.Rollback()
		logger.Infof("failed to list user groups, error: %s", err)
		return nil, 0, err
	}

	for _, group := range groups {
		respItem := &UserGroupResp{
			ID:          group.GroupID,
			Name:        group.GroupName,
			Description: group.Description,
		}

		roleList, err := orm.ListSystemRoleByGroupID(group.GroupID, tx)
		if err != nil {
			tx.Rollback()
			logger.Errorf("failed to list role for user group: %s, error: %s", group.GroupName, err)
			return nil, 0, err
		}

		for _, role := range roleList {
			respItem.SystemRoleNames = append(respItem.SystemRoleNames, role.Name)
		}

		if group.Type == int64(setting.RoleTypeSystem) {
			respItem.Type = string(setting.ResourceTypeSystem)
		} else {
			respItem.Type = string(setting.ResourceTypeCustom)
		}

		// special case for all-users
		if group.GroupName == types.AllUserGroupName {
			userCount, err := orm.CountUser(tx)
			if err != nil {
				tx.Rollback()
				logger.Errorf("failed to count user for special user group: %s, error: %s", types.AllUserGroupName, err)
				return nil, 0, err
			}
			respItem.UserTotal = userCount
		} else {
			userCount, err := orm.CountUserByGroup(group.GroupID, tx)
			if err != nil {
				tx.Rollback()
				logger.Errorf("failed to count user for user group: %s, error: %s", group.GroupName, err)
				return nil, 0, err
			}
			respItem.UserTotal = userCount
		}
		resp = append(resp, respItem)
	}

	tx.Commit()
	return resp, count, nil
}

func GetUserGroup(groupID string, logger *zap.SugaredLogger) (*types.DetailedUserGroupResp, error) {
	group, err := orm.GetUserGroup(groupID, repository.DB)

	if err != nil {
		logger.Errorf("failed to get user group info, error: %s", err)
		return nil, err
	}

	resp := &types.DetailedUserGroupResp{
		ID:          group.GroupID,
		Name:        group.GroupName,
		Description: group.Description,
	}

	if group.Type == int64(setting.RoleTypeSystem) {
		resp.Type = string(setting.ResourceTypeSystem)
	} else {
		resp.Type = string(setting.ResourceTypeCustom)
	}

	userList := make([]string, 0)
	if group.GroupName == types.AllUserGroupName {
		users, err := orm.ListAllUsers(repository.DB)
		if err != nil {
			logger.Errorf("failed to get user info for user group, error: %s", err)
			return nil, err
		}

		for _, user := range users {
			userList = append(userList, user.UID)
		}
	} else {
		users, err := orm.ListUsersByGroup(groupID, repository.DB)
		if err != nil {
			logger.Errorf("failed to get user info for user group, error: %s", err)
			return nil, err
		}

		for _, user := range users {
			userList = append(userList, user.UID)
		}
	}

	resp.UIDs = userList

	return resp, nil
}

func UpdateUserGroupInfo(groupID, name, description string, logger *zap.SugaredLogger) error {
	return orm.UpdateUserGroup(groupID, name, description, repository.DB)
}

func DeleteUserGroup(groupID string, logger *zap.SugaredLogger) error {
	// if no role is assigned to a user group, do a deletion
	return orm.DeleteUserGroup(groupID, repository.DB)
}

func BulkAddUserToUserGroup(groupID string, uids []string, logger *zap.SugaredLogger) error {
	err := orm.BulkCreateGroupBindings(groupID, uids, repository.DB)
	if err != nil {
		return err
	}
	userCache := cache.NewRedisCache(config.RedisCommonCacheTokenDB())

	for _, uid := range uids {
		userGroupKey := fmt.Sprintf(userconfig.UserGroupCacheKeyFormat, uid)
		err := userCache.Delete(userGroupKey)
		if err != nil {
			log.Warnf("failed to flush uid: %s's group id cache, error: %s", uid, err)
		}

		go func(userGroupKey string, redisCache *cache.RedisCache) {
			time.Sleep(2 * time.Second)
			redisCache.Delete(userGroupKey)
		}(userGroupKey, userCache)
	}

	return nil
}

func BulkRemoveUserFromUserGroup(groupID string, uids []string, logger *zap.SugaredLogger) error {
	err := orm.BulkDeleteGroupBindings(groupID, uids, repository.DB)
	if err != nil {
		return err
	}
	userCache := cache.NewRedisCache(config.RedisCommonCacheTokenDB())

	for _, uid := range uids {
		userGroupKey := fmt.Sprintf(userconfig.UserGroupCacheKeyFormat, uid)
		err := userCache.Delete(userGroupKey)
		if err != nil {
			log.Warnf("failed to flush uid: %s 's group id cache, error: %s", uid, err)
		}

		go func(userGroupKey string, redisCache *cache.RedisCache) {
			time.Sleep(2 * time.Second)
			redisCache.Delete(userGroupKey)
		}(userGroupKey, userCache)
	}

	return nil
}
