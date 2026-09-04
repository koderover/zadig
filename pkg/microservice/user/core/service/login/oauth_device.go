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
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	configbase "github.com/koderover/zadig/v2/pkg/config"
)

func (s *OAuthService) CreateDeviceAuthorization(deviceName string) (*OAuthDeviceAuthorizationResponse, error) {
	deviceCode, err := randomOAuthValue(32)
	if err != nil {
		return nil, err
	}
	device := &oauthDevice{
		DeviceCodeHash: hashOAuthValue(deviceCode),
		DeviceName:     deviceName,
		Status:         oauthDeviceStatusPending,
		ExpiresAt:      time.Now().UTC().Add(oauthDeviceAuthorizationTTL),
	}
	device.UserCode, err = randomOAuthUserCode()
	if err != nil {
		return nil, err
	}
	reserved, err := s.cache.WriteIfNotExists(oauthKey("user-code", device.UserCode), device.DeviceCodeHash, oauthDeviceAuthorizationTTL)
	if err != nil {
		return nil, err
	}
	if !reserved {
		return nil, fmt.Errorf("failed to allocate OAuth user code")
	}
	if err := s.writeDevice(device); err != nil {
		_ = s.cache.Delete(oauthKey("user-code", device.UserCode))
		return nil, err
	}
	verificationURI := strings.TrimRight(configbase.SystemAddress(), "/") + "/oauth/device"
	return &OAuthDeviceAuthorizationResponse{
		DeviceCode:              deviceCode,
		UserCode:                device.UserCode,
		VerificationURI:         verificationURI,
		VerificationURIComplete: verificationURI + "?code=" + url.QueryEscape(device.UserCode),
		ExpiresIn:               int64(oauthDeviceAuthorizationTTL / time.Second),
		Interval:                int64(oauthDevicePollInterval / time.Second),
	}, nil
}

func (s *OAuthService) GetDeviceAuthorization(userCode string) (*OAuthDeviceAuthorizationInfo, error) {
	device, err := s.deviceByUserCode(userCode)
	if err != nil {
		return nil, err
	}
	return &OAuthDeviceAuthorizationInfo{
		ClientName: OAuthLocalClientName,
		DeviceName: device.DeviceName,
		UserCode:   device.UserCode,
		ExpiresAt:  device.ExpiresAt,
		Status:     device.Status,
	}, nil
}

func (s *OAuthService) DecideDeviceAuthorization(userCode, decision string, user OAuthUser) (string, error) {
	if decision != oauthDecisionApprove && decision != oauthDecisionDeny {
		return "", &OAuthError{Code: OAuthErrorInvalidRequest, Description: "decision must be approve or deny"}
	}
	device, err := s.deviceByUserCode(userCode)
	if err != nil {
		return "", err
	}
	if device.Status != oauthDeviceStatusPending {
		return "", &OAuthError{Code: OAuthErrorInvalidRequest, Description: "authorization request has already been decided"}
	}
	decisionKey := oauthKey("decision", device.DeviceCodeHash)
	decided, err := s.cache.WriteIfNotExists(decisionKey, "1", time.Until(device.ExpiresAt))
	if err != nil {
		return "", err
	}
	if !decided {
		return "", &OAuthError{Code: OAuthErrorInvalidRequest, Description: "authorization request has already been decided"}
	}
	switch decision {
	case oauthDecisionApprove:
		device.Status, device.User = oauthDeviceStatusApproved, &user
	case oauthDecisionDeny:
		device.Status = oauthDeviceStatusDenied
	}
	if err := s.writeDevice(device); err != nil {
		_ = s.cache.Delete(decisionKey)
		return "", err
	}
	return device.Status, nil
}

