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

package grpc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	ext_authz_v3 "github.com/envoyproxy/go-control-plane/envoy/service/auth/v3"
	typev3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"
	"go.uber.org/zap"
	"google.golang.org/genproto/googleapis/rpc/code"
	rpc_status "google.golang.org/genproto/googleapis/rpc/status"

	"github.com/koderover/zadig/v2/pkg/microservice/user/config"
	loginsvc "github.com/koderover/zadig/v2/pkg/microservice/user/core/service/login"
	"github.com/koderover/zadig/v2/pkg/microservice/user/core/service/permission"
	"github.com/koderover/zadig/v2/pkg/tool/cache"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

var logger = log.NewFileLogger(config.DecisionLogPath())

type AuthServer struct{}

const (
	mfaEnrollmentRequiredReason   = "mfa_enrollment_required"
	mfaVerificationRequiredReason = "mfa_verification_required"
	mfaReasonHeaderKey            = "x-zadig-auth-reason"

	mfaEnrollmentBypassURLRegExp = `^/api/v1/users/[^/]+/mfa/[A-Za-z0-9_-]+$`
)

func (s *AuthServer) Check(ctx context.Context, request *ext_authz_v3.CheckRequest) (*ext_authz_v3.CheckResponse, error) {
	requestPath := request.GetAttributes().GetRequest().GetHttp().GetPath()
	body := request.GetAttributes().GetRequest().GetHttp().GetBody()
	headers := request.GetAttributes().GetRequest().GetHttp().GetHeaders()
	method := request.GetAttributes().GetRequest().GetHttp().GetMethod()

	isPublicRequest := permission.IsPublicURL(requestPath, method)

	userToken := ""
	var claims *loginsvc.Claims

	if !isPublicRequest {
		if headers["authorization"] != "" {
			segs := strings.Split(headers["authorization"], " ")
			if len(segs) != 2 {
				return denyRequest(
					requestPath,
					method,
					body,
					http.StatusUnauthorized,
					"invalid token: token is malformed",
					"",
					nil,
				), nil
			}
			userToken = segs[1]
		}

		// token in query has higher priority than auth header
		res, err := url.Parse(requestPath)
		if err != nil {
			return denyRequest(
				requestPath,
				method,
				body,
				http.StatusUnauthorized,
				"request url is malformed",
				"",
				err,
			), nil
		}
		if res.Query().Get("token") != "" {
			userToken = res.Query().Get("token")
		}

		// if no token is provided, respond with 401 unauthorized
		if userToken == "" {
			return denyRequest(
				requestPath,
				method,
				body,
				http.StatusUnauthorized,
				"missing authorization header",
				"",
				nil,
			), nil
		} else {
			// validate the token.
			var isValid bool
			claims, isValid, err = permission.ValidateToken(userToken)
			if err != nil || !isValid || claims == nil {
				return denyRequest(
					requestPath,
					method,
					body,
					http.StatusUnauthorized,
					"invalid token: token validation failed",
					"",
					err,
				), nil
			}

			// validate if internal token
			if isInternalToken(claims) {
				goto allowed
			}

			// if the expiration time is so huge that it is not possible, it is a constant api token, we don't check for the redis.
			if claims.ExpiresAt-time.Now().Unix() < 8760*60*60 {
				// check if the given token is removed from the cache
				token, err := cache.NewRedisCache(config.RedisUserTokenDB()).GetString(claims.UID)
				if err != nil {
					return denyRequest(
						requestPath,
						method,
						body,
						http.StatusUnauthorized,
						fmt.Sprintf("cache check failed, error: %s", err),
						"",
						nil,
					), nil
				}

				if token != userToken {
					return denyRequest(
						requestPath,
						method,
						body,
						http.StatusUnauthorized,
						"token mismatch",
						"",
						nil,
					), nil
				}
			}

			mfaReason, err := checkMFAPolicy(requestPath, method, claims)
			if err != nil {
				return denyRequest(
					requestPath,
					method,
					body,
					http.StatusUnauthorized,
					"mfa policy check failed",
					"",
					err,
				), nil
			}
			if mfaReason != "" {
				return denyRequest(
					requestPath,
					method,
					body,
					http.StatusForbidden,
					mfaReason,
					mfaReason,
					nil,
				), nil
			}
		}
	}

allowed:
	resp := &ext_authz_v3.CheckResponse{}
	resp.Status = &rpc_status.Status{Code: int32(code.Code_OK)}
	resp.HttpResponse = &ext_authz_v3.CheckResponse_OkResponse{OkResponse: &ext_authz_v3.OkHttpResponse{}}
	logger.Info("Request Allowed",
		zap.String("path", requestPath),
		zap.String("method", method),
		zap.String("body", body),
		zap.String("token", userToken),
	)

	return resp, nil
}

