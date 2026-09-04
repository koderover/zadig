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
	"errors"
	"time"

	"github.com/golang-jwt/jwt"
	"github.com/redis/go-redis/v9"

	"github.com/koderover/zadig/v2/pkg/setting"
)

func (s *OAuthService) RefreshToken(refreshToken string) (*OAuthTokenResponse, error) {
	if refreshToken == "" {
		return nil, &OAuthError{Code: OAuthErrorInvalidRequest, Description: "refresh_token is required"}
	}
	sessionID, err := s.cache.TakeString(oauthKey("refresh", hashOAuthValue(refreshToken)))
	if errors.Is(err, redis.Nil) {
		return nil, &OAuthError{Code: oauthErrorInvalidGrant, Description: "refresh token is invalid or expired"}
	}
	if err != nil {
		return nil, err
	}
	session, err := s.session(sessionID)
	if err != nil {
		if errors.Is(err, errOAuthInvalidSession) {
			return nil, &OAuthError{Code: oauthErrorInvalidGrant, Description: "refresh token is invalid or expired"}
		}
		return nil, err
	}
	newRefreshToken, err := randomOAuthValue(32)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	accessToken, accessTTL, err := createOAuthAccessToken(session, now)
	if err != nil {
		return nil, err
	}
	refreshTTL := oauthRefreshTokenTTL
	if remaining := session.ExpiresAt.Sub(now); remaining < refreshTTL {
		refreshTTL = remaining
	}
	if err := s.cache.Write(oauthKey("refresh", hashOAuthValue(newRefreshToken)), session.ID, refreshTTL); err != nil {
		return nil, err
	}
	return &OAuthTokenResponse{
		AccessToken: accessToken, TokenType: "Bearer", ExpiresIn: int64(accessTTL / time.Second),
		RefreshToken: newRefreshToken, Scope: OAuthLocalClientScope,
	}, nil
}

func (s *OAuthService) RevokeToken(refreshToken string) error {
	if refreshToken == "" {
		return &OAuthError{Code: OAuthErrorInvalidRequest, Description: "token is required"}
	}
	refreshTokenKey := oauthKey("refresh", hashOAuthValue(refreshToken))
	sessionID, err := s.cache.GetString(refreshTokenKey)
	if errors.Is(err, redis.Nil) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := s.deleteSession(sessionID); err != nil {
		return err
	}
	return s.cache.Delete(refreshTokenKey)
}

func (s *OAuthService) ValidateSession(sessionID, uid string) error {
	session, err := s.session(sessionID)
	if err != nil {
		return err
	}
	if session.User.UID != uid {
		return errOAuthInvalidSession
	}
	return nil
}

func (s *OAuthService) RevokeUserSessions(uid string) error {
	key := oauthKey("user-sessions", uid)
	sessionIDs, err := s.cache.ListSetMembers(key)
	if err != nil {
		return err
	}
	for _, sessionID := range sessionIDs {
		if err := s.deleteSession(sessionID); err != nil {
			return err
		}
	}
	return s.cache.Delete(key)
}

func (s *OAuthService) createOAuthSession(device *oauthDevice) (*OAuthTokenResponse, error) {
	if device.User == nil {
		return nil, &OAuthError{Code: oauthErrorInvalidGrant, Description: "authorization has no user"}
	}
	sessionID, err := randomOAuthValue(24)
	if err != nil {
		return nil, err
	}
	refreshToken, err := randomOAuthValue(32)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	session := &oauthSession{
		ID:        sessionID,
		User:      *device.User,
		ExpiresAt: now.Add(oauthSessionTTL),
	}
	accessToken, accessTTL, err := createOAuthAccessToken(session, now)
	if err != nil {
		return nil, err
	}
	if err := s.writeJSON(oauthKey("session", session.ID), session, time.Until(session.ExpiresAt)); err != nil {
		return nil, err
	}
	refreshTokenKey := oauthKey("refresh", hashOAuthValue(refreshToken))
	if err := s.cache.Write(refreshTokenKey, session.ID, oauthRefreshTokenTTL); err != nil {
		_ = s.cache.Delete(oauthKey("session", session.ID))
		return nil, err
	}
	if err := s.cache.AddElementsToSet(oauthKey("user-sessions", session.User.UID), []string{session.ID}, oauthSessionTTL); err != nil {
		_ = s.cache.Delete(refreshTokenKey)
		_ = s.cache.Delete(oauthKey("session", session.ID))
		return nil, err
	}
	return &OAuthTokenResponse{
		AccessToken: accessToken, TokenType: "Bearer", ExpiresIn: int64(accessTTL / time.Second),
		RefreshToken: refreshToken, Scope: OAuthLocalClientScope,
	}, nil
}

func (s *OAuthService) session(sessionID string) (*oauthSession, error) {
	session := new(oauthSession)
	if err := s.readJSON(oauthKey("session", sessionID), session); err != nil {
		if errors.Is(err, ErrOAuthAuthorizationNotFound) {
			return nil, errOAuthInvalidSession
		}
		return nil, err
	}
	if !time.Now().Before(session.ExpiresAt) {
		return nil, errOAuthInvalidSession
	}
	return session, nil
}

func (s *OAuthService) deleteSession(sessionID string) error {
	session, err := s.session(sessionID)
	if errors.Is(err, errOAuthInvalidSession) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := s.cache.Delete(oauthKey("session", sessionID)); err != nil {
		return err
	}
	return s.cache.RemoveElementsFromSet(oauthKey("user-sessions", session.User.UID), []string{sessionID})
}

func createOAuthAccessToken(session *oauthSession, now time.Time) (string, time.Duration, error) {
	accessTTL := oauthAccessTokenTTL
	if remaining := session.ExpiresAt.Sub(now); remaining < accessTTL {
		accessTTL = remaining
	}
	token, err := CreateToken(&Claims{
		Name:              session.User.Name,
		UID:               session.User.UID,
		PreferredUsername: session.User.Account,
		MFAVerified:       session.User.MFAVerified,
		TokenUse:          OAuthLocalTokenUseAccess,
		ClientID:          OAuthLocalClientID,
		SessionID:         session.ID,
		FederatedClaims:   FederatedClaims{ConnectorId: session.User.IdentityType, UserId: session.User.Account},
		StandardClaims: jwt.StandardClaims{
			Audience: setting.ProductName, IssuedAt: now.Unix(), ExpiresAt: now.Add(accessTTL).Unix(),
		},
	})
	return token, accessTTL, err
}
