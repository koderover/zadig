/*
Copyright 2023 The KodeRover Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package handler

import (
	"fmt"

	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/vm/service"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
)

func GetAgentAccessCmd(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Logger.Errorf("failed to generate authorization info for user: %s, error: %s", ctx.UserID, err)
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		ctx.UnAuthorized = true
		return
	}

	vmID := c.Param("vmid")
	if vmID == "" {
		ctx.RespErr = fmt.Errorf("invalid request: %s", "vmID is empty")
		return
	}
	ctx.Resp, ctx.RespErr = service.GetAgentAccessCmd(vmID, ctx.Logger)
}

func OfflineVM(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Logger.Errorf("failed to generate authorization info for user: %s, error: %s", ctx.UserID, err)
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		ctx.UnAuthorized = true
		return
	}

	ctx.RespErr = service.OfflineVM(c.Param("vmid"), ctx.UserName, ctx.Logger)
}

func RecoveryVM(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Logger.Errorf("failed to generate authorization info for user: %s, error: %s", ctx.UserID, err)
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		ctx.UnAuthorized = true
		return
	}

	ctx.Resp, ctx.RespErr = service.RecoveryVM(c.Param("vmid"), ctx.UserName, ctx.Logger)
}

func UpgradeAgent(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Logger.Errorf("failed to generate authorization info for user: %s, error: %s", ctx.UserID, err)
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		ctx.UnAuthorized = true
		return
	}

	ctx.Resp, ctx.RespErr = service.UpgradeAgent(c.Param("vmid"), ctx.UserName, ctx.Logger)
}

func ListVMs(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Logger.Errorf("failed to generate authorization info for user: %s, error: %s", ctx.UserID, err)
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	ctx.Resp, ctx.RespErr = service.ListVMs(ctx.Logger)
}

func ListVMLabels(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Logger.Errorf("failed to generate authorization info for user: %s, error: %s", ctx.UserID, err)
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	ctx.Resp, ctx.RespErr = service.ListVMLabels(c.Query("projectName"), ctx.Logger)
}

func RegisterAgent(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(service.RegisterAgentRequest)
	if err := c.ShouldBindJSON(args); err != nil {
		ctx.RespErr = fmt.Errorf("invalid request: %s", err)
		return
	}

	ctx.Resp, ctx.RespErr = service.RegisterAgent(args, ctx.Logger)
}

func VerifyAgent(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(service.VerifyAgentRequest)
	if err := c.ShouldBindJSON(args); err != nil {
		ctx.RespErr = fmt.Errorf("invalid request: %s", err)
		return
	}

	ctx.Resp, ctx.RespErr = service.VerifyAgent(args, ctx.Logger)
}

func HeartbeatAgent(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(service.HeartbeatRequest)
	if err := c.ShouldBindJSON(args); err != nil {
		ctx.RespErr = fmt.Errorf("invalid request: %s", err)
		return
	}

	ctx.Resp, ctx.RespErr = service.Heartbeat(args, ctx.Logger)
}

func PollingAgentJob(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	token := c.Query("token")
	if token == "" {
		ctx.RespErr = fmt.Errorf("invalid request: %s", "token is empty")
		return
	}

	ctx.Resp, ctx.RespErr = service.PollingAgentJob(token, 0, ctx.Logger)
}

func ReportAgentJob(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(service.ReportJobArgs)
	if err := c.ShouldBindJSON(args); err != nil {
		ctx.RespErr = fmt.Errorf("invalid request: %s", err)
		return
	}

	ctx.Resp, ctx.RespErr = service.ReportAgentJob(args, ctx.Logger)
}

func DownloadTemporaryFile(c *gin.Context) {
	ctx := internalhandler.NewContext(c)

	fileID := c.Param("fileId")
	if fileID == "" {
		ctx.RespErr = fmt.Errorf("invalid request: %s", "fileId is empty")
		internalhandler.JSONResponse(c, ctx)
		return
	}

	args := new(service.DownloadFileArgs)
	if err := c.ShouldBindJSON(args); err != nil {
		ctx.RespErr = fmt.Errorf("invalid request: %s", err)
		internalhandler.JSONResponse(c, ctx)
		return
	}

	// Handle download with token validation
	if err := service.DownloadTemporaryFile(fileID, args.Token, c, ctx.Logger); err != nil {
		ctx.RespErr = err
		internalhandler.JSONResponse(c, ctx)
		return
	}
}
