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
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/koderover/zadig/v2/pkg/microservice/user/core/repository/models"
)

var ErrUserMFAAlreadyEnabled = errors.New("user mfa already enabled")

func GetUserMFA(uid string, db *gorm.DB) (*models.UserMFA, error) {
	res := &models.UserMFA{}
	err := db.Where("uid = ?", uid).First(res).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return res, nil
}

func ListUserMFAsByUIDs(uids []string, db *gorm.DB) ([]*models.UserMFA, error) {
	if len(uids) == 0 {
		return []*models.UserMFA{}, nil
	}
	res := make([]*models.UserMFA, 0)
	if err := db.Where("uid IN ?", uids).Find(&res).Error; err != nil {
		return nil, err
	}
	return res, nil
}

// EnableUserMFA enables MFA for a user without allowing overwrite of an already-enabled MFA config.
func EnableUserMFA(uid, secretCipher, recoveryCodesJSON string, db *gorm.DB) error {
	now := time.Now().Unix()

	tx := db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	existing, err := GetUserMFA(uid, tx)
	if err != nil {
		tx.Rollback()
		return err
	}

	if existing != nil {
		if existing.Enabled {
			tx.Rollback()
			return ErrUserMFAAlreadyEnabled
		}

		if err := tx.Model(&models.UserMFA{}).
			Where("uid = ?", uid).
			Updates(map[string]interface{}{
				"enabled":             true,
				"secret_cipher":       secretCipher,
				"recovery_codes_json": recoveryCodesJSON,
				"updated_at":          now,
			}).Error; err != nil {
			tx.Rollback()
			return err
		}
		return tx.Commit().Error
	}

	record := &models.UserMFA{
		Model: models.Model{
			CreatedAt: now,
			UpdatedAt: now,
		},
		UID:               uid,
		Enabled:           true,
		SecretCipher:      secretCipher,
		RecoveryCodesJSON: recoveryCodesJSON,
	}
	if err := tx.Create(record).Error; err != nil {
		tx.Rollback()
		// Handle race: another concurrent request may have created/enabled MFA first.
		current, getErr := GetUserMFA(uid, db)
		if getErr == nil && current != nil && current.Enabled {
			return ErrUserMFAAlreadyEnabled
		}
		if getErr != nil {
			return getErr
		}
		return err
	}

	return tx.Commit().Error
}

func UpsertUserMFA(uid string, userMFA *models.UserMFA, db *gorm.DB) error {
	userMFA.UID = uid
	userMFA.UpdatedAt = time.Now().Unix()
	if userMFA.CreatedAt == 0 {
		userMFA.CreatedAt = userMFA.UpdatedAt
	}
	if err := db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "uid"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"enabled":             userMFA.Enabled,
			"secret_cipher":       userMFA.SecretCipher,
			"recovery_codes_json": userMFA.RecoveryCodesJSON,
			"updated_at":          userMFA.UpdatedAt,
		}),
	}).Create(userMFA).Error; err != nil {
		return err
	}
	return nil
}

func DeleteUserMFA(uid string, db *gorm.DB) error {
	return db.Where("uid = ?", uid).Delete(&models.UserMFA{}).Error
}
