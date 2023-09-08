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
	"github.com/google/uuid"
	"github.com/koderover/zadig/pkg/microservice/user/core/repository"
	"github.com/koderover/zadig/pkg/microservice/user/core/repository/models"
	"github.com/koderover/zadig/pkg/microservice/user/core/repository/orm"
	"github.com/koderover/zadig/pkg/setting"
	"go.uber.org/zap"
)

func CreateUserGroup(groupName, desc string, uids []string, logger *zap.SugaredLogger) error {
	tx := repository.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	gid, _ := uuid.NewUUID()

	err := orm.CreateUserGroup(&models.UserGroup{
		GroupID:     gid.String(),
		GroupName:   groupName,
		Description: desc,
		Type:        int64(setting.RoleTypeCustom),
	}, tx)
	if err != nil {
		tx.Rollback()
		logger.Errorf("failed to create usergroup: %s, error: %s", groupName, err)
		return err
	}

	err = orm.BulkCreateGroupBindings(gid.String(), uids, tx)
	if err != nil {
		tx.Rollback()
		logger.Errorf("failed to create group binding for user group: %s, error: %s", gid.String(), err)
		return err
	}

	tx.Commit()

	return nil
}

type UserGroupResp struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Type        string `json:"type"`
	UserTotal   int64  `json:"user_total"`
}

func ListUserGroups(pageNum, pageSize int, logger *zap.SugaredLogger) ([]*UserGroupResp, int64, error) {
	resp := make([]*UserGroupResp, 0)
	tx := repository.DB.Begin()

	groups, count, err := orm.ListUserGroups(pageNum, pageSize, tx)
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
		if group.Type == int64(setting.RoleTypeSystem) {
			respItem.Type = string(setting.ResourceTypeSystem)
		} else {
			respItem.Type = string(setting.ResourceTypeCustom)
		}

		// special case for all-users
		if group.GroupName == "所有用户" {
			userCount, err := orm.CountUser(tx)
			if err != nil {
				tx.Rollback()
				logger.Errorf("failed to count user for special user group: 所有用户, error: %s", err)
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

type DetailedUserGroupResp struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Type        string   `json:"type"`
	UIDs        []string `json:"uids"`
}

func GetUserGroup(groupID string, logger *zap.SugaredLogger) (*DetailedUserGroupResp, error) {
	group, err := orm.GetUserGroup(groupID, repository.DB)

	if err != nil {
		logger.Errorf("failed to get user group info, error: %s", err)
		return nil, err
	}

	resp := &DetailedUserGroupResp{
		ID:          group.GroupID,
		Name:        group.GroupName,
		Description: group.Description,
	}

	if group.Type == int64(setting.RoleTypeSystem) {
		resp.Type = string(setting.ResourceTypeSystem)
	} else {
		resp.Type = string(setting.ResourceTypeCustom)
	}

	users, err := orm.ListUsersByGroup(groupID, repository.DB)
	if err != nil {
		return nil, err
	}
	userList := make([]string, 0)

	for _, user := range users {
		userList = append(userList, user.UID)
	}

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
	return orm.BulkCreateGroupBindings(groupID, uids, repository.DB)
}

func BulkRemoveUserFromUserGroup(groupID string, uids []string, logger *zap.SugaredLogger) error {
	return orm.BulkDeleteGroupBindings(groupID, uids, repository.DB)
}
