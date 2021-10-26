package user

import (
	"github.com/gin-gonic/gin"
	"github.com/koderover/zadig/pkg/microservice/user/core/service/user"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	"strconv"
)

func GetUser(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	ctx.Resp, ctx.Err = user.GetUser(c.Param("uid"), ctx.Logger)
}

func GetUsers(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	args := &user.Users{}
	args.Name = c.Query("name")
	args.Page, _ = strconv.Atoi(c.Query("page"))
	args.PerPage, _ = strconv.Atoi(c.Query("per_page"))
	ctx.Resp, ctx.Err = user.GetUsers(args, ctx.Logger)
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
