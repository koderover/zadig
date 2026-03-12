/*
Copyright 2021 The KodeRover Authors.

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

package user

import (
	"fmt"

	"github.com/gin-gonic/gin"

	loginsvc "github.com/koderover/zadig/v2/pkg/microservice/user/core/service/login"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

func GetUserMFAStatus(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	if err != nil {
		ctx.UnAuthorized = true
		ctx.RespErr = fmt.Errorf("authorization info generation failed: %s", err)
		return
	}

	uid := c.Param("uid")
	if uid == "" {
		ctx.RespErr = fmt.Errorf("empty uid")
		return
	}

	if !ctx.Resources.IsSystemAdmin && ctx.UserID != uid {
		ctx.RespErr = e.ErrForbidden
		return
	}

	ctx.Resp, ctx.RespErr = loginsvc.GetUserMFAStatus(uid, ctx.Logger)
}

func ResetUserMFAByAdmin(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	if err != nil {
		ctx.UnAuthorized = true
		ctx.RespErr = fmt.Errorf("authorization info generation failed: %s", err)
		return
	}
	if !ctx.Resources.IsSystemAdmin {
		ctx.RespErr = e.ErrForbidden
		return
	}

	ctx.RespErr = loginsvc.ResetUserMFA(c.Param("uid"), ctx.Logger)
}

func SetupUserMFA(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	if err != nil {
		ctx.UnAuthorized = true
		ctx.RespErr = fmt.Errorf("authorization info generation failed: %s", err)
		return
	}

	uid := c.Param("uid")
	if uid == "" {
		ctx.RespErr = fmt.Errorf("empty uid")
		return
	}
	if ctx.UserID != uid {
		ctx.RespErr = e.ErrForbidden
		return
	}

	ctx.Resp, ctx.RespErr = loginsvc.SetupUserMFA(uid, ctx.Logger)
}

func EnableUserMFA(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	if err != nil {
		ctx.UnAuthorized = true
		ctx.RespErr = fmt.Errorf("authorization info generation failed: %s", err)
		return
	}

	uid := c.Param("uid")
	if uid == "" {
		ctx.RespErr = fmt.Errorf("empty uid")
		return
	}
	if ctx.UserID != uid {
		ctx.RespErr = e.ErrForbidden
		return
	}

	args := &loginsvc.MFAEnrollArgs{}
	if err := c.ShouldBindJSON(args); err != nil {
		ctx.RespErr = err
		return
	}
	ctx.Resp, ctx.RespErr = loginsvc.EnableUserMFA(uid, args, ctx.Logger)
}

func DisableUserMFA(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	if err != nil {
		ctx.UnAuthorized = true
		ctx.RespErr = fmt.Errorf("authorization info generation failed: %s", err)
		return
	}

	uid := c.Param("uid")
	if uid == "" {
		ctx.RespErr = fmt.Errorf("empty uid")
		return
	}
	if ctx.UserID != uid {
		ctx.RespErr = e.ErrForbidden
		return
	}

	args := &loginsvc.MFADisableArgs{}
	if err := c.ShouldBindJSON(args); err != nil {
		ctx.RespErr = err
		return
	}
	ctx.RespErr = loginsvc.DisableUserMFA(uid, args, ctx.Logger)
}
