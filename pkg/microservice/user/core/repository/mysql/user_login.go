package mysql

import (
	"github.com/jinzhu/gorm"
	"github.com/koderover/zadig/pkg/microservice/user/core/repository/models"
)

// CreateUserLogin add a userLogin record
func CreateUserLogin(userLogin *models.UserLogin, db *gorm.DB) error {
	if err := db.Create(&userLogin).Error; err != nil {
		return err
	}
	return nil
}

// GetUserLogin Get a userLogin based on uid
func GetUserLogin(uid string, db *gorm.DB) (*models.UserLogin, error) {
	var userLogin models.UserLogin
	err := db.Where("uid = ?", uid).First(&userLogin).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &userLogin, nil
}

// GetUserLogins Get a userLogin based on uid list
func GetUserLogins(uids []string, db *gorm.DB) (*[]models.UserLogin, error) {
	var userLogins []models.UserLogin
	err := db.Where("uid IN (?)", uids).Find(&userLogins).Error
	if err != nil {
		return nil, err
	}
	return &userLogins, nil
}

// UpdateUserLogin update login info
func UpdateUserLogin(uid string, userLogin *models.UserLogin, db *gorm.DB) error {
	if err := db.Model(&models.UserLogin{}).Where("uid = ?", uid).Updates(userLogin).Error; err != nil {
		return err
	}
	return nil
}
