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
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net/url"
	"time"

	"go.uber.org/zap"

	"github.com/koderover/zadig/v2/pkg/microservice/user/config"
	"github.com/koderover/zadig/v2/pkg/microservice/user/core/repository/models"
	"github.com/koderover/zadig/v2/pkg/shared/client/systemconfig"
	"github.com/koderover/zadig/v2/pkg/tool/cache"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

const ThirdPartyLoginStateTTL = 5 * time.Minute

func randomOAuthValue(size int) (string, error) {
	buffer := make([]byte, size)
	if _, err := io.ReadFull(rand.Reader, buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

func hashOAuthValue(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func oauthKey(kind, id string) string {
	return "zadig:oauth:" + kind + ":" + id
}

func CreateThirdPartyLoginState() (state, browserToken string, err error) {
	state, err = randomOAuthValue(32)
	if err != nil {
		log.Errorf("Failed to generate random state for third-party login, err: %s", err)
		return "", "", err
	}
	browserToken, err = randomOAuthValue(32)
	if err != nil {
		log.Errorf("Failed to generate random browser token for third-party login, err: %s", err)
		return "", "", err
	}
	err = cache.NewRedisCache(config.RedisUserTokenDB()).Write(
		oauthKey("login-state", hashOAuthValue(state)), hashOAuthValue(browserToken), ThirdPartyLoginStateTTL,
	)
	if err != nil {
		log.Errorf("Failed to write third-party login state to cache, err: %s", err)
		return "", "", err
	}
	return state, browserToken, nil
}

func ConsumeThirdPartyLoginState(state, browserToken string) error {
	if state == "" || browserToken == "" {
		return fmt.Errorf("missing login state or browser cookie")
	}
	consumed, err := cache.NewRedisCache(config.RedisUserTokenDB()).CompareAndDelete(
		oauthKey("login-state", hashOAuthValue(state)), hashOAuthValue(browserToken),
	)
	if err != nil {
		return err
	}
	if !consumed {
		return fmt.Errorf("invalid or expired login state")
	}
	return nil
}

type enabledStatus struct {
	Enabled bool `json:"enabled"`
}

func ThirdPartyLoginEnabled() *enabledStatus {
	connectors, err := systemconfig.New().ListConnectorsInternal()
	if err != nil {
		log.Warnf("Failed to list connectors, err: %s", err)
	}
	return &enabledStatus{
		Enabled: len(connectors) > 0,
	}
}

func HandleThirdPartyLoginSuccess(user *models.User, logger *zap.SugaredLogger) (string, error) {
	if user == nil {
		return "", fmt.Errorf("user not found")
	}

	mfaRequired, err := IsMFARequiredForUser(user.UID, logger)
	if err != nil {
		return "", fmt.Errorf("failed to check mfa requirement for user: %s", err)
	}

	if !mfaRequired {
		if err = markUserLoginSuccess(user.UID, user.Account, logger); err != nil {
			return "", err
		}

		resp, err := issueLoginToken(user, false, logger)
		if err != nil {
			return "", err
		}
		v := url.Values{}
		v.Add("token", resp.Token)
		return "/?" + v.Encode(), nil
	}

	action, challengeToken, expiresAt, err := PrepareMFALoginChallengeForUser(user.UID, logger)
	if err != nil {
		return "", err
	}
	return buildMFARedirectURL(challengeToken, action, expiresAt), nil
}