func (s *OAuthService) ExchangeDeviceCode(deviceCode string) (*OAuthTokenResponse, error) {
	if deviceCode == "" {
		return nil, &OAuthError{Code: OAuthErrorInvalidRequest, Description: "device_code is required"}
	}
	hash := hashOAuthValue(deviceCode)
	device, err := s.deviceByHash(hash)
	if errors.Is(err, ErrOAuthAuthorizationNotFound) {
		return nil, &OAuthError{Code: oauthErrorExpiredToken, Description: "device code is invalid or expired"}
	}
	if err != nil {
		return nil, err
	}
	allowed, err := s.cache.WriteIfNotExists(oauthKey("poll", hash), "1", oauthDevicePollInterval)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, &OAuthError{Code: oauthErrorSlowDown, Description: "polling too frequently"}
	}
	switch device.Status {
	case oauthDeviceStatusPending:
		return nil, &OAuthError{Code: oauthErrorAuthorizationPending, Description: "authorization is pending"}
	case oauthDeviceStatusDenied:
		s.deleteDevice(device)
		return nil, &OAuthError{Code: oauthErrorAccessDenied, Description: "authorization was denied"}
	case oauthDeviceStatusApproved:
		exchangeKey := oauthKey("exchange", hash)
		claimed, err := s.cache.WriteIfNotExists(exchangeKey, "1", time.Until(device.ExpiresAt))
		if err != nil {
			return nil, err
		}
		if !claimed {
			return nil, &OAuthError{Code: oauthErrorExpiredToken, Description: "device code is invalid or already used"}
		}
		payload, err := s.cache.GetString(oauthKey("device", hash))
		if errors.Is(err, redis.Nil) {
			_ = s.cache.Delete(exchangeKey)
			return nil, &OAuthError{Code: oauthErrorExpiredToken, Description: "device code is invalid or already used"}
		}
		if err != nil {
			_ = s.cache.Delete(exchangeKey)
			return nil, err
		}
		if err := json.Unmarshal([]byte(payload), device); err != nil {
			_ = s.cache.Delete(exchangeKey)
			return nil, err
		}
		tokens, err := s.createOAuthSession(device)
		if err != nil {
			_ = s.cache.Delete(exchangeKey)
			return nil, err
		}
		s.deleteDevice(device)
		return tokens, nil
	default:
		return nil, &OAuthError{Code: oauthErrorInvalidGrant, Description: "invalid authorization state"}
	}
}

func (s *OAuthService) deviceByUserCode(userCode string) (*oauthDevice, error) {
	hash, err := s.cache.GetString(oauthKey("user-code", strings.ToUpper(strings.TrimSpace(userCode))))
	if errors.Is(err, redis.Nil) {
		return nil, ErrOAuthAuthorizationNotFound
	}
	if err != nil {
		return nil, err
	}
	return s.deviceByHash(hash)
}

func (s *OAuthService) deviceByHash(hash string) (*oauthDevice, error) {
	device := new(oauthDevice)
	if err := s.readJSON(oauthKey("device", hash), device); err != nil {
		return nil, err
	}
	if !time.Now().Before(device.ExpiresAt) {
		return nil, ErrOAuthAuthorizationNotFound
	}
	return device, nil
}

func (s *OAuthService) writeDevice(device *oauthDevice) error {
	return s.writeJSON(oauthKey("device", device.DeviceCodeHash), device, time.Until(device.ExpiresAt))
}

func (s *OAuthService) deleteDevice(device *oauthDevice) {
	_ = s.cache.Delete(oauthKey("device", device.DeviceCodeHash))
	_ = s.cache.Delete(oauthKey("user-code", device.UserCode))
	_ = s.cache.Delete(oauthKey("poll", device.DeviceCodeHash))
	_ = s.cache.Delete(oauthKey("decision", device.DeviceCodeHash))
}

func (s *OAuthService) writeJSON(key string, value interface{}, ttl time.Duration) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return s.cache.Write(key, string(payload), ttl)
}

func (s *OAuthService) readJSON(key string, value interface{}) error {
	payload, err := s.cache.GetString(key)
	if errors.Is(err, redis.Nil) {
		return ErrOAuthAuthorizationNotFound
	}
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(payload), value)
}
