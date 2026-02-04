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
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"

	configbase "github.com/koderover/zadig/v2/pkg/config"
	"github.com/koderover/zadig/v2/pkg/microservice/user/core/service/login"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
)

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

func SsoTokenCallback(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ssoToken := c.Query("token")
	token, err := login.SsoTokenCallback(ssoToken, ctx.Logger)
	if err != nil {
		ctx.RespErr = err
		return
	}

	v := url.Values{}
	v.Add("token", token)
	redirectUrl := configbase.SystemAddress() + "/signin?" + v.Encode()
	c.Redirect(http.StatusSeeOther, redirectUrl)
}

type getCaptchaResp struct {
	ID      string `json:"id"`
	Content string `json:"content"`
}

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
