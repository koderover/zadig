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

func CreateGroupBinding(groupID, uid string, db *gorm.DB) error {
	insertData := &models.GroupBinding{
		GroupID: groupID,
		UID:     uid,
	}
	if err := db.Create(&insertData).Error; err != nil {
		return err
	}

	return nil
}

func BulkCreateGroupBindings(groupID string, uids []string, db *gorm.DB) error {
	bindings := make([]models.GroupBinding, 0)

	for _, uid := range uids {
		bindings = append(bindings, models.GroupBinding{
			GroupID: groupID,
			UID:     uid,
		})
	}

	if err := db.Create(&bindings).Error; err != nil {
		return err
	}

	return nil
}

func BulkDeleteGroupBindings(groupID string, uids []string, db *gorm.DB) error {
	err := db.Where("group_id = ? AND uid IN ?", groupID, uids).Delete(&models.GroupBinding{}).Error
	if err != nil {
		return err
	}
	return nil
}
