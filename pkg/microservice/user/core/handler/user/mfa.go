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

	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	loginsvc "github.com/koderover/zadig/v2/pkg/microservice/user/core/service/login"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

// @Summary 查询用户 MFA 状态
// @Description 查询指定用户当前是否已启用 MFA；系统管理员可查询任意用户，普通用户仅可查询自己
// @Tags user
// @Produce json
// @Param uid path string true "用户ID"
// @Success 200 {object} loginsvc.UserMFAStatus
// @Router /api/v1/users/{uid}/mfa/status [get]
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

// @Summary 管理员重置用户 MFA
// @Description 删除指定用户的 MFA 配置，用户后续登录需按当前策略重新完成 enroll 或 verify
// @Tags user
// @Produce json
// @Param uid path string true "用户ID"
// @Success 200
// @Router /api/v1/users/{uid}/mfa [delete]
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

	err = commonutil.CheckZadigProfessionalLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	ctx.RespErr = loginsvc.ResetUserMFA(c.Param("uid"), ctx.Logger)
}

// @Summary 初始化用户自助 MFA 配置
// @Description 为当前用户生成自助启用 MFA 所需的 secret、二维码和挑战信息
// @Tags user
// @Produce json
// @Param uid path string true "用户ID"
// @Success 200 {object} loginsvc.MFASetupResp
// @Router /api/v1/users/{uid}/mfa/setup [post]
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

// @Summary 用户自助启用 MFA
// @Description 当前用户提交 OTP 完成自助 MFA 绑定，成功后返回新的登录态和恢复码
// @Tags user
// @Accept json
// @Produce json
// @Param uid path string true "用户ID"
// @Param body body loginsvc.MFAEnrollArgs true "body"
// @Success 200 {object} loginsvc.User
// @Router /api/v1/users/{uid}/mfa/enable [post]
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

	err = commonutil.CheckZadigProfessionalLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	args := &loginsvc.MFAEnrollArgs{}
	if err := c.ShouldBindJSON(args); err != nil {
		ctx.RespErr = err
		return
	}
	ctx.Resp, ctx.RespErr = loginsvc.EnableUserMFA(uid, args, ctx.Logger)
}

// @Summary 用户自助关闭 MFA
// @Description 当前用户提交 OTP 或恢复码关闭 MFA；若系统启用了强制 MFA，该接口会被拒绝
// @Tags user
// @Accept json
// @Produce json
// @Param uid path string true "用户ID"
// @Param body body loginsvc.MFADisableArgs true "body"
// @Success 200
// @Router /api/v1/users/{uid}/mfa/disable [post]
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

// @Summary 重新生成恢复码
// @Description 当前用户提交 OTP 或恢复码后，重新生成并返回一套新的 recovery codes
// @Tags user
// @Accept json
// @Produce json
// @Param uid path string true "用户ID"
// @Param body body loginsvc.MFARecoveryCodesArgs true "body"
// @Success 200 {object} loginsvc.MFARecoveryCodesResp
// @Router /api/v1/users/{uid}/mfa/recovery-codes [post]
func RegenerateRecoveryCodes(c *gin.Context) {
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

	args := &loginsvc.MFARecoveryCodesArgs{}
	if err := c.ShouldBindJSON(args); err != nil {
		ctx.RespErr = err
		return
	}
	ctx.Resp, ctx.RespErr = loginsvc.RegenerateRecoveryCodes(uid, args, ctx.Logger)
}
