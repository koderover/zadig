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

package login

import (
	"errors"
	"fmt"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/golang-jwt/jwt"
	"github.com/google/uuid"
	"github.com/mojocn/base64Captcha"
	"github.com/patrickmn/go-cache"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"

	configbase "github.com/koderover/zadig/v2/pkg/config"
	"github.com/koderover/zadig/v2/pkg/microservice/user/config"
	"github.com/koderover/zadig/v2/pkg/microservice/user/core/repository"
	"github.com/koderover/zadig/v2/pkg/microservice/user/core/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/user/core/repository/orm"
	"github.com/koderover/zadig/v2/pkg/microservice/user/core/service/common"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/shared/client/aslan"
	"github.com/koderover/zadig/v2/pkg/shared/client/plutusvendor"
	zadigCache "github.com/koderover/zadig/v2/pkg/tool/cache"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

type LoginArgs struct {
	Account       string `json:"account"`
	Password      string `json:"password"`
	CaptchaID     string `json:"captcha_id"`
	CaptchaAnswer string `json:"captcha_answer"`
}

type User struct {
	Uid          string   `json:"uid"`
	Token        string   `json:"token"`
	Email        string   `json:"email"`
	Phone        string   `json:"phone"`
	Name         string   `json:"name"`
	Account      string   `json:"account"`
	GroupIDs     []string `json:"group_ids"`
	IdentityType string   `json:"identityType"`
}

type CheckSignatureRes struct {
	Code        int                    `json:"code"`
	Description string                 `json:"description"`
	Extra       map[string]interface{} `json:"extra"`
	Message     string                 `json:"message"`
	Type        string                 `json:"type"`
}

func CheckSignature(lastLoginTime int64, logger *zap.SugaredLogger) error {
	vendorClient := plutusvendor.New()
	err := vendorClient.Health()
	if err != nil {
		return err
	}

	status, checkErr := vendorClient.CheckZadigXLicenseStatus()
	if checkErr != nil {
		return checkErr
	}

	userNum, err := orm.CountActiveUser(status.UpdatedAt, repository.DB)
	if err != nil {
		return err
	}

	res, checkErr := vendorClient.CheckSignature(userNum)
	if checkErr != nil {
		return checkErr
	}

	if res.Code == 6694 {
		if lastLoginTime > status.UpdatedAt {
			return nil
		} else {
			return fmt.Errorf("系统使用用户数量已达授权人数上限，请联系系统管理员")
		}
	}

	return nil
}

var (
	loginCache = cache.New(time.Hour, time.Second*10)
)

func LocalLogin(args *LoginArgs, logger *zap.SugaredLogger) (*User, int, error) {
	user, err := orm.GetUser(args.Account, config.SystemIdentityType, repository.DB)
	if err != nil {
		logger.Errorf("InternalLogin get user account:%s error", args.Account)
		return nil, 0, err
	}
	if user == nil {
		return nil, 0, fmt.Errorf("user not exist")
	}
	userLogin, err := orm.GetUserLogin(user.UID, args.Account, config.AccountLoginType, repository.DB)
	if err != nil {
		logger.Errorf("LocalLogin get user:%s user login not exist, error msg:%s", args.Account, err.Error())
		return nil, 0, err
	}
	if userLogin == nil {
		logger.Errorf("InternalLogin user:%s user login not exist", args.Account)
		return nil, 0, fmt.Errorf("user login not exist")
	}

	failedCountInterface, failedCountfound := loginCache.Get(user.UID)

	if failedCountfound {
		if failedCountInterface.(int) >= 5 {
			// first check if a captcha answer is provided
			if args.CaptchaAnswer == "" || args.CaptchaID == "" {
				return nil, 5, fmt.Errorf("captcha is required")
			}

			// captcha validation
			if passed := store.Verify(args.CaptchaID, args.CaptchaAnswer, false); !passed {
				return nil, 5, fmt.Errorf("captcha is wrong")
			}
		}
	}

	password := []byte(args.Password)
	err = bcrypt.CompareHashAndPassword([]byte(userLogin.Password), password)
	if err == bcrypt.ErrMismatchedHashAndPassword {

		if !failedCountfound {
			loginCache.Set(user.UID, 1, time.Hour)
		} else {
			err := loginCache.Increment(user.UID, 1)
			if err != nil {
				logger.Errorf("failed to do login cache increment for UID: [%s], error: %s", user.UID, err)
			}
		}
		failedCount, ok := failedCountInterface.(int)
		if !ok {
			failedCount = 0
		}
		return nil, failedCount + 1, fmt.Errorf("password is wrong")
	}
	if err != nil {
		logger.Errorf("LocalLogin user:%s check password error, error msg:%s", args.Account, err)
		return nil, 0, fmt.Errorf("check password error, error msg:%s", err)
	}

	err = CheckSignature(userLogin.LastLoginTime, logger)
	if err != nil {
		return nil, 0, err
	}

	userLogin.LastLoginTime = time.Now().Unix()
	err = orm.UpdateUserLogin(userLogin.UID, userLogin, repository.DB)
	if err != nil {
		logger.Errorf("LocalLogin user:%s update user login password error, error msg:%s", args.Account, err.Error())
		return nil, 0, err
	}

	systemSettings, err := aslan.New(configbase.AslanServiceAddress()).GetSystemSecurityAndPrivacySettings()
	if err != nil {
		logger.Errorf("failed to get system security settings, error: %s", err)
		return nil, 0, fmt.Errorf("failed to get system security settings, error: %s", err)
	}

	token, err := CreateToken(&Claims{
		Name:              user.Name,
		UID:               user.UID,
		Email:             user.Email,
		PreferredUsername: user.Account,
		StandardClaims: jwt.StandardClaims{
			Audience:  setting.ProductName,
			ExpiresAt: time.Now().Add(time.Duration(systemSettings.TokenExpirationTime) * time.Hour).Unix(),
		},
		FederatedClaims: FederatedClaims{
			ConnectorId: user.IdentityType,
			UserId:      user.Account,
		},
	})
	if err != nil {
		logger.Errorf("LocalLogin user:%s create token error, error msg:%s", args.Account, err.Error())
		return nil, 0, err
	}

	groupIDList, err := common.GetUserGroupByUID(user.UID)
	if err != nil {
		logger.Errorf("LocalLogin get user:%s group error, error msg:%s", args.Account, err.Error())
		return nil, 0, err
	}
	allUserGroupID, err := common.GetAllUserGroup()
	if err != nil {
		logger.Errorf("LocalLogin get all user group error, error msg:%s", err.Error())
		return nil, 0, err
	}
	groupIDList = append(groupIDList, allUserGroupID)

	err = zadigCache.NewRedisCache(config.RedisUserTokenDB()).Write(user.UID, token, time.Duration(systemSettings.TokenExpirationTime)*time.Hour)
	if err != nil {
		logger.Errorf("failed to write token into cache, error: %s\n warn: this will cause login failure", err)
	}

	return &User{
		Uid:          user.UID,
		Token:        token,
		Email:        user.Email,
		Phone:        user.Phone,
		Name:         user.Name,
		Account:      user.Account,
		GroupIDs:     groupIDList,
		IdentityType: user.IdentityType,
	}, 0, nil
}

func LocalLogout(userID string, logger *zap.SugaredLogger) (bool, string, error) {
	userInfo, err := orm.GetUserByUid(userID, repository.DB)
	if err != nil {
		logger.Errorf("LocalLogout get user:%s error, error msg:%s", userID, err.Error())
		return false, "", err
	}

	logger.Infof("user Info: %v", userInfo)

	err = zadigCache.NewRedisCache(config.RedisUserTokenDB()).Delete(userID)
	if err != nil {
		logger.Errorf("failed to void token, error: %s", err)
		return false, "", fmt.Errorf("failed to void token, error: %s", err)
	}

	if userInfo.IdentityType != config.OauthIdentityType {
		return false, "", nil
	}

	// if we found a user with ouath login type, we check if the connector is still there
	// if the connector exist, we check if the logout configuration is enabled and return.
	connectorInfo, err := orm.GetConnectorInfo(config.OauthIdentityType, repository.DexDB)
	if err != nil {
		logger.Errorf("LocalLogout get connector info error, error msg:%s", err.Error())
		return false, "", err
	}

	if connectorInfo.EnableLogOut {
		return true, connectorInfo.LogoutRedirectURL, nil
	}

	return false, "", nil
}

var store = base64Captcha.DefaultMemStore

func GetCaptcha(logger *zap.SugaredLogger) (string, string, error) {
	driver := base64Captcha.DefaultDriverDigit

	c := base64Captcha.NewCaptcha(driver, store)
	id, b64s, err := c.Generate()
	if err != nil {
		logger.Errorf("failed to generate captcha, error: %s", err)
		return "", "", fmt.Errorf("captcha generate error")
	}
	return id, b64s, nil
}

type SsoTokenClaims struct {
	UserID  string `json:"userId"`
	Account string `json:"account"`
	jwt.StandardClaims
}

func SsoTokenCallback(tokenString string, logger *zap.SugaredLogger) (string, error) {
	parsedToken, err := jwt.ParseWithClaims(tokenString, &SsoTokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(configbase.SsoTokenSecret()), nil
	})
	if err != nil {
		return "", err
	}

	claims, ok := parsedToken.Claims.(*SsoTokenClaims)
	if !ok || !parsedToken.Valid {
		return "", fmt.Errorf("invalid token")
	}

	log.Infof("[SsoTokenCallback]: userId: %s, account: %s", claims.UserID, claims.Account)

	var userLogin *models.UserLogin
	identityType := config.SsoTokenIdentityType
	user, err := orm.GetUser(claims.UserID, identityType, repository.DB)
	if err != nil {
		err = fmt.Errorf("SsoTokenLogin get user account:%s error, error msg:%s", claims.UserID, err.Error())
		log.Errorf(err.Error())
		return "", err
	}
	if user == nil {
		uid, _ := uuid.NewUUID()
		user := &models.User{
			Name:         claims.Account,
			Email:        fmt.Sprintf("%s-%s@poc.example", claims.UserID, claims.Account),
			IdentityType: identityType,
			Account:      claims.UserID,
			UID:          uid.String(),
		}

		tx := repository.DB.Begin()
		defer func() {
			if r := recover(); r != nil {
				tx.Rollback()
			}
		}()
		err = orm.CreateUser(user, tx)
		if err != nil {
			tx.Rollback()
			logger.Errorf("[SsoTokenCallback] CreateUser :%v error, error msg:%s", user, err.Error())
			var mysqlErr *mysql.MySQLError
			if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
				return "", fmt.Errorf("存在相同用户名")
			}
			return "", fmt.Errorf("创建用户失败, error: %s", err.Error())
		}
		userLogin := &models.UserLogin{
			UID:           user.UID,
			LastLoginTime: time.Now().Unix(),
			LoginId:       user.Account,
			LoginType:     int(config.AccountLoginType),
		}
		err = orm.CreateUserLogin(userLogin, tx)
		if err != nil {
			tx.Rollback()
			err = fmt.Errorf("[SsoTokenCallback] CreateUserLogin:%v error, error msg:%s", user, err.Error())
			log.Errorf(err.Error())
			return "", err
		}
		if tx.Commit().Error != nil {
			return "", fmt.Errorf("创建用户登录信息失败, error: %s", tx.Commit().Error)
		}
	} else {
		userLogin, err = orm.GetUserLogin(user.UID, claims.UserID, config.AccountLoginType, repository.DB)
		if err != nil {
			err = fmt.Errorf("SsoTokenLogin get user:%s user login not exist, error msg:%s", claims.UserID, err.Error())
			log.Errorf(err.Error())
			return "", err
		}
	}

	if userLogin != nil {
		err = CheckSignature(userLogin.LastLoginTime, logger)
		if err != nil {
			return "", err
		}
	}

	userLogin.LastLoginTime = time.Now().Unix()
	err = orm.UpdateUserLogin(userLogin.UID, userLogin, repository.DB)
	if err != nil {
		err = fmt.Errorf("[SsoTokenCallback] user:%s update user login info error, error msg:%s", claims.UserID, err.Error())
		log.Errorf(err.Error())
		return "", err
	}

	systemSettings, err := aslan.New(configbase.AslanServiceAddress()).GetSystemSecurityAndPrivacySettings()
	if err != nil {
		err = fmt.Errorf("failed to get system security settings, error: %s", err)
		log.Errorf(err.Error())
		return "", err
	}

	token, err := CreateToken(&Claims{
		Name:              user.Name,
		UID:               user.UID,
		Email:             user.Email,
		PreferredUsername: user.Account,
		StandardClaims: jwt.StandardClaims{
			Audience:  setting.ProductName,
			ExpiresAt: time.Now().Add(time.Duration(systemSettings.TokenExpirationTime) * time.Hour).Unix(),
		},
		FederatedClaims: FederatedClaims{
			ConnectorId: user.IdentityType,
			UserId:      user.Account,
		},
	})
	if err != nil {
		err = fmt.Errorf("[SsoTokenCallback] user:%s create token error, error msg:%s", claims.UserID, err.Error())
		log.Errorf(err.Error())
		return "", err
	}

	err = zadigCache.NewRedisCache(config.RedisUserTokenDB()).Write(user.UID, token, time.Duration(systemSettings.TokenExpirationTime)*time.Hour)
	if err != nil {
		logger.Errorf("failed to write token into cache, error: %s\n warn: this will cause login failure", err)
	}

	return token, nil
}
