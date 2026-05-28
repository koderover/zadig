package handler

import (
	"fmt"
	"io"
	"strconv"

	"github.com/gin-gonic/gin"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	terminalaudit "github.com/koderover/zadig/v2/pkg/shared/terminalaudit"
)

func ListTerminalSessions(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}
	if !ctx.Resources.IsSystemAdmin {
		ctx.UnAuthorized = true
		return
	}

	args := &commonmodels.TerminalSessionListArgs{
		Status:      c.Query("status"),
		SessionType: c.Query("sessionType"),
		ProjectName: c.Query("projectName"),
		EnvName:     c.Query("envName"),
		ServiceName: c.Query("serviceName"),
		Username:    c.Query("username"),
		TargetName:  c.Query("targetName"),
		RemoteAddr:  c.Query("remoteAddr"),
		StartTime:   parseInt64Query(c, "startTime"),
		EndTime:     parseInt64Query(c, "endTime"),
		PageNum:     parseInt64WithDefault(c, "pageNum", 1),
		PageSize:    parseInt64WithDefault(c, "pageSize", 20),
	}
	ctx.Resp, ctx.RespErr = terminalaudit.ListSessions(args)
}

func GetTerminalSession(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}
	if !ctx.Resources.IsSystemAdmin {
		ctx.UnAuthorized = true
		return
	}
	ctx.Resp, ctx.RespErr = terminalaudit.GetSession(c.Param("sessionID"))
}

func GetTerminalCast(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}
	if !ctx.Resources.IsSystemAdmin {
		ctx.UnAuthorized = true
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
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}
	if !ctx.Resources.IsSystemAdmin {
		ctx.UnAuthorized = true
		return
	}

	args := &commonmodels.TerminalCommandListArgs{
		SessionID:   c.Query("sessionID"),
		ProjectName: c.Query("projectName"),
		Username:    c.Query("username"),
		TargetName:  c.Query("targetName"),
		RemoteAddr:  c.Query("remoteAddr"),
		Command:     c.Query("command"),
		StartTime:   parseInt64Query(c, "startTime"),
		EndTime:     parseInt64Query(c, "endTime"),
		PageNum:     parseInt64WithDefault(c, "pageNum", 1),
		PageSize:    parseInt64WithDefault(c, "pageSize", 20),
	}
	ctx.Resp, ctx.RespErr = terminalaudit.ListCommands(args)
}

func TerminateTerminalSession(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}
	if !ctx.Resources.IsSystemAdmin {
		ctx.UnAuthorized = true
		return
	}
	ctx.RespErr = terminalaudit.TerminateSession(c.Param("sessionID"))
}

func parseInt64Query(c *gin.Context, key string) int64 {
	return parseInt64WithDefault(c, key, 0)
}

func parseInt64WithDefault(c *gin.Context, key string, defaultValue int64) int64 {
	raw := c.Query(key)
	if raw == "" {
		return defaultValue
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return defaultValue
	}
	return value
}
