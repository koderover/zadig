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

package gin

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/shared/client/plutusvendor"
)

const (
	ErrorLicenseMissing             = "未填写许可证"
	ErrorCodeLicenseMissing         = 1000
	ErrorLicenseInvalidVersion      = "许可证版本不匹配"
	ErrorCodeLicenseInvalidVersion  = 1002
	ErrorLicenseExpired             = "许可证已过期"
	ErrorCodeLicenseExpired         = 1001
	ErrorLicenseInvalidSystemID     = "许可证系统不匹配"
	ErrorCodeLicenseInvalidSystemID = 1003
	ErrorUnknown                    = "未知错误"
	ErrorCodeUnknown                = 1010
)

const (
	ZadigXLicenseStatusUninitialized   = "uninitialized"
	ZadigXLicenseStatusNormal          = "normal"
	ZadigXLicenseStatusExpired         = "expired"
	ZadigXLicenseStatusVersionMismatch = "version_mismatch"
	ZadigXLicenseStatusInvalidSystem   = "invalid_system"
)

func ProcessLicense() gin.HandlerFunc {
	return func(c *gin.Context) {
		// if not enterprise we skip
		if !config.Enterprise() {
			c.Next()
			return
		}
		// otherwise we check the path of the request
		if c.Request.URL.Path == "/api/v1/login" ||
			c.Request.URL.Path == "/api/health" ||
			c.Request.URL.Path == "/api/metrics" ||
			c.Request.URL.Path == "/api/v1/users/search" ||
			c.Request.URL.Path == "/api/v1/users" ||
			c.Request.URL.Path == "/api/system/concurrency/workflow" ||
			c.Request.URL.Path == "/api/workflow/plugin/enterprise" ||
			strings.HasPrefix(c.Request.URL.Path, "/api/v1/bundles") ||
			strings.HasPrefix(c.Request.URL.Path, "/api/v1/callback") ||
			strings.HasPrefix(c.Request.URL.Path, "/api/callback") ||
			strings.HasPrefix(c.Request.URL.Path, "/api/v1/system-rolebindings") ||
			strings.HasPrefix(c.Request.URL.Path, "/api/cluster/clusters") {
			c.Next()
			return
		}
		// for the rest of the apis we need to check if the license works
		client := plutusvendor.New()
		resp, err := client.CheckZadigXLicenseStatus()
		if err != nil {
			// if there are some unknown errors we return a
			c.AbortWithStatusJSON(403, gin.H{
				"code":        ErrorCodeUnknown,
				"description": err.Error(),
				"message":     ErrorUnknown,
			})
			return
		}

		switch resp.Status {
		case ZadigXLicenseStatusUninitialized:
			c.AbortWithStatusJSON(403, gin.H{
				"code":        ErrorCodeLicenseMissing,
				"description": "",
				"message":     ErrorLicenseMissing,
			})
			return
		case ZadigXLicenseStatusExpired:
			c.AbortWithStatusJSON(403, gin.H{
				"code":        ErrorCodeLicenseExpired,
				"description": "",
				"message":     ErrorLicenseExpired,
			})
			return
		case ZadigXLicenseStatusVersionMismatch:
			c.AbortWithStatusJSON(403, gin.H{
				"code":        ErrorCodeLicenseInvalidVersion,
				"description": "",
				"message":     ErrorLicenseInvalidVersion,
			})
		case ZadigXLicenseStatusInvalidSystem:
			c.AbortWithStatusJSON(403, gin.H{
				"code":        ErrorCodeLicenseInvalidSystemID,
				"description": "",
				"message":     ErrorLicenseInvalidSystemID,
			})
		case ZadigXLicenseStatusNormal:
			c.Next()
		}
	}
}
