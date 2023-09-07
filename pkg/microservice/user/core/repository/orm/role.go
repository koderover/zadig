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

func CreateRole(role *models.NewRole, db *gorm.DB) error {
	if err := db.Create(&role).Error; err != nil {
		return err
	}
	return nil
}

func BulkCreateRole(roles []*models.NewRole, db *gorm.DB) error {
	if err := db.Create(&roles).Error; err != nil {
		return err
	}
	return nil
}

func GetRole(name, namespace string, db *gorm.DB) (*models.NewRole, error) {
	resp := new(models.NewRole)
	err := db.Where("name = ? AND namespace = ?", name, namespace).Find(&resp).Error

	if err != nil {
		return nil, err
	}
	return resp, nil
}

func UpdateRoleInfo(id uint, role *models.NewRole, db *gorm.DB) error {
	err := db.Model(&models.NewRole{}).Where("id = ?", id).Updates(role).Error

	if err != nil {
		return err
	}

	return nil
}

func ListRoleByNamespace(namespace string, db *gorm.DB) ([]*models.NewRole, error) {
	resp := make([]*models.NewRole, 0)

	err := db.Where("namespace = ?", namespace).Find(&resp).Error

	if err != nil {
		return nil, err
	}

	return resp, nil
}

func ListRoleByRoleNamesAndNamespace(names []string, namespace string, db *gorm.DB) ([]*models.NewRole, error) {
	resp := make([]*models.NewRole, 0)

	err := db.Where("namespace = ? AND name IN (?)", namespace, names).Find(&resp).Error

	if err != nil {
		return nil, err
	}

	return resp, nil
}

// ListRoleByUIDAndNamespace list a set of roles that is used by specific user in a given namespace
func ListRoleByUIDAndNamespace(uid, namespace string, db *gorm.DB) ([]*models.NewRole, error) {
	resp := make([]*models.NewRole, 0)

	err := db.Joins("INNER JOIN role_binding ON role.id = role_binding.role_id").
		Where("role.namespace = ?", namespace).
		Where("role_binding.uid = ?", uid).
		Find(&resp).
		Error

	if err != nil {
		return nil, err
	}

	return resp, nil
}

// ListRoleByUID list a set of roles that is used by specific user in ALL namespace
func ListRoleByUID(uid string, db *gorm.DB) ([]*models.NewRole, error) {
	resp := make([]*models.NewRole, 0)

	err := db.Joins("INNER JOIN role_binding ON role.id = role_binding.role_id").
		Where("role_binding.uid = ?", uid).
		Find(&resp).
		Error

	if err != nil {
		return nil, err
	}

	return resp, nil
}

// ListRoleByUIDAndVerb list all roles that have the specific verb permission, or project-admin
func ListRoleByUIDAndVerb(uid string, verb string, db *gorm.DB) ([]*models.NewRole, error) {
	resp := make([]*models.NewRole, 0)

	err := db.Joins("INNER JOIN role_binding ON role_binding.role_id = role.id").
		Joins("INNER JOIN user ON user.uid = role_binding.uid").
		Joins("INNER JOIN action_binding ON action_binding.role_id = role.id").
		Joins("INNER JOIN action ON action.id = action_binding.action_id").
		Where("user.uid = ? AND (action.action = ? OR role.name = ?)", uid, verb, "project-admin").
		Find(&resp).
		Error

	if err != nil {
		return nil, err
	}

	return resp, nil
}

func ListRoleByGroupIDsAndNamespace(groupIDs []string, namespace string, db *gorm.DB) ([]*models.NewRole, error) {
	resp := make([]*models.NewRole, 0)

	err := db.Joins("INNER JOIN role ON role.id = group_role_binding.role_id").
		Where("role.namespace = ?", namespace).
		Where("group_role_binding.group_id IN (?)", groupIDs).
		Find(&resp).
		Error

	if err != nil {
		return nil, err
	}

	return resp, nil
}

func ListRoleByGroupIDs(groupIDs []string, db *gorm.DB) ([]*models.NewRole, error) {
	resp := make([]*models.NewRole, 0)

	err := db.Joins("INNER JOIN group_role_binding ON role.id = group_role_binding.role_id").
		Where("group_role_binding.group_id IN (?)", groupIDs).
		Find(&resp).
		Error

	if err != nil {
		return nil, err
	}

	return resp, nil
}

func FindSystemAdminRole(db *gorm.DB) (*models.NewRole, error) {
	resp := new(models.NewRole)

	err := db.Where("name = ? AND namespace = ?", "admin", "*").
		First(&resp).
		Error

	if err != nil {
		return nil, err
	}

	return resp, nil
}

func DeleteRoleByName(name, namespace string, db *gorm.DB) error {
	var role models.NewRole

	return db.Model(&models.NewRole{}).
		Where("name = ? AND namespace = ?", name, namespace).
		Delete(&role).
		Error

}
