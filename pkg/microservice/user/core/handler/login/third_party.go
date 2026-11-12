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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	"github.com/koderover/zadig/v2/pkg/microservice/user/core/service/permission"
	"golang.org/x/oauth2"

	configbase "github.com/koderover/zadig/v2/pkg/config"
	"github.com/koderover/zadig/v2/pkg/microservice/user/config"
	"github.com/koderover/zadig/v2/pkg/microservice/user/core/service/login"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/shared/client/aslan"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

func provider() *oidc.Provider {
	ctx := oidc.ClientContext(context.Background(), http.DefaultClient)
	provider, err := oidc.NewProvider(ctx, config.IssuerURL())
	if err != nil {
		log.Panicf(fmt.Sprintf("init provider error:%s", err))
	}
	return provider
}

func thirdPartyLoginCookie() *http.Cookie {
	secure := strings.HasPrefix(strings.ToLower(configbase.SystemAddress()), "https://")
	name := "zadig_login_state"
	if secure {
		name = "__Host-" + name
	}
	return &http.Cookie{
		Name:     name,
		Path:     "/",
		MaxAge:   int(login.ThirdPartyLoginStateTTL / time.Second),
		Secure:   secure,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}
}

// @Summary 发起第三方登录
// @Description 跳转到 Dex/OIDC 登录入口
// @Tags user
// @Success 303
// @Router /api/v1/login [get]
func Login(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	oauth2Config := &oauth2.Config{
		ClientID:     config.ClientID(),
		ClientSecret: config.ClientSecret(),
		Endpoint:     provider().Endpoint(),
		Scopes:       config.Scopes(),
		RedirectURL:  config.RedirectURI(),
	}

	systemConfig, err := aslan.New(configbase.AslanServiceAddress()).GetDefaultLogin()
	if err != nil {
		ctx.RespErr = err
		return
	}
	state, browserToken, err := login.CreateThirdPartyLoginState(c.Query("redirect"))
	if err != nil {
		ctx.RespErr = err
		return
	}
	authCodeURL := oauth2Config.AuthCodeURL(state, oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("redirect", c.Query("redirect")))
	defaultLogin := ""
	replaceURL := configbase.SystemAddress() + "/dex/auth"
	if systemConfig.DefaultLogin != setting.DefaultLoginLocal {
		defaultLogin = systemConfig.DefaultLogin
		replaceURL = replaceURL + "/" + defaultLogin
	}
	authCodeURL = strings.Replace(authCodeURL, config.IssuerURL()+"/auth", replaceURL, -1)

	cookie := thirdPartyLoginCookie()
	cookie.Value = browserToken
	http.SetCookie(c.Writer, cookie)
	c.Redirect(http.StatusSeeOther, authCodeURL)
}

type completeLocalLoginArgs struct {
	State string `json:"state"`
}

// CompleteLocalLogin restores the saved redirect after a local password or MFA login.
func CompleteLocalLogin(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	authorization := strings.Fields(c.GetHeader(setting.AuthorizationHeader))
	if len(authorization) != 2 || !strings.EqualFold(authorization[0], "Bearer") {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	claims, valid, err := permission.ValidateWebSessionToken(authorization[1])
	if err != nil || !valid || claims == nil || claims.UID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	args := new(completeLocalLoginArgs)
	if err := c.ShouldBindJSON(args); err != nil || strings.TrimSpace(args.State) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "state is required"})
		return
	}

	cookie := thirdPartyLoginCookie()
	browserToken, err := c.Cookie(cookie.Name)
	if err != nil || browserToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "login browser cookie is required"})
		return
	}
	redirectPath, err := login.ConsumeThirdPartyLoginState(args.State, browserToken)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid or expired login state"})
		return
	}
	cookie.MaxAge = -1
	http.SetCookie(c.Writer, cookie)
	c.JSON(http.StatusOK, gin.H{"redirect": redirectPath})
}

