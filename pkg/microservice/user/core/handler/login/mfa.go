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

// @Summary 初始化登录态 MFA 配置
// @Description 基于 mfa_challenge_token 生成当前登录挑战所需的 MFA 配置数据，返回 secret、二维码和 required_action
// @Tags user
// @Accept json
// @Produce json
// @Param body body loginsvc.MFASetupArgs true "body"
// @Success 200 {object} loginsvc.MFASetupResp
// @Router /api/v1/login/mfa/setup [post]
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

// @Summary 完成登录态 MFA 绑定
// @Description 在登录挑战阶段提交 OTP 完成首次 MFA 绑定，成功后返回正式登录态和恢复码
// @Tags user
// @Accept json
// @Produce json
// @Param body body loginsvc.MFAEnrollArgs true "body"
// @Success 200 {object} loginsvc.User
// @Router /api/v1/login/mfa/enroll [post]
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

// @Summary 完成登录态 MFA 验证
// @Description 在登录挑战阶段提交 OTP 或恢复码完成 MFA 验证，成功后返回正式登录态
// @Tags user
// @Accept json
// @Produce json
// @Param body body loginsvc.MFAVerifyArgs true "body"
// @Success 200 {object} loginsvc.User
// @Router /api/v1/login/mfa/verify [post]
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
