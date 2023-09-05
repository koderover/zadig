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
}

func ListUserGroups(pageNum, pageSize int, logger *zap.SugaredLogger) ([]*UserGroupResp, error) {
	resp := make([]*UserGroupResp, 0)

	groups, err := orm.ListUserGroups(pageNum, pageSize, repository.DB)
	if err != nil {
		logger.Infof("failed to list user groups, error: %s", err)
		return nil, err
	}

	for _, group := range groups {
		resp = append(resp, &UserGroupResp{
			ID:          group.GroupID,
			Name:        group.GroupName,
			Description: group.Description,
		})
	}

	return resp, nil
}

func GetUserGroup(groupID string, logger *zap.SugaredLogger) (*models.UserGroup, error) {
	resp, err := orm.GetUserGroup(groupID, repository.DB)

	if err != nil {
		logger.Errorf("failed to get user group info, error: %s", err)
		return nil, err
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
