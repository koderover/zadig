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
	"fmt"
	"time"

	"github.com/golang-jwt/jwt"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"

	"github.com/koderover/zadig/pkg/microservice/user/config"
	"github.com/koderover/zadig/pkg/microservice/user/core"
	"github.com/koderover/zadig/pkg/microservice/user/core/repository/orm"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/shared/client/plutusvendor"
)

type LoginArgs struct {
	Account  string `json:"account"`
	Password string `json:"password"`
}

type User struct {
	Uid          string `json:"uid"`
	Token        string `json:"token"`
	Email        string `json:"email"`
	Phone        string `json:"phone"`
	Name         string `json:"name"`
	Account      string `json:"account"`
	IdentityType string `json:"identityType"`
}

type CheckSignatureRes struct {
	Code        int                    `json:"code"`
	Description string                 `json:"description"`
	Extra       map[string]interface{} `json:"extra"`
	Message     string                 `json:"message"`
	Type        string                 `json:"type"`
}

func CheckSignature(ifLoggedIn bool, logger *zap.SugaredLogger) error {
	userNum, err := orm.CountUser(core.DB)
	if err != nil {
		return err
	}
	vendorClient := plutusvendor.New()
	err = vendorClient.Health()
	if err == nil {
		res, checkErr := vendorClient.CheckSignature(userNum)
		if checkErr != nil {
			return checkErr
		}
		if res.Code == 6694 {
			if ifLoggedIn {
				return nil
			} else {
				return fmt.Errorf("系统使用用户数量已达授权人数上限，请联系系统管理员")
			}

		}
	}
	return nil
}

func LocalLogin(args *LoginArgs, logger *zap.SugaredLogger) (*User, error) {
	user, err := orm.GetUser(args.Account, config.SystemIdentityType, core.DB)
	if err != nil {
		logger.Errorf("InternalLogin get user account:%s error", args.Account)
		return nil, err
	}
	if user == nil {
		return nil, fmt.Errorf("user not exist")
	}
	userLogin, err := orm.GetUserLogin(user.UID, args.Account, config.AccountLoginType, core.DB)
	if err != nil {
		logger.Errorf("LocalLogin get user:%s user login not exist, error msg:%s", args.Account, err.Error())
		return nil, err
	}
	if userLogin == nil {
		logger.Errorf("InternalLogin user:%s user login not exist", args.Account)
		return nil, fmt.Errorf("user login not exist")
	}
	password := []byte(args.Password)
	err = bcrypt.CompareHashAndPassword([]byte(userLogin.Password), password)
	if err == bcrypt.ErrMismatchedHashAndPassword {
		return nil, fmt.Errorf("password is wrong")
	}
	if err != nil {
		logger.Errorf("LocalLogin user:%s check password error, error msg:%s", args.Account, err)
		return nil, fmt.Errorf("check password error, error msg:%s", err)
	}
	err = CheckSignature(userLogin.LastLoginTime > 0, logger)
	if err != nil {
		return nil, err
	}
	userLogin.LastLoginTime = time.Now().Unix()
	err = orm.UpdateUserLogin(userLogin.UID, userLogin, core.DB)
	if err != nil {
		logger.Errorf("LocalLogin user:%s update user login password error, error msg:%s", args.Account, err.Error())
		return nil, err
	}
	token, err := CreateToken(&Claims{
		Name:              user.Name,
		UID:               user.UID,
		Email:             user.Email,
		PreferredUsername: user.Account,
		StandardClaims: jwt.StandardClaims{
			Audience:  setting.ProductName,
			ExpiresAt: time.Now().Add(24 * time.Hour).Unix(),
		},
		FederatedClaims: FederatedClaims{
			ConnectorId: user.IdentityType,
			UserId:      user.Account,
		},
	})
	if err != nil {
		logger.Errorf("LocalLogin user:%s create token error, error msg:%s", args.Account, err.Error())
		return nil, err
	}

	return &User{
		Uid:          user.UID,
		Token:        token,
		Email:        user.Email,
		Phone:        user.Phone,
		Name:         user.Name,
		Account:      user.Account,
		IdentityType: user.IdentityType,
	}, nil
}
