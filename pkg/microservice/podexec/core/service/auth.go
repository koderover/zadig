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

package service

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/sets"

	configbase "github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/podexec/config"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/shared/poetry"
	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/koderover/zadig/pkg/types/permission"
)

func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		url := r.URL.Path
		if strings.Contains("/api/health/", url) {
			next.ServeHTTP(w, r)
			return
		}

		authorization := r.Header.Get(setting.AuthorizationHeader)
		if authorization != "" {
			if strings.Contains(authorization, setting.RootAPIKey) {
				token := strings.Split(authorization, " ")
				if len(token) == 2 && token[1] == config.PoetryAPIRootKey() {
					next.ServeHTTP(w, r)
				}
				return
			}

			if strings.Contains(authorization, setting.UserAPIKey) {
				token := strings.Split(authorization, " ")
				if len(token) == 2 {
					//根据token获取用户
					userInfo, err := poetry.GetUserDetailByToken(configbase.PoetryServiceAddress(), token[1])
					if err != nil {
						w.WriteHeader(http.StatusUnauthorized)
						_ = json.NewEncoder(w).Encode(&EndpointResponse{ResultCode: http.StatusUnauthorized, ErrorMsg: "auth failed"})
						return
					}

					r.Header.Set("userId", strconv.Itoa(userInfo.ID))
					r.Header.Set("isSuperUser", strconv.FormatBool(userInfo.IsSuperUser))
					next.ServeHTTP(w, r)
				}
				return
			}
		} else if token, err := r.Cookie("TOKEN"); err == nil {
			userInfo, err := poetry.GetUserDetailByToken(configbase.PoetryServiceAddress(), token.Value)
			if err != nil {
				log.Errorf("PoetryRequest err:%v", err)
				w.WriteHeader(http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(&EndpointResponse{ResultCode: http.StatusUnauthorized, ErrorMsg: "auth failed"})
				return
			}

			r.Header.Set("userId", strconv.Itoa(userInfo.ID))
			r.Header.Set("isSuperUser", strconv.FormatBool(userInfo.IsSuperUser))
			next.ServeHTTP(w, r)
		}
	})
}

func PermissionMiddleware(next http.Handler) http.Handler {
	logger := log.SugaredLogger()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		url := r.URL.Path
		if strings.Contains("/api/health/", url) {
			next.ServeHTTP(w, r)
			return
		}
		pathParams := mux.Vars(r)
		productName := pathParams["productName"]
		userIDStr := r.Header.Get("userId")
		isSuperUserStr := r.Header.Get("isSuperUser")
		isSuperUser, parseErr := strconv.ParseBool(isSuperUserStr)
		if isSuperUser {
			next.ServeHTTP(w, r)
			return
		}
		userID, atoiErr := strconv.Atoi(userIDStr)
		if parseErr != nil || atoiErr != nil {
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(&EndpointResponse{ResultCode: http.StatusForbidden, ErrorMsg: "forbidden!"})
			return
		}

		queryList := r.URL.Query()
		clusterID := queryList.Get("clusterId")
		if clusterID == "" {
			isHaveTestEnvManagePermission := PoetryRequestPermission(userID, permission.TestEnvManageUUID, productName, logger)
			if isHaveTestEnvManagePermission {
				next.ServeHTTP(w, r)
				return
			}
		} else {
			isHaveProdEnvManagePermission := PoetryRequestPermission(userID, permission.ProdEnvManageUUID, productName, logger)
			if isHaveProdEnvManagePermission {
				next.ServeHTTP(w, r)
				return
			}
		}

		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(&EndpointResponse{ResultCode: http.StatusForbidden, ErrorMsg: "forbidden!"})
	})
}

func PoetryRequestPermission(userID int, permissionUUID, productName string, logger *zap.SugaredLogger) bool {
	poetryClient := poetry.New(configbase.PoetryServiceAddress(), config.PoetryAPIRootKey())
	if poetryClient.HasOperatePermission(productName, permissionUUID, userID, false, logger) {
		return true
	}

	uuids, err := poetryClient.GetUserPermissionUUIDs(setting.RoleUserID, "", logger)
	if err != nil {
		log.Errorf("GetUserPermissionUUIDs error: %v", err)
		return false
	}
	if sets.NewString(uuids...).Has(permissionUUID) {
		return true
	}

	return false
}