// @Summary 查询第三方登录是否开启
// @Description 返回系统当前是否配置了可用的第三方登录连接器
// @Tags user
// @Produce json
// @Success 200 {object} login.enabledStatus
// @Router /api/v1/login-enabled [get]
func ThirdPartyLoginEnabled(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp = login.ThirdPartyLoginEnabled()
}

func verifyAndDecode(ctx context.Context, code string) (*login.Claims, error) {
	oidcCtx := oidc.ClientContext(ctx, http.DefaultClient)
	oauth2Config := &oauth2.Config{
		ClientID:     config.ClientID(),
		ClientSecret: config.ClientSecret(),
		Endpoint:     provider().Endpoint(),
		Scopes:       nil,
		RedirectURL:  config.RedirectURI(),
	}
	var token *oauth2.Token
	token, err := oauth2Config.Exchange(oidcCtx, code)
	if err != nil {
		return nil, e.ErrCallBackUser.AddDesc(fmt.Sprintf("failed to get token: %v", err))
	}
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return nil, e.ErrCallBackUser.AddDesc("no id_token in token response")
	}
	idToken, err := provider().Verifier(&oidc.Config{ClientID: config.ClientID()}).Verify(ctx, rawIDToken)
	if err != nil {
		return nil, e.ErrCallBackUser.AddDesc(fmt.Sprintf("failed to verify ID token: %v", err))
	}
	var claimsRaw json.RawMessage
	if err := idToken.Claims(&claimsRaw); err != nil {
		return nil, e.ErrCallBackUser.AddDesc(fmt.Sprintf("error decoding ID token claims: %v", err))
	}
	buff := new(bytes.Buffer)
	if err := json.Indent(buff, claimsRaw, "", "  "); err != nil {
		return nil, e.ErrCallBackUser.AddDesc(fmt.Sprintf("error indenting ID token claims: %v", err))
	}
	var claims login.Claims
	err = json.Unmarshal(claimsRaw, &claims)
	if err != nil {
		return nil, err
	}
	if len(claims.Name) == 0 {
		claims.Name = claims.PreferredUsername
	}
	return &claims, nil
}

// @Summary 第三方登录回调
// @Description 处理 Dex/OIDC 回调并重定向到前端页面；若命中 MFA 策略，将重定向到 MFA 继续页
// @Tags user
// @Success 303
// @Router /api/v1/callback [get]
func Callback(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	cookie := thirdPartyLoginCookie()
	browserToken, err := c.Cookie(cookie.Name)
	if err != nil {
		ctx.RespErr = fmt.Errorf("missing login browser cookie")
		return
	}
	// Atomically validate and consume state before exchanging the authorization code.
	redirectPath, err := login.ConsumeThirdPartyLoginState(c.Query("state"), browserToken)
	if err != nil {
		ctx.RespErr = err
		return
	}
	cookie.MaxAge = -1
	http.SetCookie(c.Writer, cookie)

	// Authorization redirect callback from OAuth2 auth flow.
	if errMsg := c.Query("error"); errMsg != "" {
		ctx.RespErr = e.ErrCallBackUser.AddDesc(errMsg)
		return
	}
	code := c.Query("code")
	if code == "" {
		ctx.RespErr = e.ErrCallBackUser.AddDesc(fmt.Sprintf("no code in request: %q", c.Request.Form))
		return
	}
	claims, err := verifyAndDecode(c.Request.Context(), code)
	if err != nil {
		ctx.RespErr = err
		return
	}

	user, err := permission.SyncUser(&permission.SyncUserInfo{
		Account:      claims.PreferredUsername,
		Name:         claims.Name,
		Email:        claims.Email,
		Phone:        claims.Phone,
		IdentityType: claims.FederatedClaims.ConnectorId,
	}, false, ctx.Logger)
	if err != nil {
		ctx.RespErr = err
		return
	}

	redirectURL, err := login.HandleThirdPartyLoginSuccess(user, redirectPath, ctx.Logger)
	if err != nil {
		ctx.RespErr = err
		return
	}
	c.Redirect(http.StatusSeeOther, redirectURL)
}
