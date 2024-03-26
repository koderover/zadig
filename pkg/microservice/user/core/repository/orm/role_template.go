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
	"github.com/koderover/zadig/v2/pkg/microservice/user/core/repository/models"
	"gorm.io/gorm"
)

func CreateRoleTemplate(roleTemplate *models.RoleTemplate, db *gorm.DB) error {
	if err := db.Create(&roleTemplate).Error; err != nil {
		return err
	}
	return nil
}

func BulkCreateRoleTemplate(roleTemplates []*models.RoleTemplate, db *gorm.DB) error {
	if err := db.Create(&roleTemplates).Error; err != nil {
		return err
	}
	return nil
}

func GetRoleTemplate(name string, db *gorm.DB) (*models.RoleTemplate, error) {
	resp := new(models.RoleTemplate)
	err := db.Where("name = ?", name).Find(&resp).Error

	if err != nil {
		return nil, err
	}
	return resp, nil
}

func ListRoleTemplates(db *gorm.DB) ([]*models.RoleTemplate, error) {
	resp := make([]*models.RoleTemplate, 0)

	err := db.Find(&resp).Error

	if err != nil {
		return nil, err
	}

	return resp, nil
}

func GetRoleTemplateByID(id uint, db *gorm.DB) (*models.RoleTemplate, error) {
	resp := new(models.RoleTemplate)
	err := db.Where("id = ?", id).Find(&resp).Error
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func UpdateRoleTemplateInfo(id uint, role *models.RoleTemplate, db *gorm.DB) error {
	err := db.Model(&models.RoleTemplate{}).Where("id = ?", id).Updates(role).Error
	if err != nil {
		return err
	}
	return nil
}

func DeleteRoleTemplateByName(name string, db *gorm.DB) error {
	var role models.RoleTemplate
	return db.Model(&models.RoleTemplate{}).
		Where("name = ?", name).
		Delete(&role).
		Error
}
