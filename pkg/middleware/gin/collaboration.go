/*
Copyright 2022 The KodeRover Authors.

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

	"github.com/koderover/zadig/pkg/microservice/aslan/core/collaboration/service"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
)

func GetCollaborationNew() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := internalhandler.NewContext(c)
		defer func() { internalhandler.JSONResponse(c, ctx) }()

		projectName := c.Query("projectName")
		ifPassFilter := c.Query("ifPassFilter")
		if ifPassFilter == "true" ||
			projectName == "" ||
			c.Request.URL.Path == "/api/collaboration/collaborations/sync" ||
			strings.Contains(c.Request.URL.Path, "/estimated-values") ||
			c.Request.URL.Path == "/api/aslan/environment/rendersets/default-values" {
			c.Next()
			return
		}
		newResp, err := service.GetCollaborationNew(projectName, ctx.UserID, ctx.IdentityType, ctx.Account, ctx.Logger)
		if err != nil {
			ctx.Err = err
			c.Abort()
		}
		if newResp == nil {
			c.Next()
		}
		if newResp.IfSync {
			err := service.SyncCollaborationInstance(nil, projectName, ctx.UserID, ctx.IdentityType, ctx.Account, ctx.RequestID, ctx.Logger)
			if err != nil {
				ctx.Err = err
				c.Abort()
			}
		}
		if len(newResp.Product) == 0 && len(newResp.Workflow) == 0 {
			c.Next()
		} else {
			ctx.Resp = newResp
			c.Abort()
		}
	}
}
