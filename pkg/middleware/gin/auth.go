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

package gin

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/shared/poetry"
	"github.com/koderover/zadig/pkg/tool/log"
)

// Auth 返回会判断 Session 的中间件
func Auth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authorization := c.Request.Header.Get(setting.AuthorizationHeader)
		if authorization != "" {
			if strings.Contains(authorization, setting.RootAPIKey) {
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
			if strings.Contains(authorization, setting.UserAPIKey) {
				token := strings.Split(authorization, " ")
				if len(token) == 2 {
					userInfo, err := poetry.GetUserDetailByToken(config.PoetryServiceAddress(), token[1])
					if err != nil {
						log.Errorf("get user detail err :%v", err)
						c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "get user detail error"})
						return
					}
					c.Set(setting.SessionUsername, userInfo.Name)
					c.Set(setting.SessionUser, userInfo)
					c.Next()
					return
				}
			}
		} else if token, err := c.Cookie("TOKEN"); err == nil {
			userInfo, err := poetry.GetUserDetailByToken(config.PoetryServiceAddress(), token)
			if err != nil {
				log.Errorf("get user detail err :%v", err)
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "get user detail error"})
				return
			}
			c.Set(setting.SessionUsername, userInfo.Name)
			c.Set(setting.SessionUser, userInfo)
			c.Next()
			return
		}

		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "auth failed"})
	}
}
