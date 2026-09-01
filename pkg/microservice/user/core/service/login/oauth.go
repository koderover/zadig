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

package login

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
	"time"

	userconfig "github.com/koderover/zadig/v2/pkg/microservice/user/config"
	"github.com/koderover/zadig/v2/pkg/tool/cache"
)

const (
	OAuthCLIClientID          = "zadig-cli"
	OAuthCLIScope             = "zadig.api offline_access"
	OAuthTokenUseCLIAccess    = "cli_access"
	OAuthDeviceStatusPending  = "pending"
	OAuthDeviceStatusApproved = "approved"
	OAuthDeviceStatusDenied   = "denied"
	OAuthDecisionApprove      = "approve"
	OAuthDecisionDeny         = "deny"

	OAuthErrorInvalidRequest       = "invalid_request"
	OAuthErrorInvalidClient        = "invalid_client"
	OAuthErrorInvalidGrant         = "invalid_grant"
	OAuthErrorAuthorizationPending = "authorization_pending"
	OAuthErrorSlowDown             = "slow_down"
	OAuthErrorAccessDenied         = "access_denied"
	OAuthErrorExpiredToken         = "expired_token"

	OAuthDeviceAuthorizationTTL = 10 * time.Minute
	OAuthDevicePollInterval     = 5 * time.Second
	OAuthAccessTokenTTL         = time.Hour
	OAuthRefreshTokenTTL        = 30 * 24 * time.Hour
	OAuthSessionTTL             = 180 * 24 * time.Hour

	oauthRedisKeyPrefix = "zadig:oauth:"
)

var (
	ErrOAuthAuthorizationNotFound = errors.New("oauth device authorization not found")
	ErrOAuthInvalidSession        = errors.New("oauth session is invalid")
)

type OAuthError struct {
	Code        string `json:"error"`
	Description string `json:"error_description,omitempty"`
}

func (e *OAuthError) Error() string {
	return e.Code + ": " + e.Description
}

type OAuthDeviceAuthorizationArgs struct {
	ClientID   string `form:"client_id"`
	Scope      string `form:"scope"`
	DeviceName string `form:"device_name"`
}

type OAuthDeviceAuthorizationResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int64  `json:"expires_in"`
	Interval                int64  `json:"interval"`
}

type OAuthDeviceAuthorizationInfo struct {
	ClientName string    `json:"client_name"`
	DeviceName string    `json:"device_name"`
	UserCode   string    `json:"user_code"`
	ExpiresAt  time.Time `json:"expires_at"`
	Status     string    `json:"status"`
}

type OAuthTokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
}

type OAuthUser struct {
	UID          string `json:"uid"`
	Name         string `json:"name"`
	Account      string `json:"account"`
	IdentityType string `json:"identity_type"`
	MFAVerified  bool   `json:"mfa_verified"`
}

type oauthDevice struct {
	DeviceCodeHash string     `json:"device_code_hash"`
	UserCode       string     `json:"user_code"`
	ClientID       string     `json:"client_id"`
	Scope          string     `json:"scope"`
	DeviceName     string     `json:"device_name"`
	Status         string     `json:"status"`
	User           *OAuthUser `json:"user,omitempty"`
	ExpiresAt      time.Time  `json:"expires_at"`
}

type oauthSession struct {
	ID        string    `json:"id"`
	ClientID  string    `json:"client_id"`
	Scope     string    `json:"scope"`
	User      OAuthUser `json:"user"`
	ExpiresAt time.Time `json:"expires_at"`
}

type OAuthService struct {
	cache *cache.RedisCache
}

func NewOAuthService() *OAuthService {
	return &OAuthService{cache: cache.NewRedisCache(userconfig.RedisUserTokenDB())}
}

func randomOAuthValue(size int) (string, error) {
	buffer := make([]byte, size)
	if _, err := io.ReadFull(rand.Reader, buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

func randomOAuthUserCode() (string, error) {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	random := make([]byte, 8)
	if _, err := io.ReadFull(rand.Reader, random); err != nil {
		return "", err
	}
	for i := range random {
		random[i] = alphabet[int(random[i])%len(alphabet)]
	}
	return string(random[:4]) + "-" + string(random[4:]), nil
}

func hashOAuthValue(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func oauthKey(kind, id string) string {
	return oauthRedisKeyPrefix + kind + ":" + id
}
