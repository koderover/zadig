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
	"time"

	"gorm.io/gorm"

	"github.com/koderover/zadig/v2/pkg/microservice/user/core/repository/models"
	"github.com/koderover/zadig/v2/pkg/types"
)

func CreateUserGroup(userGroup *models.UserGroup, db *gorm.DB) error {
	userGroup.CreatedAt = time.Now().Unix()
	userGroup.UpdatedAt = time.Now().Unix()

	if err := db.Create(&userGroup).Error; err != nil {
		return err
	}
	return nil
}

func ListUserGroups(queryName string, pageNum, pageSize int, db *gorm.DB) ([]*models.UserGroup, int64, error) {
	resp := make([]*models.UserGroup, 0)
	var count int64

	query := db
	if len(queryName) != 0 {
		query = query.Where("group_name LIKE ?", "%"+queryName+"%")
	}

	err := query.Order("updated_at Desc").Offset((pageNum - 1) * pageSize).Limit(pageSize).Find(&resp).Error

	if err != nil {
		return nil, 0, err
	}

	err = query.Table(models.UserGroup{}.TableName()).Count(&count).Error
	if err != nil {
		return nil, 0, err
	}

	return resp, count, nil
}

func GetUserGroup(groupID string, db *gorm.DB) (*models.UserGroup, error) {
	resp := new(models.UserGroup)

	err := db.Where("group_id = ?", groupID).Find(&resp).Error
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func GetUserGroupByName(name string, db *gorm.DB) (*models.UserGroup, error) {
	resp := new(models.UserGroup)

	err := db.Where("group_name = ?", name).Find(&resp).Error
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func UpdateUserGroup(groupID, name, description string, db *gorm.DB) error {
	usergroup := map[string]interface{}{
		"group_name":  name,
		"description": description,
		"updated_at":  time.Now().Unix(),
	}

	if err := db.Model(&models.UserGroup{}).Where("group_id = ?", groupID).Updates(usergroup).Error; err != nil {
		return err
	}
	return nil
}

func DeleteUserGroup(groupID string, db *gorm.DB) error {
	var userGroup models.UserGroup
	err := db.Where("group_id = ?", groupID).Delete(&userGroup).Error
	if err != nil {
		return err
	}
	return nil
}

func ListUserGroupByUID(uid string, db *gorm.DB) ([]*models.UserGroup, error) {
	resp := make([]*models.UserGroup, 0)

	err := db.Joins("INNER JOIN group_binding ON group_binding.group_id = user_group.group_id").
		Where("group_binding.uid = ?", uid).
		Find(&resp).
		Error

	if err != nil {
		return nil, err
	}

	return resp, nil
}

func GetAllUserGroup(db *gorm.DB) (*models.UserGroup, error) {
	resp := new(models.UserGroup)

	err := db.Where("group_name = ?", types.AllUserGroupName).
		Find(&resp).
		Error

	if err != nil {
		return nil, err
	}

	return resp, nil
}
