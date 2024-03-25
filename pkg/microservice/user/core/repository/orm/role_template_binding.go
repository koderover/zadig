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

func ListRoleTemplateBindingByID(id uint, db *gorm.DB) ([]*models.RoleTemplateBinding, error) {
	resp := make([]*models.RoleTemplateBinding, 0)

	err := db.
		Where("role_template.id = ?", id).
		Find(&resp).
		Error

	if err != nil {
		return nil, err
	}

	return resp, nil
}

func BulkCreateRoleTemplateBindings(bindings []*models.RoleTemplateBinding, db *gorm.DB) error {
	return db.Create(&bindings).Error
}
