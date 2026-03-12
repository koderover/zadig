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

	"github.com/koderover/zadig/v2/pkg/microservice/user/core/service/login"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
)

// @Summary 本地登录
// @Description 本地账号密码登录，若命中 MFA 策略则返回 mfa_required、required_action 和 mfa_challenge_token，由前端继续完成 MFA 流程
// @Tags user
// @Accept json
// @Produce json
// @Param body body login.LoginArgs true "body"
// @Success 200 {object} login.User
// @Router /api/v1/login [post]
func LocalLogin(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	args := &login.LoginArgs{}
	if err := c.ShouldBindJSON(args); err != nil {
		ctx.RespErr = err
		return
	}
	resp, failedCount, err := login.LocalLogin(args, ctx.Logger)
	if failedCount >= 5 {
		c.Header("x-require-captcha", "true")
	} else {
		c.Header("x-require-captcha", "false")
	}
	ctx.Resp, ctx.RespErr = resp, err
}

type getCaptchaResp struct {
	ID      string `json:"id"`
	Content string `json:"content"`
}

// @Summary 获取登录验证码
// @Description 当登录失败次数达到阈值后，前端可调用该接口获取验证码
// @Tags user
// @Produce json
// @Success 200 {object} getCaptchaResp
// @Router /api/v1/captcha [get]
func GetCaptcha(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	id, picBase64, err := login.GetCaptcha(ctx.Logger)
	if err != nil {
		ctx.RespErr = err
		return
	}
	ctx.Resp = &getCaptchaResp{
		ID:      id,
		Content: picBase64,
	}
}

type LocalLogoutResp struct {
	EnableRedirect bool   `json:"enable_redirect"`
	RedirectURL    string `json:"redirect_url"`
}

// @Summary 退出登录
// @Description 清理当前用户登录态；若为第三方登录，可返回额外登出跳转地址
// @Tags user
// @Produce json
// @Success 200 {object} LocalLogoutResp
// @Router /api/v1/logout [get]
func LocalLogout(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	shouldRedirect, redirectURL, _ := login.LocalLogout(ctx.UserID, ctx.Logger)
	// TODO: for now only oauth2 service need to actually logout, so we just do nothing when an error happen
	// this need to be fixed when there are more logout logic.
	ctx.Resp = &LocalLogoutResp{
		EnableRedirect: shouldRedirect,
		RedirectURL:    redirectURL,
	}
}
