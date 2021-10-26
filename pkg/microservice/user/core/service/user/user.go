package user

import (
	"fmt"
	"github.com/google/uuid"
	"github.com/koderover/zadig/pkg/microservice/user/config"
	"github.com/koderover/zadig/pkg/microservice/user/core"
	"github.com/koderover/zadig/pkg/microservice/user/core/repository/models"
	"github.com/koderover/zadig/pkg/microservice/user/core/repository/mysql"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	Name     string `json:"name"`
	Password string `json:"password"`
	Email    string `json:"email"`
	Phone    string `json:"phone,omitempty"`
}

type Users struct {
	Name    string `json:"name"`
	PerPage int    `json:"per_page"`
	Page    int    `json:"page"`
}

type UserInfo struct {
	LastLoginTime int64  `json:"lastLoginTime"`
	Uid           string `json:"uid"`
	Name          string `json:"name"`
	IdentityType  string `gorm:"default:'unknown'" json:"identity_type"`
	Email         string `json:"email"`
	Phone         string `json:"phone"`
}

type Password struct {
	Uid         string `json:"uid"`
	OldPassword string `json:"oldPassword"`
	NewPassword string `json:"newPassword"`
}

type GetUsersResp struct {
	Users      []UserInfo `json:"users"`
	TotalCount int        `json:"totalCount"`
}

func GetUser(uid string, logger *zap.SugaredLogger) (*models.User, error) {
	user, err := mysql.GetUserByUid(uid, core.DB)
	if err != nil {
		logger.Errorf("GetUser getUserByUid:%s error, error msg:%s", uid, err.Error())
		return nil, err
	}
	return user, nil
}

func GetUsers(args *Users, logger *zap.SugaredLogger) (*GetUsersResp, error) {
	count, err := mysql.GetUsersCount(args.Name)
	if err != nil {
		logger.Errorf("GetUsers GetUsersCount By name:%s error, error msg:%s", args.Name, err.Error())
		return nil, err
	}
	users, err := mysql.GetUsers(args.Page, args.PerPage, args.Name, core.DB)
	if err != nil {
		logger.Errorf("GetUsers GetUsers By name:%s error, error msg:%s", args.Name, err.Error())
		return nil, err
	}
	var uids []string
	for _, user := range users {
		uids = append(uids, user.Uid)
	}
	userLogins, err := mysql.GetUserLogins(uids, core.DB)
	if err != nil {
		logger.Errorf("GetUsers GetUserLogins By uids:%s error, error msg:%s", uids, err.Error())
		return nil, err
	}
	userLoginMap := make(map[string]models.UserLogin)
	for _, userLogin := range *userLogins {
		userLoginMap[userLogin.Uid] = userLogin
	}
	var usersInfo []UserInfo
	for _, user := range users {
		if userLogin, ok := userLoginMap[user.Uid]; ok {
			usersInfo = append(usersInfo, UserInfo{
				LastLoginTime: userLogin.LastLoginTime,
				Uid:           user.Uid,
				Phone:         user.Phone,
				Name:          user.Name,
				Email:         user.Email,
				IdentityType:  user.IdentityType,
			})
		} else {
			logger.Error("user:%s login info not exist")
		}
	}
	return &GetUsersResp{
		Users:      usersInfo,
		TotalCount: count,
	}, nil
}

func CreateUser(args *User, logger *zap.SugaredLogger) error {
	uid, _ := uuid.NewUUID()
	user := &models.User{
		Name:         args.Name,
		Email:        args.Email,
		IdentityType: config.SystemIdentityType,
		Phone:        args.Phone,
		Uid:          uid.String(),
	}
	tx := core.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()
	err := mysql.CreateUser(user, tx)
	if err != nil {
		tx.Rollback()
		logger.Errorf("CreateUser CreateUser :%v error, error msg:%s", user, err.Error())
		return err
	}
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(args.Password), bcrypt.DefaultCost)
	userLogin := &models.UserLogin{
		Uid:           user.Uid,
		Password:      string(hashedPassword),
		LastLoginTime: 0,
	}
	err = mysql.CreateUserLogin(userLogin, tx)
	if err != nil {
		tx.Rollback()
		logger.Errorf("CreateUser CreateUserLogin:%v error, error msg:%s", user, err.Error())
		return err
	}
	return tx.Commit().Error
}

func UpdatePassword(args *Password, logger *zap.SugaredLogger) error {
	user, err := mysql.GetUserByUid(args.Uid, core.DB)
	if err != nil {
		logger.Errorf("UpdatePassword GetUserByUid:%s error, error msg:%s", args.Uid, err.Error())
		return err
	}
	if user == nil {
		return fmt.Errorf("user not exist")
	}
	userLogin, err := mysql.GetUserLogin(user.Uid, core.DB)
	if err != nil {
		logger.Errorf("UpdatePassword GetUserLogin:%s error, error msg:%s", args.Uid, err.Error())
		return err
	}
	if userLogin == nil {
		logger.Errorf("UpdatePassword GetUserLogin:%s not exist", args.Uid)
		return fmt.Errorf("userLogin not exist")
	}
	password := []byte(args.OldPassword)
	err = bcrypt.CompareHashAndPassword([]byte(userLogin.Password), password)
	if err == bcrypt.ErrMismatchedHashAndPassword {
		return fmt.Errorf("password is wrong")
	}
	if err != nil {
		logger.Errorf("UpdatePassword CompareHashAndPassword userLogin password:%s, password:%s error,"+
			" error msg:%s", userLogin.Password, password, err.Error())
		return err
	}
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(args.NewPassword), bcrypt.DefaultCost)
	userLogin = &models.UserLogin{
		Uid:      user.Uid,
		Password: string(hashedPassword),
	}
	err = mysql.UpdateUserLogin(user.Uid, userLogin, core.DB)
	if err != nil {
		logger.Errorf("UpdatePassword UpdateUserLogin:%v error, error msg:%s", userLogin, err.Error())
		return err
	}
	return nil
}
