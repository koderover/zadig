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
	"bytes"
	"fmt"
	"io"

	"github.com/gin-gonic/gin"

	userhandler "github.com/koderover/zadig/pkg/microservice/user/core/handler/user"
	"github.com/koderover/zadig/pkg/microservice/user/core/service/permission"
	"github.com/koderover/zadig/pkg/setting"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/log"
)

func CreateRole(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("CreateRole c.GetRawData() err : %v", err)
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	args := &permission.CreateRoleReq{}
	if err := c.ShouldBindJSON(args); err != nil {
		ctx.Err = err
		return
	}

	projectName := c.Query("namespace")
	if projectName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("namespace is empty")
		return
	}

	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, projectName, setting.OperationSceneProject, "创建", "角色", "角色名称："+args.Name, string(data), ctx.Logger, args.Name)

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if projectName == "*" {
			ctx.UnAuthorized = true
			return
		}
		if authInfo, ok := ctx.Resources.ProjectAuthInfo[projectName]; !ok {
			ctx.UnAuthorized = true
			return
		} else if !authInfo.IsProjectAdmin {
			ctx.UnAuthorized = true
			return
		}
	}

	ctx.Err = permission.CreateRole(projectName, args, ctx.Logger)
}

func UpdateRole(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("UpdateRole c.GetRawData() err : %v", err)
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	args := &permission.CreateRoleReq{}
	if err := c.ShouldBindJSON(args); err != nil {
		ctx.Err = err
		return
	}

	projectName := c.Query("namespace")
	if projectName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("namespace is empty")
		return
	}
	name := c.Param("name")
	args.Name = name

	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, projectName, setting.OperationSceneProject, "更新", "角色", "角色名称："+args.Name, string(data), ctx.Logger, args.Name)

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if projectName == "*" {
			ctx.UnAuthorized = true
			return
		}
		if authInfo, ok := ctx.Resources.ProjectAuthInfo[projectName]; !ok {
			ctx.UnAuthorized = true
			return
		} else if !authInfo.IsProjectAdmin {
			ctx.UnAuthorized = true
			return
		}
	}

	ctx.Err = permission.UpdateRole(projectName, args, ctx.Logger)
}

func ListRoles(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	projectName := c.Query("namespace")
	if projectName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("args namespace can't be empty")
		return
	}
	uid := c.Query("uid")
	if uid == "" {
		ctx.Resp, ctx.Err = permission.ListRolesByNamespace(projectName, ctx.Logger)
	} else {
		ctx.Resp, ctx.Err = permission.ListRolesByNamespaceAndUserID(projectName, uid, ctx.Logger)
	}
}

func GetRole(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	projectName := c.Query("namespace")
	if projectName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("args namespace can't be empty")
		return
	}

	ctx.Resp, ctx.Err = permission.GetRole(projectName, c.Param("name"), ctx.Logger)
}

func DeleteRole(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	name := c.Param("name")
	projectName := c.Query("namespace")
	if projectName == "" {
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
		if projectName == "*" {
			ctx.UnAuthorized = true
			return
		}
		if authInfo, ok := ctx.Resources.ProjectAuthInfo[projectName]; !ok {
			ctx.UnAuthorized = true
			return
		} else if !authInfo.IsProjectAdmin {
			ctx.UnAuthorized = true
			return
		}
	}

	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, projectName, setting.OperationSceneProject, "删除", "角色", "角色名称："+name, "", ctx.Logger, name)

	ctx.Err = permission.DeleteRole(name, projectName, ctx.Logger)
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
