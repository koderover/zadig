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

func CreateRoleBinding(role *models.NewRoleBinding, db *gorm.DB) error {
	if err := db.Create(&role).Error; err != nil {
		return err
	}
	return nil
}

func BulkCreateRoleBindingForUser(uid string, roleIDs []uint, db *gorm.DB) error {
	if len(roleIDs) == 0 {
		return nil
	}
	rbs := make([]*models.NewRoleBinding, 0)
	for _, roleID := range roleIDs {
		rbs = append(rbs, &models.NewRoleBinding{
			UID:    uid,
			RoleID: roleID,
		})
	}
	if err := db.Create(&rbs).Error; err != nil {
		return err
	}
	return nil
}

func BulkCreateRoleBindingForRole(roleID uint, UIDs []string, db *gorm.DB) error {
	if len(UIDs) == 0 {
		return nil
	}

	rbs := make([]*models.NewRoleBinding, 0)
	for _, uid := range UIDs {
		rbs = append(rbs, &models.NewRoleBinding{
			UID:    uid,
			RoleID: roleID,
		})
	}
	if err := db.Create(&rbs).Error; err != nil {
		return err
	}
	return nil
}

func GetRoleBinding(roleID uint, uid string, db *gorm.DB) (*models.NewRoleBinding, error) {
	resp := new(models.NewRoleBinding)

	err := db.Where("role_id = ? AND uid = ?", roleID, uid).
		First(&resp).
		Error

	if err != nil {
		return nil, err
	}

	return resp, nil
}

func ListRoleBindingByNamespace(namespace string, db *gorm.DB) ([]*models.NewRoleBinding, error) {
	resp := make([]*models.NewRoleBinding, 0)

	err := db.
		Joins("INNER JOIN role ON role_binding.role_id = role.id").
		Where("role.namespace = ?", namespace).
		Find(&resp).
		Error

	if err != nil {
		return nil, err
	}

	return resp, nil
}

// DeleteRoleBindingByUID deletes all user-role bindings with a certain user under a specific namespace
func DeleteRoleBindingByUID(uid, namespace string, db *gorm.DB) error {
	resp := make([]*models.NewRoleBinding, 0)
	err := db.
		Joins("INNER JOIN role ON role.id = role_binding.role_id").
		Where("role.namespace = ?", namespace).
		Where("role_binding.uid = ?", uid).
		Find(&resp).
		Error

	if err != nil {
		return err
	}

	err = db.Delete(resp).Error

	if err != nil {
		return err
	}

	return nil
}
