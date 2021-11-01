package user

import (
	"fmt"
	"time"

	"github.com/dexidp/dex/connector/ldap"
	ldapv3 "github.com/go-ldap/ldap/v3"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"

	"github.com/koderover/zadig/pkg/microservice/user/config"
	"github.com/koderover/zadig/pkg/microservice/user/core"
	"github.com/koderover/zadig/pkg/microservice/user/core/repository/models"
	"github.com/koderover/zadig/pkg/microservice/user/core/repository/orm"
	"github.com/koderover/zadig/pkg/shared/client/systemconfig"
)

type User struct {
	Name     string `json:"name,omitempty"`
	Password string `json:"password"`
	Email    string `json:"email,omitempty"`
	Account  string `json:"account"`
	Phone    string `json:"phone,omitempty"`
}

type QueryArgs struct {
	Account string   `json:"account,omitempty"`
	UIDs    []string `json:"uids,omitempty"`
	PerPage int      `json:"per_page,omitempty"`
	Page    int      `json:"page,omitempty"`
}

type UserInfo struct {
	LastLoginTime int64  `json:"lastLoginTime"`
	Uid           string `json:"uid"`
	Name          string `json:"name"`
	IdentityType  string `gorm:"default:'unknown'" json:"identity_type"`
	Email         string `json:"email"`
	Phone         string `json:"phone"`
	Account       string `json:"account"`
}

type Password struct {
	Uid         string `json:"uid"`
	OldPassword string `json:"oldPassword"`
	NewPassword string `json:"newPassword"`
}

type UsersResp struct {
	Users      []UserInfo `json:"users"`
	TotalCount int64      `json:"totalCount"`
}

type SyncUserInfo struct {
	Account      string `json:"account"`
	IdentityType string `json:"identityType"`
	Name         string `json:"name"`
}

func SearchAndSyncUser(si *systemconfig.Connector, logger *zap.SugaredLogger) error {
	if si == nil || si.Config == nil {
		logger.Error("can't find connector")
		return fmt.Errorf("can't find connector")
	}
	config, ok := si.Config.(*ldap.Config)
	if !ok {
		return fmt.Errorf("connector config error")
	}
	l, err := ldapv3.Dial("tcp", config.Host)
	if err != nil {
		logger.Errorf("ldap dial host:%s error, error msg:%s", config.Host, err)
		return err
	}
	defer l.Close()

	err = l.Bind(config.BindDN, config.BindPW)
	if err != nil {
		logger.Errorf("ldap bind host:%s error, error msg:%s", config.Host, err)
		return err
	}

	searchRequest := ldapv3.NewSearchRequest(
		config.GroupSearch.BaseDN,
		ldapv3.ScopeWholeSubtree, ldapv3.NeverDerefAliases, 0, 0, false,
		config.GroupSearch.Filter,            // The filter to apply
		[]string{config.UserSearch.NameAttr}, // A list attributes to retrieve
		nil,
	)

	sr, err := l.Search(searchRequest)
	if err != nil {
		logger.Errorf("ldap search host:%s error, error msg:%s", config.Host, err)
		return err
	}

	for _, entry := range sr.Entries {
		_, err := SyncUser(&SyncUserInfo{
			Account:      entry.GetAttributeValue(config.UserSearch.NameAttr),
			IdentityType: si.ID,
		}, logger)
		if err != nil {
			logger.Errorf("ldap host:%s sync user error, error msg:%s", config.Host, err)
			return err
		}
	}
	return nil
}

func GetUser(uid string, logger *zap.SugaredLogger) (*UserInfo, error) {
	user, err := orm.GetUserByUid(uid, core.DB)
	if err != nil {
		logger.Errorf("GetUser getUserByUid:%s error, error msg:%s", uid, err.Error())
		return nil, err
	}
	userLogin, err := orm.GetUserLogin(uid, user.Account, config.AccountLoginType, core.DB)
	if err != nil {
		logger.Errorf("GetUser GetUserLogin:%s error, error msg:%s", uid, err.Error())
		return nil, err
	}
	userInfo := mergeUserLogin([]models.User{*user}, []models.UserLogin{*userLogin}, logger)
	return &userInfo[0], nil
}

func SearchUsers(args *QueryArgs, logger *zap.SugaredLogger) (*UsersResp, error) {
	count, err := orm.GetUsersCount(args.Account)
	if err != nil {
		logger.Errorf("SeachUsers GetUsersCount By account:%s error, error msg:%s", args.Account, err.Error())
		return nil, err
	}
	if count == 0 {
		return &UsersResp{
			TotalCount: 0,
		}, nil
	}

	users, err := orm.ListUsers(args.Page, args.PerPage, args.Account, core.DB)
	if err != nil {
		logger.Errorf("SeachUsers SeachUsers By account:%s error, error msg:%s", args.Account, err.Error())
		return nil, err
	}
	var uids []string
	for _, user := range users {
		uids = append(uids, user.UID)
	}
	userLogins, err := orm.ListUserLogins(uids, core.DB)
	if err != nil {
		logger.Errorf("SeachUsers ListUserLogins By uids:%s error, error msg:%s", uids, err.Error())
		return nil, err
	}
	usersInfo := mergeUserLogin(users, *userLogins, logger)
	return &UsersResp{
		Users:      usersInfo,
		TotalCount: count,
	}, nil
}

