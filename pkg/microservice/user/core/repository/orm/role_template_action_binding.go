/*
Copyright 2024 The KodeRover Authors.

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

func BulkCreateRoleTemplateActionBindings(roleTemplateID uint, actionIDs []uint, db *gorm.DB) error {
	if len(actionIDs) == 0 {
		return nil
	}
	bindings := make([]models.RoleTemplateActionBinding, 0)

	for _, aID := range actionIDs {
		bindings = append(bindings, models.RoleTemplateActionBinding{
			ActionID:       aID,
			RoleTemplateID: roleTemplateID,
		})
	}

	if err := db.Create(&bindings).Error; err != nil {
		return err
	}

	return nil
}

func DeleteRoleTemplateActionBindingByRole(roleTemplateID uint, db *gorm.DB) error {
	var binding models.RoleTemplateActionBinding
	err := db.Where("role_template_id = ?", roleTemplateID).Delete(&binding).Error
	if err != nil {
		return err
	}
	return nil
}
