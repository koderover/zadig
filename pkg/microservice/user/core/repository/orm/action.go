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

func CreateAction(action *models.Action, db *gorm.DB) error {
	if err := db.Create(&action).Error; err != nil {
		return err
	}
	return nil
}

func ListActionByType(actionScope int, db *gorm.DB) ([]*models.Action, error) {
	resp := make([]*models.Action, 0)

	err := db.Where("scope = ?", actionScope).Find(&resp).Error

	if err != nil {
		return nil, err
	}

	return resp, nil
}

func ListActionByRole(roleID uint, db *gorm.DB) ([]*models.Action, error) {
	resp := make([]*models.Action, 0)
	err := db.Where("role_action_binding.role_id = ?", roleID).
		Joins("INNER JOIN action ON role_action_binding.action_id = action.id").
		Find(&resp).
		Error

	if err != nil {
		return nil, err
	}

	return resp, nil
}

func GetActionByVerb(verb string, db *gorm.DB) (*models.Action, error) {
	resp := new(models.Action)

	err := db.Where("action = ?", verb).Find(&resp).Error

	if err != nil {
		return nil, err
	}

	return resp, nil
}
