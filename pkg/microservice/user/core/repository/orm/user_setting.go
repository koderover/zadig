/*
Copyright 2022 The KodeRover Authors.

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
	"github.com/koderover/zadig/pkg/microservice/user/core/repository/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// GetUserSettingByUid Gets user setting info from db by uid
func GetUserSettingByUid(uid string, db *gorm.DB) (*models.UserSetting, error) {
	user := &models.UserSetting{}
	err := db.Where("uid = ?", uid).First(user).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return user, nil
}

// UpsertUserSetting update user setting info
func UpsertUserSetting(userSetting *models.UserSetting, db *gorm.DB) error {
	return db.Model(&models.UserSetting{}).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "uid"}},
		DoUpdates: clause.AssignmentColumns([]string{"theme", "log_bg_color", "log_font_color"}),
	}).Create(userSetting).Error
}

// DeleteUserSettingByUid Delete  user setting by on uid
func DeleteUserSettingByUid(uid string, db *gorm.DB) error {
	var userSetting models.UserSetting
	return db.Where("uid = ?", uid).Delete(&userSetting).Error
}
