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
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/shared/poetry"
	"github.com/koderover/zadig/pkg/types/permission"
	"github.com/koderover/zadig/pkg/util/ginzap"
)

// RequireSuperAdminAuth require super user role
func RequireSuperAdminAuth(c *gin.Context) {
	log := ginzap.WithContext(c).Sugar()
	username := c.GetString(setting.SessionUsername)
	user, err := authUser(username, c)
	if err != nil {
		log.Errorf("authUser(%s) failed, %v", username, err)
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "Authentication failed."})
		return
	}
	if user.IsSuperUser {
		c.Next()
		return
	}
	c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"message": "Require super user permission"})
}

//判断用户是否有某个权限
func IsHavePermission(permissionUUIDs []string, paramType int) gin.HandlerFunc {
	return func(c *gin.Context) {
		log := ginzap.WithContext(c).Sugar()
		username := c.GetString(setting.SessionUsername)
		user, err := authUser(username, c)
		if err != nil {
			log.Errorf("authUser(%s) failed, %v", username, err)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "Authentication failed."})
			return
		}

		if user.IsSuperUser {
			c.Next()
			return
		}

		var productName string
		poetryClient := poetry.New(config.PoetryServiceAddress(), config.PoetryAPIRootKey())

		if paramType == permission.ParamType {
			productName = c.Param("name")
			if productName == "" {
				productName = c.Param("productName")
			}
		} else if paramType == permission.QueryType {
			productName = c.Query("productName")
			if productName == "" {
				productName = c.Query("productTmpl")
			}
		} else if paramType == permission.ContextKeyType {
			productName = c.GetString("productName")
		}

		if productName == "" {
			log.Errorf("productName is null")
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "Authentication failed."})
			return
		}

		//其他权限
		permissionUUID := ""
		if len(permissionUUIDs) == 1 {
			permissionUUID = permissionUUIDs[0]
		} else if len(permissionUUIDs) == 2 || len(permissionUUIDs) == 3 {
			envType := c.Query("envType")
			switch envType {
			case setting.ProdENV:
				permissionUUID = permissionUUIDs[1]
			default:
				permissionUUID = permissionUUIDs[0]
			}
		}

		//判断用户是否有操作的权限uuid
		if rolePermissionMap, err := poetryClient.GetUserPermissionUUIDMap(productName, permissionUUID, user.ID, log); err == nil {
			if rolePermissionMap["isContain"] {
				c.Next()
				return
			}
		} else {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "Authentication failed."})
			return
		}

		//判断用户是否有环境授权权限uuid
		if roleEnvPermissions, err := poetryClient.ListEnvRolePermission(productName, "", 0, log); err == nil {
			for _, roleEnvPermission := range roleEnvPermissions {
				if roleEnvPermission.PermissionUUID == permissionUUID {
					c.Next()
					return
				}
			}
		} else {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "Authentication failed."})
			return
		}

		if len(permissionUUIDs) == 3 {
			permissionUUID = permissionUUIDs[2]
			if rolePermissionMap, err := poetryClient.GetUserPermissionUUIDMap(productName, permissionUUID, user.ID, log); err == nil {
				if rolePermissionMap["isContain"] {
					c.Next()
					return
				}
			} else {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "Authentication failed."})
				return
			}
		}

		// 判断项目里面是否有all-users设置，以及该有的权限
		productRole, _ := poetryClient.ListRoles(productName, log)
		if productRole != nil {
			uuids, err := poetryClient.GetUserPermissionUUIDs(productRole.ID, productName, log)
			if err != nil {
				log.Errorf("GetUserPermissionUUIDs error: %v", err)
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "Authentication failed."})
				return
			}
			if sets.NewString(uuids...).Has(permissionUUID) {
				c.Next()
				return
			}
		}

		//判断普通用户的默认权限是否包含
		uuids, err := poetryClient.GetUserPermissionUUIDs(setting.RoleUserID, "", log)
		if err != nil {
			log.Errorf("GetUserPermissionUUIDs error: %v", err)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "Authentication failed."})
			return
		}
		if sets.NewString(uuids...).Has(permissionUUID) {
			c.Next()
			return
		}

		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"message": "对不起,您没有权限访问!"})
	}
}

func authUser(username interface{}, c *gin.Context) (*permission.User, error) {

	username, ok := username.(string)
	if !ok || username == "" {
		return nil, fmt.Errorf("empty user id")
	}

	userInfo, isExist := c.Get(setting.SessionUser)
	user := new(poetry.UserInfo)
	if isExist {
		user = userInfo.(*poetry.UserInfo)
	}
	//说明用户可能已经删除
	if user.Name == "" {
		return nil, fmt.Errorf("user %s has been disabled", username)
	}

	return poetry.ConvertUserInfo(user), nil
}
