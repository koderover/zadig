package login

import (
	"github.com/google/uuid"
	"github.com/koderover/zadig/pkg/microservice/user/core"
	"github.com/koderover/zadig/pkg/microservice/user/core/repository/models"
	"github.com/koderover/zadig/pkg/microservice/user/core/repository/mysql"
	"go.uber.org/zap"
	"time"
)

type SyncUserInfo struct {
	Email        string `json:"email"`
	IdentityType string `json:"identityType"`
	Name         string `json:"name"`
}

func SyncUser(syncUserInfo *SyncUserInfo, logger *zap.SugaredLogger) (*models.User, error) {
	user, err := mysql.GetUser(syncUserInfo.Email, syncUserInfo.IdentityType, core.DB)
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
			Uid:          uid.String(),
			Name:         syncUserInfo.Name,
			Email:        syncUserInfo.Email,
			IdentityType: syncUserInfo.IdentityType,
		}
		err = mysql.CreateUser(user, tx)
		if err != nil {
			tx.Rollback()
			logger.Error("SyncUser create user:%s error, error msg:%s", syncUserInfo.Email, err.Error())
			return nil, err
		}
	}
	userLogin, err := mysql.GetUserLogin(user.Uid, tx)
	if err != nil {
		tx.Rollback()
		logger.Error("UpdateLoginInfo get user:%s login error, error msg:%s", user.Uid, err.Error())
		return nil, err
	}
	if userLogin != nil {
		userLogin.LastLoginTime = time.Now().Unix()
		err = mysql.UpdateUserLogin(user.Uid, userLogin, tx)
		if err != nil {
			tx.Rollback()
			logger.Error("UpdateLoginInfo update user:%s login error, error msg:%s", user.Uid, err.Error())
			return nil, err
		}
	} else {
		err = mysql.CreateUserLogin(&models.UserLogin{
			Uid:           user.Uid,
			LastLoginTime: time.Now().Unix(),
		}, tx)
		if err != nil {
			tx.Rollback()
			logger.Error("UpdateLoginInfo create user:%s login error, error msg:%s", user.Uid, err.Error())
			return nil, err
		}
	}
	err = tx.Commit().Error
	if err != nil {
		return nil, err
	}
	return user, nil
}
