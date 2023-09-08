package handler

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/agent/service"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
)

func CreateAgent(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Logger.Errorf("failed to generate authorization info for user: %s, error: %s", ctx.UserID, err)
		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		ctx.UnAuthorized = true
		return
	}

	//args := new(service.CreateAgentRequest)

}

func UpdateAgent(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Logger.Errorf("failed to generate authorization info for user: %s, error: %s", ctx.UserID, err)
		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		ctx.UnAuthorized = true
		return
	}
}

func DeleteAgent(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Logger.Errorf("failed to generate authorization info for user: %s, error: %s", ctx.UserID, err)
		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		ctx.UnAuthorized = true
		return
	}
}

func ListAgents(c *gin.Context) {

}

func GetAgent(c *gin.Context) {

}

func RegisterAgent(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(service.RegisterAgentRequest)
	if err := c.ShouldBindJSON(args); err != nil {
		ctx.Err = fmt.Errorf("invalid request: %s", err)
		return
	}

	// TODO: 是否增加操作日志
	ctx.Resp, ctx.Err = service.RegisterAgent(args, ctx.Logger)
}

func VerifyAgent(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(service.VerifyAgentRequest)
	if err := c.ShouldBindJSON(args); err != nil {
		ctx.Err = fmt.Errorf("invalid request: %s", err)
		return
	}

	ctx.Resp, ctx.Err = service.VerifyAgent(args, ctx.Logger)
}

func PingAgent(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(service.HeartbeatRequest)
	if err := c.ShouldBindJSON(args); err != nil {
		ctx.Err = fmt.Errorf("invalid request: %s", err)
		return
	}

	ctx.Resp, ctx.Err = service.Heartbeat(args, ctx.Logger)
}
