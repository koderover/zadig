package handler

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/vm"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/vm/service"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

func CreateHost(c *gin.Context) {
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

	args := new(service.CreateHostRequest)
	if err := c.ShouldBindJSON(args); err != nil {
		ctx.Err = e.ErrCreateZadigHost.AddErr(fmt.Errorf("invalid request: %s", err))
		return
	}
	if err := args.Validate(); err != nil {
		ctx.Err = e.ErrCreateZadigHost.AddErr(fmt.Errorf("invalid request: %s", err))
		return
	}

	ctx.Err = service.CreateHost(args, ctx.UserName, ctx.Logger)
}

func GetAgentAccessCmd(c *gin.Context) {
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

	platform := c.Query("platform")
	if platform == "" {
		ctx.Err = fmt.Errorf("invalid request: %s", "platform is empty")
		return
	}
	agentID := c.Param("agentID")
	if agentID == "" {
		ctx.Err = fmt.Errorf("invalid request: %s", "agentID is empty")
		return
	}
	ctx.Resp, ctx.Err = service.GetAgentAccessCmd(platform, agentID, ctx.Logger)
}

func UpdateHost(c *gin.Context) {
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

	args := new(service.CreateHostRequest)
	if err := c.ShouldBindJSON(args); err != nil {
		ctx.Err = e.ErrUpdateZadigHost.AddErr(fmt.Errorf("invalid request: %s", err))
		return
	}
	if err := args.Validate(); err != nil {
		ctx.Err = e.ErrUpdateZadigHost.AddErr(fmt.Errorf("invalid request: %s", err))
		return
	}

	ctx.Err = service.UpdateHost(args, c.Param("agentID"), ctx.UserName, ctx.Logger)
}

func DeleteHost(c *gin.Context) {
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

	ctx.Err = service.DeleteHost(c.Param("agentID"), ctx.UserName, ctx.Logger)
}

func OfflineHost(c *gin.Context) {
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

	ctx.Err = service.OfflineHost(c.Param("agentID"), ctx.UserName, ctx.Logger)
}

func UpgradeAgent(c *gin.Context) {
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

	ctx.Err = service.UpgradeAgent(c.Param("agentID"), ctx.UserName, ctx.Logger)
}

func ListHosts(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Logger.Errorf("failed to generate authorization info for user: %s, error: %s", ctx.UserID, err)
		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	ctx.Resp, ctx.Err = service.ListHosts(ctx.Logger)
}

func GetHost(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Logger.Errorf("failed to generate authorization info for user: %s, error: %s", ctx.UserID, err)
		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	ctx.Resp, ctx.Err = service.GetHost(c.Param("agentID"), ctx.Logger)
}

func ListVMLabels(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Logger.Errorf("failed to generate authorization info for user: %s, error: %s", ctx.UserID, err)
		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	ctx.Resp, ctx.Err = service.ListVMLabels(ctx.Logger)
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

func HeartbeatAgent(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(service.HeartbeatRequest)
	if err := c.ShouldBindJSON(args); err != nil {
		ctx.Err = fmt.Errorf("invalid request: %s", err)
		return
	}

	ctx.Resp, ctx.Err = service.Heartbeat(args, ctx.Logger)
}

func PollingAgentJob(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	token := c.Query("token")
	if token == "" {
		ctx.Err = fmt.Errorf("invalid request: %s", "token is empty")
		return
	}

	ctx.Resp, ctx.Err = service.PollingAgentJob(token, ctx.Logger)
}

func ReportAgentJob(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(service.ReportJobArgs)
	if err := c.ShouldBindJSON(args); err != nil {
		ctx.Err = fmt.Errorf("invalid request: %s", err)
		return
	}

	ctx.Err = service.ReportAgentJob(args, ctx.Logger)
}

func CreateWfJob2DB(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Logger.Errorf("failed to generate authorization info for user: %s, error: %s", ctx.UserID, err)
		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := new(vm.VMJob)
	if err := c.ShouldBindJSON(args); err != nil {
		ctx.Err = fmt.Errorf("invalid request: %s", err)
		return
	}
	ctx.Err = service.CreateWfJob2DB(args, ctx.Logger)
}
