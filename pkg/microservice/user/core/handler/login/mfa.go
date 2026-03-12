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

package login

import (
	"github.com/gin-gonic/gin"

	loginsvc "github.com/koderover/zadig/v2/pkg/microservice/user/core/service/login"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
)

func MFASetup(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := &loginsvc.MFASetupArgs{}
	if err := c.ShouldBindJSON(args); err != nil {
		ctx.RespErr = err
		return
	}
	ctx.Resp, ctx.RespErr = loginsvc.SetupMFA(args, ctx.Logger)
}

func MFAEnroll(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := &loginsvc.MFAEnrollArgs{}
	if err := c.ShouldBindJSON(args); err != nil {
		ctx.RespErr = err
		return
	}
	ctx.Resp, ctx.RespErr = loginsvc.EnrollMFA(args, ctx.Logger)
}

func MFAVerify(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := &loginsvc.MFAVerifyArgs{}
	if err := c.ShouldBindJSON(args); err != nil {
		ctx.RespErr = err
		return
	}
	ctx.Resp, ctx.RespErr = loginsvc.VerifyMFA(args, ctx.Logger)
}
