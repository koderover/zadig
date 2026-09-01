/*
Copyright 2026 The KodeRover Authors.

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

package oauth

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	configbase "github.com/koderover/zadig/v2/pkg/config"
	"github.com/koderover/zadig/v2/pkg/microservice/user/core/service/login"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
)

const (
	deviceCodeGrantType = "urn:ietf:params:oauth:grant-type:device_code"
	refreshTokenGrant   = "refresh_token"
)

type approvalArgs struct {
	Decision string `json:"decision"`
}

func DeviceAuthorization(c *gin.Context) {
	args := new(login.OAuthDeviceAuthorizationArgs)
	if err := c.ShouldBind(args); err != nil {
		writeOAuthError(c, &login.OAuthError{Code: login.OAuthErrorInvalidRequest, Description: err.Error()})
		return
	}
	verificationURI := strings.TrimRight(configbase.SystemAddress(), "/") + "/oauth/device"
	response, err := login.NewOAuthService().CreateDeviceAuthorization(args, verificationURI)
	if err != nil {
		writeOAuthError(c, err)
		return
	}
	writeOAuthJSON(c, http.StatusOK, response)
}

func GetDeviceAuthorization(c *gin.Context) {
	response, err := login.NewOAuthService().GetDeviceAuthorization(c.Param("userCode"))
	if err != nil {
		writeBrowserError(c, err)
		return
	}
	c.JSON(http.StatusOK, response)
}

func DecideDeviceAuthorization(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	if ctx.UserID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized", "message": "authenticated user is required"})
		return
	}
	args := new(approvalArgs)
	if err := c.ShouldBindJSON(args); err != nil {
		writeBrowserError(c, &login.OAuthError{Code: login.OAuthErrorInvalidRequest, Description: err.Error()})
		return
	}
	err := login.NewOAuthService().DecideDeviceAuthorization(c.Param("userCode"), args.Decision, login.OAuthUser{
		UID:          ctx.UserID,
		Name:         ctx.UserName,
		Account:      ctx.Account,
		IdentityType: ctx.IdentityType,
		MFAVerified:  ctx.MFAVerified,
	})
	if err != nil {
		writeBrowserError(c, err)
		return
	}
	status := login.OAuthDeviceStatusDenied
	if args.Decision == login.OAuthDecisionApprove {
		status = login.OAuthDeviceStatusApproved
	}
	c.JSON(http.StatusOK, gin.H{"status": status})
}

func Token(c *gin.Context) {
	var (
		response *login.OAuthTokenResponse
		err      error
	)
	switch c.PostForm("grant_type") {
	case deviceCodeGrantType:
		response, err = login.NewOAuthService().ExchangeDeviceCode(c.PostForm("client_id"), c.PostForm("device_code"))
	case refreshTokenGrant:
		response, err = login.NewOAuthService().RefreshToken(c.PostForm("client_id"), c.PostForm("refresh_token"))
	default:
		err = &login.OAuthError{Code: "unsupported_grant_type", Description: "unsupported grant_type"}
	}
	if err != nil {
		writeOAuthError(c, err)
		return
	}
	writeOAuthJSON(c, http.StatusOK, response)
}

func Revoke(c *gin.Context) {
	if err := login.NewOAuthService().RevokeToken(c.PostForm("client_id"), c.PostForm("token")); err != nil {
		writeOAuthError(c, err)
		return
	}
	c.Data(http.StatusOK, "application/json", nil)
}

func writeOAuthError(c *gin.Context, err error) {
	status := http.StatusInternalServerError
	response := &login.OAuthError{Code: "server_error", Description: "oauth request failed"}
	var oauthErr *login.OAuthError
	if errors.As(err, &oauthErr) {
		response = oauthErr
		status = http.StatusBadRequest
		if oauthErr.Code == login.OAuthErrorInvalidClient {
			status = http.StatusUnauthorized
		}
	}
	writeOAuthJSON(c, status, response)
}

func writeBrowserError(c *gin.Context, err error) {
	if errors.Is(err, login.ErrOAuthAuthorizationNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "not_found", "message": "device authorization not found"})
		return
	}
	var oauthErr *login.OAuthError
	if errors.As(err, &oauthErr) {
		c.JSON(http.StatusBadRequest, oauthErr)
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": "server_error", "message": "oauth request failed"})
}

func writeOAuthJSON(c *gin.Context, status int, value interface{}) {
	c.Header("Cache-Control", "no-store")
	c.Header("Pragma", "no-cache")
	c.JSON(status, value)
}
