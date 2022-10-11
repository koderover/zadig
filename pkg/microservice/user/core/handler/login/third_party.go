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
	"net/url"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"

	configbase "github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/user/config"
	"github.com/koderover/zadig/pkg/microservice/user/core/service/login"
	"github.com/koderover/zadig/pkg/microservice/user/core/service/user"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/shared/client/aslan"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/log"
)

func provider() *oidc.Provider {
	ctx := oidc.ClientContext(context.Background(), http.DefaultClient)
	provider, err := oidc.NewProvider(ctx, config.IssuerURL())
	if err != nil {
		log.Panicf(fmt.Sprintf("init provider error:%s", err))
	}
	return provider
}

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

	authCodeURL := oauth2Config.AuthCodeURL(config.AppState, oauth2.AccessTypeOffline)
	systemConfig, err := aslan.New(configbase.AslanServiceAddress()).GetDefaultLogin()
	if err != nil {
		ctx.Err = err
		return
	}
	defaultLogin := ""
	replaceURL := configbase.SystemAddress() + "/dex/auth"
	if systemConfig.DefaultLogin != setting.DefaultLoginLocal {
		defaultLogin = systemConfig.DefaultLogin
		replaceURL = replaceURL + "/" + defaultLogin
	}
	authCodeURL = strings.Replace(authCodeURL, config.IssuerURL()+"/auth", replaceURL, -1)

	c.Redirect(http.StatusSeeOther, authCodeURL)
}

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

func Callback(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	// Authorization redirect callback from OAuth2 auth flow.
	if errMsg := c.Query("error"); errMsg != "" {
		ctx.Err = e.ErrCallBackUser.AddDesc(errMsg)
		return
	}
	code := c.Query("code")
	if code == "" {
		ctx.Err = e.ErrCallBackUser.AddDesc(fmt.Sprintf("no code in request: %q", c.Request.Form))
		return
	}
	if state := c.Query("state"); state != config.AppState {
		ctx.Err = e.ErrCallBackUser.AddDesc(fmt.Sprintf("expected state %q got %q", config.AppState, state))
		return
	}
	claims, err := verifyAndDecode(c.Request.Context(), code)
	if err != nil {
		ctx.Err = err
		return
	}

	user, err := user.SyncUser(&user.SyncUserInfo{
		Account:      claims.PreferredUsername,
		Name:         claims.Name,
		Email:        claims.Email,
		IdentityType: claims.FederatedClaims.ConnectorId,
	}, true, ctx.Logger)
	if err != nil {
		ctx.Err = err
		return
	}
	claims.UID = user.UID
	claims.StandardClaims.ExpiresAt = time.Now().Add(time.Duration(config.TokenExpiresAt()) * time.Minute).Unix()
	userToken, err := login.CreateToken(claims)
	if err != nil {
		ctx.Err = err
		return
	}
	v := url.Values{}
	v.Add("token", userToken)
	redirectUrl := "/?" + v.Encode()
	c.Redirect(http.StatusSeeOther, redirectUrl)
}
