package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/environment/service"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
)

func ConnectSshPmExec(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	projectName := c.Query("projectName")
	ip := c.Query("ip")
	name := c.Param("name")

	ctx.Err = service.ConnectSshPmExec(c, ctx.UserName, name, projectName, ip, ctx.Logger)
}
