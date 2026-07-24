package handler

import (
	"fmt"
	"io"
	"strconv"

	"github.com/gin-gonic/gin"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	terminalaudit "github.com/koderover/zadig/v2/pkg/shared/terminalaudit"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

func ListTerminalSessions(c *gin.Context) {
	ctx, authorized := newTerminalAuditAdminContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	if !authorized {
		return
	}

	args := new(commonmodels.TerminalSessionListArgs)
	if err := c.ShouldBindQuery(args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}
	ctx.Resp, ctx.RespErr = terminalaudit.ListSessions(args)
}

func GetTerminalSession(c *gin.Context) {
	ctx, authorized := newTerminalAuditAdminContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	if !authorized {
		return
	}
	ctx.Resp, ctx.RespErr = terminalaudit.GetSession(c.Param("sessionID"))
}

func GetTerminalCast(c *gin.Context) {
	ctx, authorized := newTerminalAuditAdminContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	if !authorized {
		return
	}

	stream, err := terminalaudit.GetCastStream(c.Param("sessionID"))
	if err != nil {
		ctx.RespErr = err
		return
	}
	defer stream.Body.Close()

	c.Header("Content-Type", "application/octet-stream")
	if stream.FileSize > 0 {
		c.Header("Content-Length", strconv.FormatInt(stream.FileSize, 10))
	}
	c.Status(200)
	_, ctx.RespErr = io.Copy(c.Writer, stream.Body)
}

func ListTerminalCommands(c *gin.Context) {
	ctx, authorized := newTerminalAuditAdminContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	if !authorized {
		return
	}

	args := new(commonmodels.TerminalCommandListArgs)
	if err := c.ShouldBindQuery(args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}
	ctx.Resp, ctx.RespErr = terminalaudit.ListCommands(args)
}

func TerminateTerminalSession(c *gin.Context) {
	ctx, authorized := newTerminalAuditAdminContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	if !authorized {
		return
	}
	ctx.RespErr = terminalaudit.TerminateSession(c.Param("sessionID"))
}

func newTerminalAuditAdminContext(c *gin.Context) (*internalhandler.Context, bool) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return ctx, false
	}
	if !ctx.Resources.IsSystemAdmin {
		ctx.UnAuthorized = true
		return ctx, false
	}
	return ctx, true
}
