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
	"github.com/gin-gonic/gin"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"

	"github.com/koderover/zadig/v2/pkg/microservice/user/core/service/permission"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
)

func OpenAPIGetResourceActionDefinitions(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectName := c.Query("projectName")
	if projectName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("namespace is empty")
		return
	}

	if !ctx.Resources.IsSystemAdmin {
		if projectName == "*" {
			ctx.UnAuthorized = true
			return
		}

		if _, ok := ctx.Resources.ProjectAuthInfo[projectName]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if !ctx.Resources.ProjectAuthInfo[projectName].IsProjectAdmin {
			ctx.UnAuthorized = true
			return
		}
	}

	envType := c.Query("envType")
	ctx.Resp, ctx.Err = permission.GetResourceActionDefinitions("project", envType, ctx.Logger)
}

func GetResourceActionDefinitions(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	scope := c.Query("scope")
	envType := c.Query("env_type")
	ctx.Resp, ctx.Err = permission.GetResourceActionDefinitions(scope, envType, ctx.Logger)
}
