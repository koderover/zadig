package user

import (
	"fmt"

	"github.com/dexidp/dex/connector/ldap"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	ldapv3 "github.com/go-ldap/ldap/v3"

	"github.com/koderover/zadig/pkg/microservice/systemconfig/core/service"
	"github.com/koderover/zadig/pkg/microservice/user/core/service/user"
	"github.com/koderover/zadig/pkg/shared/client/systemconfig"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
)

func searchAndSyncUser(si *service.Connector, logger *zap.SugaredLogger) error {
	if si == nil || si.Config == nil {
		logger.Error("can't find connector")
		return fmt.Errorf("can't find connector")
	}
	config, _ := si.Config.(*ldap.Config)
	l, err := ldapv3.Dial("tcp", config.Host)
	if err != nil {
		logger.Error(err)
		return err
	}
	defer l.Close()

	err = l.Bind(config.BindDN, config.BindPW)
	if err != nil {
		logger.Error(err)
		return err
	}

	searchRequest := ldapv3.NewSearchRequest(
		config.GroupSearch.BaseDN,
		ldapv3.ScopeWholeSubtree, ldapv3.NeverDerefAliases, 0, 0, false,
		config.GroupSearch.Filter, // The filter to apply
		[]string{config.UserSearch.NameAttr, config.UserSearch.EmailAttr}, // A list attributes to retrieve
		nil,
	)

	sr, err := l.Search(searchRequest)
	if err != nil {
		logger.Error(err)
		return err
	}

	for _, entry := range sr.Entries {
		_, err := user.SyncUser(&user.SyncUserInfo{
			Account:      entry.GetAttributeValue(config.UserSearch.NameAttr),
			IdentityType: si.ID,
		}, logger)
		if err != nil {
			return err
		}
	}
	return nil
}

func SyncLdapUser(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	ctx.Logger.Info("SyncLdapUser start")
	ldapId := c.Param("ldapId")
	systemConfigClient := systemconfig.New()
	si, err := systemConfigClient.GetConnector(ldapId)
	if err != nil {
		ctx.Logger.Error(err)
		ctx.Err = err
		return
	}
	if err != nil {
		ctx.Err = err
		return
	}
	ctx.Err = searchAndSyncUser(si, ctx.Logger)
}

func GetUser(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	ctx.Resp, ctx.Err = user.GetUser(c.Param("uid"), ctx.Logger)
}

func GetPersonalUser(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	uid := c.Param("uid")
	if ctx.UserID != uid {
		ctx.Err = fmt.Errorf("token uid doesn't match uid")
	}
	ctx.Resp, ctx.Err = user.GetUser(uid, ctx.Logger)
}

func ListUsers(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	args := &user.QueryArgs{}
	if err := c.ShouldBindJSON(args); err != nil {
		ctx.Err = err
		return
	}
	if len(args.UIDs) > 0 {
		ctx.Resp, ctx.Err = user.SearchUsersByUIDs(args.UIDs, ctx.Logger)
		return
	} else {
		ctx.Resp, ctx.Err = user.SearchUsers(args, ctx.Logger)
		return
	}
}

func CreateUser(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	args := &user.User{}
	if err := c.ShouldBindJSON(args); err != nil {
		ctx.Err = err
		return
	}
	ctx.Resp, ctx.Err = user.CreateUser(args, ctx.Logger)
}

func UpdatePassword(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	args := &user.Password{}
	if err := c.ShouldBindJSON(args); err != nil {
		ctx.Err = err
		return
	}
	args.Uid = c.Param("uid")
	ctx.Err = user.UpdatePassword(args, ctx.Logger)
}
