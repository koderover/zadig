package user

import (
	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/pkg/microservice/user/core/service/user"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
)

func GetUser(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	ctx.Resp, ctx.Err = user.GetUser(c.Param("uid"), ctx.Logger)
}

func ListUsers(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	args := &user.QueryArgs{}
	if err := c.ShouldBindQuery(args); err != nil {
		ctx.Err = err
		return
	}
	ctx.Resp, ctx.Err = user.SeachUsers(args, ctx.Logger)
}

func CreateUser(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	args := &user.User{}
	if err := c.ShouldBindJSON(args); err != nil {
		ctx.Err = err
		return
	}
	ctx.Err = user.CreateUser(args, ctx.Logger)
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
