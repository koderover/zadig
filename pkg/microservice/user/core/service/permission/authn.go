/*
Copyright 2023 The KodeRover Authors.

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

package permission

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/golang-jwt/jwt"
	globalConfig "github.com/koderover/zadig/v2/pkg/config"
	userConfig "github.com/koderover/zadig/v2/pkg/microservice/user/config"
	"github.com/koderover/zadig/v2/pkg/microservice/user/core/service/login"
	"github.com/koderover/zadig/v2/pkg/tool/cache"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

const (
	larkWebhookURLRegExp         = `^\/api\/aslan\/system\/lark\/\w+\/webhook$`
	dingTalkWebhookURLRegExp     = `^\/api\/aslan\/system\/dingtalk\/\w+\/webhook$`
	workwxWebhookURLRegExp       = `^\/api\/aslan\/system\/workwx\/\w+\/webhook$`
	getClusterAgentYamlURLRegExp = `^\/api\/aslan\/cluster\/agent\/\w+\/agent.yaml$`
	envWorkloadUrlRegExp         = `^\/api\/aslan\/environment\/environments\/[\w-]+\/check\/workloads\/k8services$`
	envShareEnableURLRegExp      = `^\/api\/aslan\/environment\/environments\/[\w-]+\/check\/sharenv\/enable\/ready$`
	envShareDisableURLRegExp     = `^\/api\/aslan\/environment\/environments\/[\w-]+\/check\/sharenv\/disable\/ready$`
	serviceDeployableURLRegExp   = `^\/api\/aslan\/service\/services\/[\w-]+\/environments\/deployable$`
	generalWebhookURLRegExp      = `^\/api\/aslan\/workflow\/v4\/generalhook\/[\w-]+\/[^/]+\/webhook$`
	codeHostAuthURLRegExp        = `^\/api\/v1\/codehosts\/\w+\/auth$`
	// workflowTestTaskReportURLRegExp = `^\/api\/aslan\/testing\/report\/workflowv4\/[\w-]+\/id\/\w+\/job\/[^/]+$`
	// testingTaskReportURLRegExp      = `^\/api\/aslan\/testing\/testtask\/[\w-]+\/\w+\/[^/]+$`
)

func IsPublicURL(reqPath, method string) bool {
	// remove query from the given path
	segments := strings.Split(reqPath, "?")

	realPath := segments[0]

	// not starting with api means this is a frontend request, making it public
	if !strings.HasPrefix(realPath, "/api") && !strings.HasPrefix(realPath, "/openapi") {
		return true
	}

	// api docs is public
	if strings.HasPrefix(realPath, "/api/aslan/apidocs/") {
		return true
	}

	// all agent related APIs do their authentication process in the business logic layer
	if strings.HasPrefix(realPath, "/api/aslan/vm/agents") {
		return true
	}

	if realPath == "/api/v1/reset" && method == http.MethodPost {
		return true
	}

	if realPath == "/api/aslan/system/initialization/status" && method == http.MethodGet {
		return true
	}

	if realPath == "/api/aslan/system/initialization/user" && method == http.MethodPost {
		return true
	}

	if strings.HasPrefix(realPath, "/debug") && method == http.MethodGet {
		return true
	}

	if realPath == "/api/plutus-enterprise/license" && (method == http.MethodPost || method == http.MethodGet) {
		return true
	}

	if realPath == "/api/plutus-enterprise/organization" && method == http.MethodGet {
		return true
	}

	if realPath == "/api/plutus/version/new/check" && method == http.MethodGet {
		return true
	}

	if realPath == "/api/plutus/license/sync" && method == http.MethodPost {
		return true
	}

	if strings.HasPrefix(realPath, "/api/plutus/license/download") && method == http.MethodGet {
		return true
	}

	if realPath == "/api/plutus/customer/uploadCurrentVersion" && method == http.MethodPost {
		return true
	}

	if realPath == "/api/plutus/version/latest" && method == http.MethodGet {
		return true
	}

	if realPath == "/api/plutus/license/check" && method == http.MethodPost {
		return true
	}

	if strings.HasPrefix(realPath, "/api/plutus/customer/installer") && method == http.MethodGet {
		return true
	}

	if realPath == "/api/plutus/connect" && method == http.MethodGet {
		return true
	}

	if realPath == "/api/aslan/metrics" && method == http.MethodGet {
		return true
	}

	if realPath == "/api/aslan/webhook" && method == http.MethodPost {
		return true
	}

	if realPath == "/api/hub/connect" && method == http.MethodGet {
		return true
	}

	if realPath == "/api/v1/login" && (method == http.MethodGet || method == http.MethodPost) {
		return true
	}

	if realPath == "/api/v1/captcha" && method == http.MethodGet {
		return true
	}

	if realPath == "/api/v1/login/mfa/setup" && method == http.MethodPost {
		return true
	}

	if realPath == "/api/v1/login/mfa/enroll" && method == http.MethodPost {
		return true
	}

	if realPath == "/api/v1/login/mfa/verify" && method == http.MethodPost {
		return true
	}

	if realPath == "/api/v1/signup" && (method == http.MethodGet || method == http.MethodPost) {
		return true
	}

	if realPath == "/api/v1/retrieve" && (method == http.MethodGet || method == http.MethodPost) {
		return true
	}

	if realPath == "/api/v1/login-enabled" && (method == http.MethodGet || method == http.MethodPost) {
		return true
	}

	if (realPath == "/api/v1/codehosts/callback" || realPath == "/api/directory/codehosts/callback") && method == http.MethodGet {
		return true
	}

	if realPath == "/api/aslan/system/llm/integration/check" && method == http.MethodGet {
		return true
	}

	if realPath == "/api/aslan/system/theme" && method == http.MethodGet {
		return true
	}

	if realPath == "/api/callback" && method == http.MethodPost {
		return true
	}

	if realPath == "/api/v1/callback" {
		return true
	}

	if strings.HasPrefix(realPath, "/api/aslan/system/project_management/jira/webhook") && method == http.MethodPost {
		return true
	}

	if strings.HasPrefix(realPath, "/api/aslan/system/project_management/meego/webhook") && method == http.MethodPost {
		return true
	}

	if strings.HasPrefix(realPath, "/api/v1/features") && method == http.MethodGet {
		return true
	}

	if strings.HasPrefix(realPath, "/api/aslan/testing/report/html/workflowv4") && method == http.MethodGet {
		return true
	}
	if strings.HasPrefix(realPath, "/api/aslan/testing/report/html/testing") && method == http.MethodGet {
		return true
	}

	// the only possible error for MatchString is an invalid regular expression, which is not possible, so we will just ignore it
	match, _ := regexp.MatchString(larkWebhookURLRegExp, realPath)
	if match && method == http.MethodPost {
		return true
	}

	match, _ = regexp.MatchString(dingTalkWebhookURLRegExp, realPath)
	if match && method == http.MethodPost {
		return true
	}

	match, _ = regexp.MatchString(workwxWebhookURLRegExp, realPath)
	if match && (method == http.MethodPost || method == http.MethodGet) {
		return true
	}

	match, _ = regexp.MatchString(getClusterAgentYamlURLRegExp, realPath)
	if match && method == http.MethodGet {
		return true
	}

	match, _ = regexp.MatchString(codeHostAuthURLRegExp, realPath)
	if match && method == http.MethodGet {
		return true
	}

	match, _ = regexp.MatchString(envWorkloadUrlRegExp, realPath)
	if match && method == http.MethodGet {
		return true
	}

	match, _ = regexp.MatchString(envShareEnableURLRegExp, realPath)
	if match && method == http.MethodGet {
		return true
	}

	match, _ = regexp.MatchString(envShareDisableURLRegExp, realPath)
	if match && method == http.MethodGet {
		return true
	}

	match, _ = regexp.MatchString(serviceDeployableURLRegExp, realPath)
	if match && method == http.MethodGet {
		return true
	}

	match, _ = regexp.MatchString(generalWebhookURLRegExp, realPath)
	if match && method == http.MethodPost {
		return true
	}

	return false
}

// ValidateToken validates if the token is valid and returns the claims that belongs to this token if the token is valid
func ValidateToken(tokenString string) (*login.Claims, bool, error) {
	secretKey := globalConfig.SecretKey()

	token, err := jwt.ParseWithClaims(tokenString, &login.Claims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(secretKey), nil
	})

	if err != nil {
		log.Errorf("Failed to parse JWT, err: %s", err)
		return nil, false, err
	}

	if claims, ok := token.Claims.(*login.Claims); ok && token.Valid {
		// internal tokens bypass runtime revocation checks
		if isInternalTokenClaims(claims) {
			return claims, true, nil
		}

		// short-lived login token: validate against redis cache
		if claims.ExpiresAt-time.Now().Unix() < 8760*60*60 {
			cachedToken, err := cache.NewRedisCache(userConfig.RedisUserTokenDB()).GetString(claims.UID)
			if err != nil {
				log.Errorf("Failed to validate token against redis cache, uid: %s, err: %s", claims.UID, err)
				return nil, false, err
			}
			if cachedToken != tokenString {
				log.Errorf("token mismatch for uid: %s", claims.UID)
				return nil, false, fmt.Errorf("token mismatch")
			}
			return claims, true, nil
		}

		// long-lived api token: validate against current user token and authorization switch
		matched, err := ValidateAPIToken(claims.UID, tokenString)
		if err != nil {
			log.Errorf("Failed to validate api token, uid: %s, err: %s", claims.UID, err)
			return nil, false, err
		}
		if !matched {
			log.Errorf("api token mismatch or unauthorized for uid: %s", claims.UID)
			return nil, false, fmt.Errorf("token mismatch")
		}

		return claims, true, nil
	} else {
		log.Errorf("invalid token detected")
		return nil, false, fmt.Errorf("invalid token")
	}
}

func isInternalTokenClaims(claims *login.Claims) bool {
	return claims.ExpiresAt == 0 &&
		(claims.Name == "aslan" && claims.PreferredUsername == "aslan" && claims.FederatedClaims.UserId == "aslan" && claims.Email == "aslan@koderover.com" ||
			claims.Name == "user" && claims.PreferredUsername == "user" && claims.FederatedClaims.UserId == "user" && claims.Email == "user@koderover.com" ||
			claims.Name == "cron" && claims.PreferredUsername == "cron" && claims.FederatedClaims.UserId == "cron" && claims.Email == "cron@koderover.com" ||
			claims.Name == "hub-agent" && claims.PreferredUsername == "hub-agent" && claims.FederatedClaims.UserId == "hub-agent" && claims.Email == "hub-agent@koderover.com" ||
			claims.Name == "hub-server" && claims.PreferredUsername == "hub-server" && claims.FederatedClaims.UserId == "hub-server" && claims.Email == "hub-server@koderover.com")
}
