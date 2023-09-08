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
	userhandler "github.com/koderover/zadig/pkg/microservice/user/core/handler/user"
	"github.com/koderover/zadig/pkg/microservice/user/core/service/permission"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

type initializeProjectReq struct {
	Namespace string   `json:"namespace"`
	IsPublic  bool     `json:"is_public"`
	Admins    []string `json:"admins"`
}

func InitializeProject(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	req := new(initializeProjectReq)

	err := c.BindJSON(&req)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	if req.Namespace == "*" {
		ctx.Err = e.ErrInvalidParam.AddDesc("args namespace can't be *")
		return
	}

	err = userhandler.GenerateUserAuthInfo(ctx)
	if err != nil {
		ctx.UnAuthorized = true
		ctx.Err = fmt.Errorf("failed to generate user authorization info, error: %s", err)
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if authInfo, ok := ctx.Resources.ProjectAuthInfo[req.Namespace]; !ok {
			ctx.UnAuthorized = true
			return
		} else if !authInfo.IsProjectAdmin {
			ctx.UnAuthorized = true
			return
		}
	}

	ctx.Err = permission.InitializeProjectAuthorization(req.Namespace, req.IsPublic, req.Admins, ctx.Logger)
}

func DeleteProjectRoles(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	namespace := c.Query("namespace")
	if namespace == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("args namespace can't be empty")
		return
	}

	err := userhandler.GenerateUserAuthInfo(ctx)
	if err != nil {
		ctx.UnAuthorized = true
		ctx.Err = fmt.Errorf("failed to generate user authorization info, error: %s", err)
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if authInfo, ok := ctx.Resources.ProjectAuthInfo[namespace]; !ok {
			ctx.UnAuthorized = true
			return
		} else if !authInfo.IsProjectAdmin {
			ctx.UnAuthorized = true
			return
		}
	}

	ctx.Err = permission.DeleteAllRolesInNamespace(namespace, ctx.Logger)
}
