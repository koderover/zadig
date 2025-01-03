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
	"github.com/koderover/zadig/v2/pkg/microservice/user/core/repository"
	"gorm.io/gorm"

	"github.com/koderover/zadig/v2/pkg/microservice/user/core/repository/models"
	"github.com/koderover/zadig/v2/pkg/types"
)

// CreateUser create a user
func CreateUser(user *models.User, db *gorm.DB) error {
	if err := db.Create(&user).Error; err != nil {
		return err
	}
	return nil
}

// GetUser Get a user based on email and identityType
func GetUser(account string, identityType string, db *gorm.DB) (*models.User, error) {
	var user models.User
	err := db.Where("account = ? and identity_type = ?", account, identityType).First(&user).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &user, nil
}

// GetUserByUid Get a user based on uid
func GetUserByUid(uid string, db *gorm.DB) (*models.User, error) {
	var user models.User
	err := db.Where("uid = ?", uid).First(&user).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &user, nil
}

// GetUserByName Get a user based on name
func GetUserByName(name string, db *gorm.DB) (*models.User, error) {
	var user models.User
	err := db.Where("name = ?", name).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func ListAllUsers(db *gorm.DB) ([]*models.User, error) {
	resp := make([]*models.User, 0)

	err := db.Find(&resp).Error
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// ListUsers gets a list of users based on paging constraints
func ListUsers(page int, perPage int, name string, db *gorm.DB) ([]models.User, error) {
	var (
		users []models.User
		err   error
	)

	err = db.Where("name LIKE ?", "%"+name+"%").Order("account ASC").Offset((page - 1) * perPage).Limit(perPage).Find(&users).Error

	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}

	return users, nil
}

// ListUsersByNameAndRole gets a list of users based on paging constraints, the name of the user, and the roles of the user
func ListUsersByNameAndRole(page int, perPage int, name string, roles []string, db *gorm.DB) ([]models.User, error) {
	var (
		users []models.User
		err   error
	)

	err = db.Where("user.name LIKE ? AND role.name IN ?", "%"+name+"%", roles).
		Joins("INNER JOIN role_binding on role_binding.uid = user.uid").
		Joins("INNER JOIN role on role_binding.role_id = role.id").Order("account ASC").Offset((page - 1) * perPage).
		Group("user.uid").
		Limit(perPage).
		Find(&users).
		Error

	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}

	return users, nil
}

func ListUsersByGroup(groupID string, db *gorm.DB) ([]*models.User, error) {
	resp := make([]*models.User, 0)

	err := db.Joins("INNER JOIN group_binding on group_binding.uid = user.uid").
		Where("group_binding.group_id = ?", groupID).
		Find(&resp).
		Error

	if err != nil {
		return nil, err
	}

	return resp, nil
}

// ListUsersByUIDs gets a list of users based on paging constraints
func ListUsersByUIDs(uids []string, db *gorm.DB) ([]models.User, error) {
	var (
		users []models.User
		err   error
	)

	err = db.Find(&users, "uid in ?", uids).Error

	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}

	return users, nil
}

// ListUsersByIdentityType gets a list of users based on identityType
func ListUsersByIdentityType(identityType string, db *gorm.DB) ([]models.User, error) {
	var (
		users []models.User
		err   error
	)

	err = db.Find(&users, "identity_type = ?", identityType).Error

	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}

	return users, nil
}

// DeleteUserByUids Delete  users based on uids
func DeleteUserByUids(uids []string, db *gorm.DB) error {
	var user models.User
	err := db.Where("uid in ?", uids).Delete(&user).Error
	if err != nil {
		return err
	}
	return nil
}

// DeleteUserByUid Delete  users based on uids
func DeleteUserByUid(uid string, db *gorm.DB) error {
	var user models.User
	err := db.Where("uid = ?", uid).Delete(&user).Error
	if err != nil {
		return err
	}
	return nil
}

// GetUsersCount gets user count
func GetUsersCount(name string) (int64, error) {
	var (
		users []models.User
		err   error
		count int64
	)

	err = repository.DB.Where("name LIKE ?", "%"+name+"%").Find(&users).Count(&count).Error

	if err != nil {
		return 0, err
	}

	return count, nil
}

// GetUsersCount gets user count
func GetUsersCountByRoles(name string, roles []string) (int64, error) {
	var (
		users []models.User
		err   error
		count int64
	)

	err = repository.DB.Where("user.name LIKE ? AND role.name IN ?", "%"+name+"%", roles).
		Joins("INNER JOIN role_binding on role_binding.uid = user.uid").
		Joins("INNER JOIN role on role_binding.role_id = role.id").
		Group("user.uid").
		Find(&users).
		Count(&count).
		Error

	if err != nil {
		return 0, err
	}

	return count, nil
}

// UpdateUser update user info
func UpdateUser(uid string, user *models.User, db *gorm.DB) error {
	if err := db.Model(&models.User{}).Where("uid = ?", uid).Updates(user).Error; err != nil {
		return err
	}
	return nil
}

func CountUserByType(db *gorm.DB) ([]*types.UserCountByType, error) {
	var resp []*types.UserCountByType
	err := db.Model(&models.User{}).Select("count(*) as count, identity_type").Group("identity_type").Find(&resp).Error
	if err != nil {
		return nil, err
	}
	return resp, nil
}
