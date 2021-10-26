package login

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	"github.com/koderover/zadig/pkg/microservice/user/config"
	"github.com/koderover/zadig/pkg/microservice/user/core/service/login"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/util"
	"golang.org/x/oauth2"
	"net/http"
	"net/url"
	"time"
)

func Login(c *gin.Context) {
	oauth2Config := &oauth2.Config{
		ClientID:     config.ClientID(),
		ClientSecret: config.ClientSecret(),
		Endpoint:     config.Provider().Endpoint(),
		Scopes:       config.Scopes(),
		RedirectURL:  config.RedirectURI(),
	}
	authCodeURL := oauth2Config.AuthCodeURL(config.AppState, oauth2.AccessTypeOffline)
	c.Redirect(http.StatusSeeOther, authCodeURL)
}

func Callback(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	oidcCtx := oidc.ClientContext(c.Request.Context(), config.Client())
	oauth2Config := &oauth2.Config{
		ClientID:     config.ClientID(),
		ClientSecret: config.ClientSecret(),
		Endpoint:     config.Provider().Endpoint(),
		Scopes:       nil,
		RedirectURL:  config.RedirectURI(),
	}
	var err error
	var token *oauth2.Token
	switch c.Request.Method {
	case http.MethodGet:
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
		token, err = oauth2Config.Exchange(oidcCtx, code)
	case http.MethodPost:
		// Form request from frontend to refresh a token.
		refresh := c.Query("refresh_token")
		if refresh == "" {
			ctx.Err = e.ErrCallBackUser.AddDesc(fmt.Sprintf("no refresh_token in request: %q", c.Request.Form))
			return
		}
		t := &oauth2.Token{
			RefreshToken: refresh,
			Expiry:       time.Now().Add(-time.Hour),
		}
		token, err = oauth2Config.TokenSource(oidcCtx, t).Token()
	default:
		ctx.Err = e.ErrCallBackUser.AddDesc(fmt.Sprintf("method not implemented: %s", c.Request.Method))
		return
	}

	if err != nil {
		ctx.Err = e.ErrCallBackUser.AddDesc(fmt.Sprintf("failed to get token: %v", err))
		return
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		ctx.Err = e.ErrCallBackUser.AddDesc("no id_token in token response")
		return
	}

	idToken, err := config.Verifier().Verify(c.Request.Context(), rawIDToken)
	if err != nil {
		ctx.Err = e.ErrCallBackUser.AddDesc(fmt.Sprintf("failed to verify ID token: %v", err))
		return
	}

	var claimsRaw json.RawMessage
	if err := idToken.Claims(&claimsRaw); err != nil {
		ctx.Err = e.ErrCallBackUser.AddDesc(fmt.Sprintf("error decoding ID token claims: %v", err))
		return
	}
	buff := new(bytes.Buffer)
	if err := json.Indent(buff, claimsRaw, "", "  "); err != nil {
		ctx.Err = e.ErrCallBackUser.AddDesc(fmt.Sprintf("error indenting ID token claims: %v", err))
		return
	}

	var claims util.Claims
	err = json.Unmarshal(claimsRaw, &claims)
	if err != nil {
		ctx.Err = err
		return
	}
	user, err := login.SyncUser(&login.SyncUserInfo{
		Email:        claims.Email,
		Name:         claims.Name,
		IdentityType: claims.FederatedClaims.ConnectorId,
	}, ctx.Logger)
	if err != nil {
		ctx.Err = err
		return
	}
	claims.Uid = user.Uid
	claims.StandardClaims.ExpiresAt = time.Now().Add(24 * time.Hour).Unix()
	userToken, err := util.CreateToken(&claims)
	if err != nil {
		ctx.Err = err
		return
	}
	v := url.Values{}
	v.Add("token", userToken)
	redirectUrl := "../..?" + v.Encode()
	c.Redirect(http.StatusSeeOther, redirectUrl)
}
