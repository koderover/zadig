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
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	ext_authz_v3 "github.com/envoyproxy/go-control-plane/envoy/service/auth/v3"
	typev3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"
	"go.uber.org/zap"
	"google.golang.org/genproto/googleapis/rpc/code"
	rpc_status "google.golang.org/genproto/googleapis/rpc/status"

	"github.com/koderover/zadig/v2/pkg/microservice/user/config"
	"github.com/koderover/zadig/v2/pkg/microservice/user/core/service/permission"
	"github.com/koderover/zadig/v2/pkg/tool/cache"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

var logger = log.NewFileLogger(config.DecisionLogPath())

type AuthServer struct{}

func (s *AuthServer) Check(ctx context.Context, request *ext_authz_v3.CheckRequest) (*ext_authz_v3.CheckResponse, error) {
	requestPath := request.GetAttributes().GetRequest().GetHttp().GetPath()
	body := request.GetAttributes().GetRequest().GetHttp().GetBody()
	headers := request.GetAttributes().GetRequest().GetHttp().GetHeaders()
	method := request.GetAttributes().GetRequest().GetHttp().GetMethod()

	resp := &ext_authz_v3.CheckResponse{}

	isPublicRequest := permission.IsPublicURL(requestPath, method)

	userToken := ""

	if !isPublicRequest {
		if headers["authorization"] != "" {
			segs := strings.Split(headers["authorization"], " ")
			if len(segs) != 2 {
				resp.Status = &rpc_status.Status{Code: int32(code.Code_UNAUTHENTICATED)}
				resp.HttpResponse = &ext_authz_v3.CheckResponse_DeniedResponse{DeniedResponse: &ext_authz_v3.DeniedHttpResponse{
					Status: &typev3.HttpStatus{Code: http.StatusUnauthorized},
				}}
				logger.Info("Request Denied",
					zap.String("path", requestPath),
					zap.String("method", method),
					zap.String("body", body),
					zap.String("reason", "invalid token: token is malformed"),
				)
				return resp, nil
			}
			userToken = segs[1]
		}

		// token in query has higher priority than auth header
		res, err := url.Parse(requestPath)
		if err != nil {
			resp.Status = &rpc_status.Status{Code: int32(code.Code_UNAUTHENTICATED)}
			resp.HttpResponse = &ext_authz_v3.CheckResponse_DeniedResponse{DeniedResponse: &ext_authz_v3.DeniedHttpResponse{
				Status: &typev3.HttpStatus{Code: http.StatusUnauthorized},
			}}
			logger.Info("Request Denied",
				zap.String("path", requestPath),
				zap.String("method", method),
				zap.String("body", body),
				zap.String("reason", "request url is malformed"),
				zap.String("error", err.Error()),
			)
			return resp, nil
		}
		if res.Query().Get("token") != "" {
			userToken = res.Query().Get("token")
		}

		// if no token is provided, respond with 401 unauthorized
		if userToken == "" {
			resp.Status = &rpc_status.Status{Code: int32(code.Code_UNAUTHENTICATED)}
			resp.HttpResponse = &ext_authz_v3.CheckResponse_DeniedResponse{DeniedResponse: &ext_authz_v3.DeniedHttpResponse{
				Status: &typev3.HttpStatus{Code: http.StatusUnauthorized},
			}}
			logger.Info("Request Denied",
				zap.String("path", requestPath),
				zap.String("method", method),
				zap.String("body", body),
				zap.String("reason", "missing authorization header"),
			)
			return resp, nil
		} else {
			// validate the token.
			claims, isValid, err := permission.ValidateToken(userToken)
			if err != nil || !isValid {
				resp.Status = &rpc_status.Status{Code: int32(code.Code_UNAUTHENTICATED)}
				resp.HttpResponse = &ext_authz_v3.CheckResponse_DeniedResponse{DeniedResponse: &ext_authz_v3.DeniedHttpResponse{
					Status: &typev3.HttpStatus{Code: http.StatusUnauthorized},
				}}
				logger.Info("Request Denied",
					zap.String("path", requestPath),
					zap.String("method", method),
					zap.String("body", body),
					zap.String("reason", "invalid token: token validation failed"),
					zap.String("error", err.Error()),
				)
				return resp, nil
			}

			// validate if internal token
			if claims.ExpiresAt == 0 &&
				(claims.Name == "aslan" && claims.PreferredUsername == "aslan" && claims.FederatedClaims.UserId == "aslan" && claims.Email == "aslan@koderover.com" ||
					claims.Name == "user" && claims.PreferredUsername == "user" && claims.FederatedClaims.UserId == "user" && claims.Email == "user@koderover.com" ||
					claims.Name == "cron" && claims.PreferredUsername == "cron" && claims.FederatedClaims.UserId == "cron" && claims.Email == "cron@koderover.com" ||
					claims.Name == "hub-agent" && claims.PreferredUsername == "hub-agent" && claims.FederatedClaims.UserId == "hub-agent" && claims.Email == "hub-agent@koderover.com" ||
					claims.Name == "hub-server" && claims.PreferredUsername == "hub-server" && claims.FederatedClaims.UserId == "hub-server" && claims.Email == "hub-server@koderover.com") {
				goto allowed
			}

			// if the expiration time is so huge that it is not possible, it is a constant api token, we don't check for the redis.
			if claims.ExpiresAt-time.Now().Unix() < 8760*60*60 {
				// check if the given token is removed from the cache
				token, err := cache.NewRedisCache(config.RedisUserTokenDB()).GetString(claims.UID)
				if err != nil {
					resp.Status = &rpc_status.Status{Code: int32(code.Code_UNAUTHENTICATED)}
					resp.HttpResponse = &ext_authz_v3.CheckResponse_DeniedResponse{DeniedResponse: &ext_authz_v3.DeniedHttpResponse{
						Status: &typev3.HttpStatus{Code: http.StatusUnauthorized},
					}}
					errReason := fmt.Sprintf("cache check failed, error: %s", err)
					logger.Info("Request Denied",
						zap.String("path", requestPath),
						zap.String("method", method),
						zap.String("body", body),
						zap.String("reason", errReason),
					)
					return resp, nil
				}

				if token != userToken {
					resp.Status = &rpc_status.Status{Code: int32(code.Code_UNAUTHENTICATED)}
					resp.HttpResponse = &ext_authz_v3.CheckResponse_DeniedResponse{DeniedResponse: &ext_authz_v3.DeniedHttpResponse{
						Status: &typev3.HttpStatus{Code: http.StatusUnauthorized},
					}}
					logger.Info("Request Denied",
						zap.String("path", requestPath),
						zap.String("method", method),
						zap.String("body", body),
						zap.String("reason", "token mismatch"),
					)
					return resp, nil
				}
			}
		}
	}

allowed:
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
