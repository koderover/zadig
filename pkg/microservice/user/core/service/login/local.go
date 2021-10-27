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
)

type LoginArgs struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type User struct {
	Uid          string `json:"uid"`
	Token        string `json:"token"`
	Email        string `json:"email"`
	Phone        string `json:"phone"`
	Name         string `json:"name"`
	IdentityType string `json:"identityType"`
}

func LocalLogin(args *LoginArgs, logger *zap.SugaredLogger) (*User, error) {
	user, err := orm.GetUser(args.Email, config.SystemIdentityType, core.DB)
	if err != nil {
		logger.Errorf("InternalLogin get user email:%s error", args.Email)
		return nil, err
	}
	if user == nil {
		return nil, fmt.Errorf("user not exist")
	}
	userLogin, err := orm.GetUserLogin(user.UID, core.DB)
	if err != nil {
		logger.Errorf("InternalLogin get user:%s user login not exist, error msg:%s", args.Email, err.Error())
		return nil, err
	}
	if userLogin == nil {
		logger.Errorf("InternalLogin user:%s user login not exist", args.Email)
		return nil, fmt.Errorf("user login not exist")
	}
	password := []byte(args.Password)
	err = bcrypt.CompareHashAndPassword([]byte(userLogin.Password), password)
	if err != nil {
		logger.Errorf("InternalLogin user:%s check password error, error msg:%s", args.Email, err.Error())
		return nil, fmt.Errorf("check password error, error msg:%s", err.Error())
	}
	userLogin.LastLoginTime = time.Now().Unix()
	err = orm.UpdateUserLogin(userLogin.UID, userLogin, core.DB)
	if err != nil {
		logger.Errorf("InternalLogin user:%s update user login password error, error msg:%s", args.Email, err.Error())
		return nil, err
	}
	token, err := CreateToken(&Claims{
		Name:  user.Name,
		Email: user.Email,
		Uid:   user.UID,
		StandardClaims: jwt.StandardClaims{
			Audience:  setting.ProductName,
			ExpiresAt: time.Now().Add(24 * time.Hour).Unix(),
		},
	})
	if err != nil {
		logger.Errorf("InternalLogin user:%s create token error, error msg:%s", args.Email, err.Error())
		return nil, err
	}
	return &User{
		Uid:          user.UID,
		Token:        token,
		Email:        user.Email,
		Phone:        user.Phone,
		Name:         user.Name,
		IdentityType: user.IdentityType,
	}, nil
}
