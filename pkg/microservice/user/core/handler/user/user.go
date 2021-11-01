package user

import (
	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/pkg/microservice/user/core/service/user"
	"github.com/koderover/zadig/pkg/shared/client/systemconfig"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"

	e "github.com/koderover/zadig/pkg/tool/errors"
)

func SyncLdapUser(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
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
	ctx.Err = user.SearchAndSyncUser(&systemconfig.Connector{
		ConnectorBase: si.ConnectorBase,
		ID:            si.ID,
		Name:          si.Name,
		Config:        si.Config,
	}, ctx.Logger)
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
		ctx.Err = e.ErrForbidden
		return
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
