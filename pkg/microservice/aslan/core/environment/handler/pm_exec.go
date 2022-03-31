package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/environment/service"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

func ConnectSshPmExec(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	projectName := c.Query("projectName")
	ip := c.Query("ip")
	name := c.Param("name")
	if projectName == "" || ip == "" || name == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("param projectName or ip or name is empty")
	}
	colsStr := c.DefaultQuery("cols", "135")
	cols, err := strconv.Atoi(colsStr)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
	}
	rowsStr := c.DefaultQuery("rows", "40")
	rows, err := strconv.Atoi(rowsStr)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
	}

	ctx.Err = service.ConnectSshPmExec(c, ctx.UserName, name, projectName, ip, cols, rows, ctx.Logger)
}
