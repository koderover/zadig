package login

import (
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/user/core"
	"github.com/koderover/zadig/pkg/microservice/user/core/repository/models"
	"github.com/koderover/zadig/pkg/microservice/user/core/repository/orm"
)

type SyncUserInfo struct {
	Email        string `json:"email"`
	IdentityType string `json:"identityType"`
	Name         string `json:"name"`
}

func SyncUser(syncUserInfo *SyncUserInfo, logger *zap.SugaredLogger) (*models.User, error) {
	user, err := orm.GetUser(syncUserInfo.Email, syncUserInfo.IdentityType, core.DB)
	if err != nil {
		logger.Error("SyncUser get user:%s error, error msg:%s", syncUserInfo.Email, err.Error())
		return nil, err
	}
	tx := core.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()
	if user == nil {
		uid, _ := uuid.NewUUID()
		user = &models.User{
			UID:          uid.String(),
			Name:         syncUserInfo.Name,
			Email:        syncUserInfo.Email,
			IdentityType: syncUserInfo.IdentityType,
		}
		err = orm.CreateUser(user, tx)
		if err != nil {
			tx.Rollback()
			logger.Error("SyncUser create user:%s error, error msg:%s", syncUserInfo.Email, err.Error())
			return nil, err
		}
	}
	userLogin, err := orm.GetUserLogin(user.UID, tx)
	if err != nil {
		tx.Rollback()
		logger.Error("UpdateLoginInfo get user:%s login error, error msg:%s", user.UID, err.Error())
		return nil, err
	}
	if userLogin != nil {
		userLogin.LastLoginTime = time.Now().Unix()
		err = orm.UpdateUserLogin(user.UID, userLogin, tx)
		if err != nil {
			tx.Rollback()
			logger.Error("UpdateLoginInfo update user:%s login error, error msg:%s", user.UID, err.Error())
			return nil, err
		}
	} else {
		err = orm.CreateUserLogin(&models.UserLogin{
			UID:           user.UID,
			LastLoginTime: time.Now().Unix(),
		}, tx)
		if err != nil {
			tx.Rollback()
			logger.Error("UpdateLoginInfo create user:%s login error, error msg:%s", user.UID, err.Error())
			return nil, err
		}
	}
	err = tx.Commit().Error
	if err != nil {
		return nil, err
	}
	return user, nil
}
