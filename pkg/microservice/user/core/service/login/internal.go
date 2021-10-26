package login

import (
	"fmt"
	"github.com/dgrijalva/jwt-go"
	"github.com/koderover/zadig/pkg/microservice/user/config"
	"github.com/koderover/zadig/pkg/microservice/user/core"
	"github.com/koderover/zadig/pkg/microservice/user/core/repository/mysql"
	"github.com/koderover/zadig/pkg/util"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"time"
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

func InternalLogin(args *LoginArgs, logger *zap.SugaredLogger) (*User, error) {
	user, err := mysql.GetUser(args.Email, config.SystemIdentityType, core.DB)
	if err != nil {
		logger.Errorf("InternalLogin get user email:%s error", args.Email)
		return nil, err
	}
	if user == nil {
		return nil, fmt.Errorf("user not exist")
	}
	userLogin, err := mysql.GetUserLogin(user.Uid, core.DB)
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
	err = mysql.UpdateUserLogin(userLogin.Uid, userLogin, core.DB)
	if err != nil {
		logger.Errorf("InternalLogin user:%s update user login password error, error msg:%s", args.Email, err.Error())
		return nil, err
	}
	token, err := util.CreateToken(&util.Claims{
		Name:  user.Name,
		Email: user.Email,
		Uid:   user.Uid,
		StandardClaims: jwt.StandardClaims{
			Audience:  config.Audience,
			ExpiresAt: time.Now().Add(24 * time.Hour).Unix(),
		},
	})
	if err != nil {
		logger.Errorf("InternalLogin user:%s create token error, error msg:%s", args.Email, err.Error())
		return nil, err
	}
	return &User{
		Uid:          user.Uid,
		Token:        token,
		Email:        user.Email,
		Phone:        user.Phone,
		Name:         user.Name,
		IdentityType: user.IdentityType,
	}, nil
}
