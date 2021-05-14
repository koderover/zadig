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
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/koderover/zadig/lib/microservice/podexec/config"
	"github.com/koderover/zadig/lib/setting"
	"github.com/koderover/zadig/lib/tool/log"
	"github.com/koderover/zadig/lib/tool/xlog"
	"github.com/koderover/zadig/lib/types/permission"
)

func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		url := r.URL.Path
		if strings.Contains("/api/podexec/health/", url) {
			next.ServeHTTP(w, r)
			return
		}
		poetryClient := &PoetryClient{
			PoetryAPIServer: config.PoetryAPIServer(),
			ApiRootKey:      config.PoetryAPIRootKey(),
		}

		authorization := r.Header.Get(setting.HEADERAUTH)
		if authorization != "" {
			if strings.Contains(authorization, setting.ROOTAPIKEY) {
				token := strings.Split(authorization, " ")
				if len(token) == 2 && token[1] == poetryClient.ApiRootKey {
					next.ServeHTTP(w, r)
					return
				}
			}
			if strings.Contains(authorization, setting.USERAPIKEY) {
				token := strings.Split(authorization, " ")
				if len(token) == 2 {
					//根据token获取用户
					err := PoetryRequest(poetryClient, r)
					if err != nil {
						log.Errorf("PoetryRequest err:%v", err)
						w.WriteHeader(http.StatusUnauthorized)
						_ = json.NewEncoder(w).Encode(&EndpointResponse{ResultCode: http.StatusUnauthorized, ErrorMsg: "auth failed"})
					} else {
						next.ServeHTTP(w, r)
					}
					return
				}
			}
		} else {
			if session, err := r.Cookie("SESSION"); err == nil {
				r.Header.Add("cookie", session.String())
				err := PoetryRequest(poetryClient, r)
				if err != nil {
					log.Errorf("PoetryRequest err:%v", err)
					w.WriteHeader(http.StatusUnauthorized)
					_ = json.NewEncoder(w).Encode(&EndpointResponse{ResultCode: http.StatusUnauthorized, ErrorMsg: "auth failed"})
				} else {
					next.ServeHTTP(w, r)
				}
				return
			}
		}
	})
}

func PermissionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		url := r.URL.Path
		if strings.Contains("/api/podexec/health/", url) {
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
		poetryClient := &PoetryClient{
			PoetryAPIServer: config.PoetryAPIServer(),
			ApiRootKey:      config.PoetryAPIRootKey(),
		}
		queryList := r.URL.Query()
		clusterId := queryList.Get("clusterId")
		if clusterId == "" {
			isHaveTestEnvManagePermission := PoetryRequestPermission(poetryClient, userID, permission.TestEnvManageUUID, productName, w, r)
			if isHaveTestEnvManagePermission {
				next.ServeHTTP(w, r)
				return
			}
		} else {
			isHaveProdEnvManagePermission := PoetryRequestPermission(poetryClient, userID, permission.ProdEnvManageUUID, productName, w, r)
			if isHaveProdEnvManagePermission {
				next.ServeHTTP(w, r)
				return
			}
		}

		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(&EndpointResponse{ResultCode: http.StatusForbidden, ErrorMsg: "forbidden!"})
	})
}

func PoetryRequestPermission(poetryClient *PoetryClient, userID int, permissionUUID, productName string, w http.ResponseWriter, r *http.Request) bool {
	header := http.Header{}
	header.Set(setting.Auth, fmt.Sprintf("%s%s", setting.AuthPrefix, poetryClient.ApiRootKey))
	responseBody, err := poetryClient.SendRequest(fmt.Sprintf("%s/directory/userPermissionRelation?userId=%d&permissionUUID=%s&permissionType=%d&productName=%s", poetryClient.PoetryAPIServer, userID, permissionUUID, 2, productName), "GET", nil, header)
	if err != nil {
		log.Errorf("userPermissionRelation request error: %v", err)
		return false
	}
	rolePermissionMap := make(map[string]bool)
	err = poetryClient.Deserialize([]byte(responseBody), &rolePermissionMap)
	if err != nil {
		return false
	}
	if rolePermissionMap["isContain"] {
		return true
	}

	logger := xlog.New(w, r)
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

func PoetryRequest(poetryClient *PoetryClient, r *http.Request) error {
	r.Header.Set("Accept-Encoding", "")
	responseBody, err := poetryClient.SendRequest(fmt.Sprintf("%s/directory/user/detail", poetryClient.PoetryAPIServer), "GET", "", r.Header)
	if err == nil {
		var userViewInfo UserViewReponseModel
		err := json.Unmarshal([]byte(responseBody), &userViewInfo)
		if err == nil {
			r.Header.Set("userId", strconv.Itoa(userViewInfo.User.ID))
			r.Header.Set("isSuperUser", strconv.FormatBool(userViewInfo.User.IsSuperUser))
			return nil
		} else {
			return fmt.Errorf("unmarshal err :%v", err)
		}
	} else {
		return fmt.Errorf("SendRequest err :%v", err)
	}
}
