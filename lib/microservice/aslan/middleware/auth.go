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

package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/qiniu/x/log.v7"

	"github.com/koderover/zadig/lib/microservice/aslan/config"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/service/poetry"
	"github.com/koderover/zadig/lib/setting"
)

// Auth 返回会判断 Session 的中间件
func Auth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authorization := c.Request.Header.Get(setting.HEADERAUTH)
		if authorization != "" {
			if strings.Contains(authorization, setting.ROOTAPIKEY) {
				token := strings.Split(authorization, " ")
				if len(token) == 2 && token[1] == config.PoetryAPIRootKey() {
					c.Set(setting.SessionUsername, "fake-user")
					c.Set(setting.SessionUser, &poetry.UserInfo{Name: "fake-user", IsSuperUser: true, IsAdmin: true, OrganizationID: 1})
					c.Next()
					return
				}
			}
			if strings.Contains(authorization, setting.TIMERAPIKEY) {
				token := strings.Split(authorization, " ")
				if len(token) == 2 && token[1] == config.PoetryAPIRootKey() {
					c.Set(setting.SessionUsername, "timer")
					c.Set(setting.SessionUser, &poetry.UserInfo{Name: "timer", IsSuperUser: true, IsAdmin: true, OrganizationID: 1})
					c.Next()
					return
				}
			}
			if strings.Contains(authorization, setting.USERAPIKEY) {
				token := strings.Split(authorization, " ")
				if len(token) == 2 {
					poetryClient := poetry.NewPoetryServer(config.PoetryAPIServer(), config.PoetryAPIRootKey())

					//根据token获取用户
					responseBody, err := poetryClient.SendRequest(fmt.Sprintf("%s/directory/user/detail", poetryClient.PoetryAPIServer), "GET", "", c.Request.Header)
					if err == nil {
						var userViewInfo poetry.UserViewReponseModel
						err = poetryClient.Deserialize([]byte(responseBody), &userViewInfo)
						if err == nil {
							c.Set(setting.SessionUsername, userViewInfo.User.Name)
							c.Set(setting.SessionUser, userViewInfo.User)
						} else {
							log.Errorf("get user detail err :%v", err)
							c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "can't get user name"})
							return
						}
						c.Next()
						return
					} else {
						log.Errorf("get user detail err :%v", err)
						c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "get user detail error"})
						return
					}
				}
			}
		}

		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "auth failed"})
	}
}