func checkMFAPolicy(requestPath, method string, claims *loginsvc.Claims) (string, error) {
	if isMFAGateBypassURL(requestPath, method) {
		return "", nil
	}
	if claims == nil || claims.UID == "" {
		return mfaVerificationRequiredReason, nil
	}

	mfaRequired, mfaEnabled, err := loginsvc.GetMFAPolicyForUser(claims.UID, logger.Sugar())
	if err != nil {
		return "", err
	}
	if !mfaRequired {
		return "", nil
	}

	if !mfaEnabled {
		return mfaEnrollmentRequiredReason, nil
	}
	if !claims.MFAVerified {
		return mfaVerificationRequiredReason, nil
	}

	return "", nil
}

func isMFAGateBypassURL(requestPath, method string) bool {
	path := stripQuery(requestPath)

	if path == "/api/v1/login" && (method == http.MethodGet || method == http.MethodPost) {
		return true
	}
	if path == "/api/v1/callback" && method == http.MethodGet {
		return true
	}
	if path == "/api/v1/captcha" && method == http.MethodGet {
		return true
	}
	if path == "/api/v1/login-enabled" && (method == http.MethodGet || method == http.MethodPost) {
		return true
	}
	if path == "/api/v1/logout" && method == http.MethodGet {
		return true
	}
	if path == "/api/v1/login/mfa/setup" && method == http.MethodPost {
		return true
	}
	if path == "/api/v1/login/mfa/enroll" && method == http.MethodPost {
		return true
	}
	if path == "/api/v1/login/mfa/verify" && method == http.MethodPost {
		return true
	}

	match, _ := regexp.MatchString(mfaEnrollmentBypassURLRegExp, path)
	if match && (method == http.MethodPost || method == http.MethodGet) {
		return true
	}

	return false
}

func stripQuery(path string) string {
	segments := strings.Split(path, "?")
	return segments[0]
}

func isInternalToken(claims *loginsvc.Claims) bool {
	if claims == nil {
		return false
	}
	return claims.ExpiresAt == 0 &&
		(claims.Name == "aslan" && claims.PreferredUsername == "aslan" && claims.FederatedClaims.UserId == "aslan" && claims.Email == "aslan@koderover.com" ||
			claims.Name == "user" && claims.PreferredUsername == "user" && claims.FederatedClaims.UserId == "user" && claims.Email == "user@koderover.com" ||
			claims.Name == "cron" && claims.PreferredUsername == "cron" && claims.FederatedClaims.UserId == "cron" && claims.Email == "cron@koderover.com" ||
			claims.Name == "hub-agent" && claims.PreferredUsername == "hub-agent" && claims.FederatedClaims.UserId == "hub-agent" && claims.Email == "hub-agent@koderover.com" ||
			claims.Name == "hub-server" && claims.PreferredUsername == "hub-server" && claims.FederatedClaims.UserId == "hub-server" && claims.Email == "hub-server@koderover.com")
}

func denyRequest(requestPath, method, body string, statusCode int, reason, responseReason string, err error) *ext_authz_v3.CheckResponse {
	resp := &ext_authz_v3.CheckResponse{
		Status: &rpc_status.Status{Code: int32(toRPCCode(statusCode))},
	}

	denied := &ext_authz_v3.DeniedHttpResponse{
		Status: &typev3.HttpStatus{Code: typev3.StatusCode(statusCode)},
	}
	if responseReason != "" {
		denied.Headers = []*corev3.HeaderValueOption{
			{Header: &corev3.HeaderValue{Key: mfaReasonHeaderKey, Value: responseReason}},
			{Header: &corev3.HeaderValue{Key: "content-type", Value: "application/json"}},
		}
		denied.Body = encodeReasonJSON(responseReason)
	}
	resp.HttpResponse = &ext_authz_v3.CheckResponse_DeniedResponse{DeniedResponse: denied}

	fields := []zap.Field{
		zap.String("path", requestPath),
		zap.String("method", method),
		zap.String("body", body),
		zap.String("reason", reason),
	}
	if err != nil {
		fields = append(fields, zap.String("error", err.Error()))
	}
	logger.Info("Request Denied", fields...)
	return resp
}

func encodeReasonJSON(reason string) string {
	payload, err := json.Marshal(map[string]string{"reason": reason})
	if err != nil {
		return fmt.Sprintf("{\"reason\":\"%s\"}", reason)
	}
	return string(payload)
}

func toRPCCode(statusCode int) code.Code {
	switch statusCode {
	case http.StatusUnauthorized:
		return code.Code_UNAUTHENTICATED
	case http.StatusForbidden:
		return code.Code_PERMISSION_DENIED
	default:
		return code.Code_UNKNOWN
	}
}