func mergeUserLogin(users []models.User, userLogins []models.UserLogin, logger *zap.SugaredLogger) []UserInfo {
	userLoginMap := make(map[string]models.UserLogin)
	for _, userLogin := range userLogins {
		userLoginMap[userLogin.UID] = userLogin
	}
	var usersInfo []UserInfo
	for _, user := range users {
		if userLogin, ok := userLoginMap[user.UID]; ok {
			usersInfo = append(usersInfo, UserInfo{
				LastLoginTime: userLogin.LastLoginTime,
				Uid:           user.UID,
				Phone:         user.Phone,
				Name:          user.Name,
				Email:         user.Email,
				IdentityType:  user.IdentityType,
				Account:       user.Account,
			})
		} else {
			logger.Error("user:%s login info not exist")
		}
	}
	return usersInfo
}

func SearchUsersByUIDs(uids []string, logger *zap.SugaredLogger) (*UsersResp, error) {
	users, err := orm.ListUsersByUIDs(uids, core.DB)
	if err != nil {
		logger.Errorf("SearchUsersByUIDs SeachUsers By uids:%s error, error msg:%s", uids, err.Error())
		return nil, err
	}
	userLogins, err := orm.ListUserLogins(uids, core.DB)
	if err != nil {
		logger.Errorf("SearchUsersByUIDs ListUserLogins By uids:%s error, error msg:%s", uids, err.Error())
		return nil, err
	}
	usersInfo := mergeUserLogin(users, *userLogins, logger)
	return &UsersResp{
		Users:      usersInfo,
		TotalCount: int64(len(usersInfo)),
	}, nil
}

func getLoginId(user *models.User, loginType config.LoginType) string {
	switch loginType {
	case config.AccountLoginType:
		return user.Account
	default:
		return user.Account
	}

}

func CreateUser(args *User, logger *zap.SugaredLogger) (*models.User, error) {
	uid, _ := uuid.NewUUID()
	user := &models.User{
		Name:         args.Name,
		Email:        args.Email,
		IdentityType: config.SystemIdentityType,
		Phone:        args.Phone,
		Account:      args.Account,
		UID:          uid.String(),
	}
	tx := core.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()
	err := orm.CreateUser(user, tx)
	if err != nil {
		tx.Rollback()
		logger.Errorf("CreateUser CreateUser :%v error, error msg:%s", user, err.Error())
		return nil, err
	}
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(args.Password), bcrypt.DefaultCost)
	userLogin := &models.UserLogin{
		UID:           user.UID,
		Password:      string(hashedPassword),
		LastLoginTime: 0,
		LoginId:       getLoginId(user, config.AccountLoginType),
		LoginType:     int(config.AccountLoginType),
	}
	err = orm.CreateUserLogin(userLogin, tx)
	if err != nil {
		tx.Rollback()
		logger.Errorf("CreateUser CreateUserLogin:%v error, error msg:%s", user, err.Error())
		return nil, err
	}
	return user, tx.Commit().Error
}

func UpdatePassword(args *Password, logger *zap.SugaredLogger) error {
	user, err := orm.GetUserByUid(args.Uid, core.DB)
	if err != nil {
		logger.Errorf("UpdatePassword GetUserByUid:%s error, error msg:%s", args.Uid, err.Error())
		return err
	}
	if user == nil {
		return fmt.Errorf("user not exist")
	}
	userLogin, err := orm.GetUserLogin(user.UID, user.Account, config.AccountLoginType, core.DB)
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
		UID:      user.UID,
		Password: string(hashedPassword),
	}
	err = orm.UpdateUserLogin(user.UID, userLogin, core.DB)
	if err != nil {
		logger.Errorf("UpdatePassword UpdateUserLogin:%v error, error msg:%s", userLogin, err.Error())
		return err
	}
	return nil
}

func SyncUser(syncUserInfo *SyncUserInfo, logger *zap.SugaredLogger) (*models.User, error) {
	user, err := orm.GetUser(syncUserInfo.Account, syncUserInfo.IdentityType, core.DB)
	if err != nil {
		logger.Error("SyncUser get user:%s error, error msg:%s", syncUserInfo.Account, err.Error())
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
			Account:      syncUserInfo.Account,
			IdentityType: syncUserInfo.IdentityType,
		}
		err = orm.CreateUser(user, tx)
		if err != nil {
			tx.Rollback()
			logger.Error("SyncUser create user:%s error, error msg:%s", syncUserInfo.Account, err.Error())
			return nil, err
		}
	}
	userLogin, err := orm.GetUserLogin(user.UID, user.Account, config.AccountLoginType, tx)
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
			LoginId:       getLoginId(user, config.AccountLoginType),
			LoginType:     int(config.AccountLoginType),
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
