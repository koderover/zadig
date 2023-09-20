/*
Copyright 2021 The KodeRover Authors.

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

func BulkCreateRoleActionBindings(roleID uint, actionIDs []uint, db *gorm.DB) error {
	if len(actionIDs) == 0 {
		return nil
	}
	bindings := make([]models.RoleActionBinding, 0)

	for _, aID := range actionIDs {
		bindings = append(bindings, models.RoleActionBinding{
			ActionID: aID,
			RoleID:   roleID,
		})
	}

	if err := db.Create(&bindings).Error; err != nil {
		return err
	}

	return nil
}

func DeleteRoleActionBindingByRole(roleID uint, db *gorm.DB) error {
	var binding models.RoleActionBinding
	err := db.Where("role_id = ?", roleID).Delete(&binding).Error
	if err != nil {
		return err
	}
	return nil
}
