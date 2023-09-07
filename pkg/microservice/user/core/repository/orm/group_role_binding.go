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

package orm

import (
	"gorm.io/gorm"

	"github.com/koderover/zadig/pkg/microservice/user/core/repository/models"
)

func BulkCreateGroupRoleBindings(groupID string, roleIDs []uint, db *gorm.DB) error {
	if len(roleIDs) == 0 {
		return nil
	}
	rbs := make([]*models.GroupRoleBinding, 0)
	for _, roleID := range roleIDs {
		rbs = append(rbs, &models.GroupRoleBinding{
			GroupID: groupID,
			RoleID:  roleID,
		})
	}
	if err := db.Create(&rbs).Error; err != nil {
		return err
	}
	return nil
}

func BulkCreateRoleBindingForGroup(gid string, roleIDs []uint, db *gorm.DB) error {
	if len(roleIDs) == 0 {
		return nil
	}
	rbs := make([]*models.GroupRoleBinding, 0)
	for _, roleID := range roleIDs {
		rbs = append(rbs, &models.GroupRoleBinding{
			GroupID: gid,
			RoleID:  roleID,
		})
	}
	if err := db.Create(&rbs).Error; err != nil {
		return err
	}
	return nil
}

func ListGroupRoleBindingsByGroupsAndRoles(roleID uint, groupIDs []string, db *gorm.DB) ([]*models.GroupRoleBinding, error) {
	resp := make([]*models.GroupRoleBinding, 0)

	err := db.Where("role_id = ? AND group_id IN (?)", roleID, groupIDs).
		Find(&resp).
		Error

	if err != nil {
		return nil, err
	}

	return resp, nil
}

func ListGroupRoleBindingsByNamespace(namespace string, db *gorm.DB) ([]*models.GroupRoleBinding, error) {
	resp := make([]*models.GroupRoleBinding, 0)

	err := db.
		Joins("INNER JOIN role ON group_role_binding.role_id = role.id").
		Where("role.namespace = ?", namespace).
		Find(&resp).
		Error

	if err != nil {
		return nil, err
	}

	return resp, nil
}

func CountUserByGroup(gid string, db *gorm.DB) (int64, error) {
	var count int64
	err := db.
		Table(models.GroupBinding{}.TableName()).
		Where("group_id = ?", gid).
		Count(&count).
		Error

	if err != nil {
		return 0, err
	}

	return count, nil
}

func DeleteGroupRoleBindingByGID(gid, namespace string, db *gorm.DB) error {
	resp := make([]*models.GroupRoleBinding, 0)
	err := db.
		Joins("INNER JOIN role ON role.id = group_role_binding.role_id").
		Where("role.namespace = ?", namespace).
		Where("group_role_binding.group_id = ?", gid).
		Find(&resp).
		Error

	if err != nil {
		return err
	}

	if len(resp) == 0 {
		return nil
	}

	err = db.Delete(resp).Error

	if err != nil {
		return err
	}

	return nil
}
