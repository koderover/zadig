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
	"strings"

	"github.com/gin-gonic/gin"

	userhandler "github.com/koderover/zadig/v2/pkg/microservice/user/core/handler/user"
	"github.com/koderover/zadig/v2/pkg/microservice/user/core/service/permission"
	"github.com/koderover/zadig/v2/pkg/setting"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/types"
)

func OpenAPIListRoleBindings(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	err := userhandler.GenerateUserAuthInfo(ctx)
	if err != nil {
		ctx.UnAuthorized = true
		ctx.RespErr = fmt.Errorf("failed to generate user authorization info, error: %s", err)
		return
	}

	projectName := c.Query("namespace")
	if projectName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("namespace is empty")
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

	uid := c.Query("uid")
	gid := c.Query("gid")
	if uid != "" && gid != "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("cannot pass uid and gid together")
		return
	}

	ctx.Resp, ctx.RespErr = permission.ListRoleBindings(projectName, uid, gid, ctx.Logger)
}

func ListRoleBindings(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	projectName := c.Query("namespace")
	if projectName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("namespace is empty")
		return
	}

	uid := c.Query("uid")
	gid := c.Query("gid")
	if uid != "" && gid != "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("cannot pass uid and gid together")
		return
	}

	ctx.Resp, ctx.RespErr = permission.ListRoleBindings(projectName, uid, gid, ctx.Logger)
}

type createRoleBindingReq struct {
	Identities []*types.Identity `json:"identities"`
	Role       string            `json:"role"`
}

func OpenAPICreateRoleBinding(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	ctx.UserName = ctx.UserName + "(openAPI)"
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	CreateRoleBindingImpl(c, ctx)
}

func CreateRoleBinding(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	CreateRoleBindingImpl(c, ctx)
}

func CreateRoleBindingImpl(c *gin.Context, ctx *internalhandler.Context) {
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("CreateRoleBinding c.GetRawData() err : %v", err)
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	projectName := c.Query("namespace")
	if projectName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("namespace is empty")
		return
	}

	req := new(createRoleBindingReq)
	if err := c.ShouldBindJSON(&req); err != nil {
		ctx.RespErr = err
		return
	}

	detail := ""
	detailEn := ""
	for _, arg := range req.Identities {
		if arg.IdentityType == "user" {
			userInfo, err := permission.GetUser(arg.UID, ctx.Logger)
			if err != nil {
				ctx.RespErr = e.ErrInvalidParam.AddErr(err)
				return
			}
			username := ""
			if userInfo != nil {
				username = userInfo.Name
			}
			detail += "用户：" + username + "，"
			detailEn += "User: " + username + ", "
		} else if arg.IdentityType == "group" {
			groupInfo, err := permission.GetUserGroup(arg.GID, ctx.Logger)
			if err != nil {
				ctx.RespErr = e.ErrInvalidParam.AddErr(err)
				return
			}
			username := ""
			if groupInfo != nil {
				username = groupInfo.Name
			}
			detail += "用户组：" + username + "，"
			detailEn += "User Group: " + username + ", "
		}
	}
	detail += "角色名称：" + req.Role + "\n"
	detailEn += "Role Name: " + req.Role + "\n"
	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, projectName, setting.OperationSceneProject, "创建", "角色绑定", detail, detailEn, string(data), types.RequestBodyTypeJSON, ctx.Logger, "")

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

	ctx.RespErr = permission.CreateRoleBindings(req.Role, projectName, req.Identities, ctx.Logger)
}

type updateRoleBindingForUserReq struct {
	Roles []string `json:"roles"`
}

func OpenAPIUpdateRoleBindingForUser(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	ctx.UserName = ctx.UserName + "(openAPI)"
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	UpdateRoleBindingForUserImpl(c, ctx)
}

func UpdateRoleBindingForUser(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	UpdateRoleBindingForUserImpl(c, ctx)
}

func UpdateRoleBindingForUserImpl(c *gin.Context, ctx *internalhandler.Context) {

	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("CreateSystemRoleBinding c.GetRawData() err : %v", err)
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	projectName := c.Query("namespace")
	if projectName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("namespace is empty")
		return
	}
	userID := c.Param("uid")
	if userID == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("uid is empty")
		return
	}
	args := new(updateRoleBindingForUserReq)
	if err := c.ShouldBindJSON(&args); err != nil {
		ctx.RespErr = err
		return
	}

	userInfo, err := permission.GetUser(userID, ctx.Logger)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}

	username := ""
	if userInfo != nil {
		username = userInfo.Name
	}
	detail := "用户：" + username + "，角色名称：" + strings.Join(args.Roles, "， ")
	detailEn := "User: " + username + ", Role Name: " + strings.Join(args.Roles, ", ")
	detail = strings.Trim(detail, "，")
	detailEn = strings.Trim(detailEn, ", ")

	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, projectName, setting.OperationSceneProject, "更新", "角色绑定", detail, detailEn, string(data), types.RequestBodyTypeJSON, ctx.Logger, "")

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

	ctx.RespErr = permission.UpdateRoleBindingForUser(userID, projectName, args.Roles, ctx.Logger)
}

func OpenAPIDeleteRoleBindingForUser(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	ctx.UserName = ctx.UserName + "(openAPI)"
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	DeleteRoleBindingForUserImpl(c, ctx)
}

func DeleteRoleBindingForUser(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	DeleteRoleBindingForUserImpl(c, ctx)
}

func DeleteRoleBindingForUserImpl(c *gin.Context, ctx *internalhandler.Context) {

	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("CreateSystemRoleBinding c.GetRawData() err : %v", err)
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	projectName := c.Query("namespace")
	if projectName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("namespace is empty")
		return
	}
	userID := c.Param("uid")
	if userID == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("uid is empty")
		return
	}

	userInfo, err := permission.GetUser(userID, ctx.Logger)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}

	username := ""
	if userInfo != nil {
		username = userInfo.Name
	}
	detail := "用户：" + username
	detailEn := "User: " + username
	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, projectName, setting.OperationSceneProject, "删除", "角色绑定", detail, detailEn, string(data), types.RequestBodyTypeJSON, ctx.Logger, "")

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

	ctx.RespErr = permission.DeleteRoleBindingForUser(userID, projectName, ctx.Logger)
}

func OpenAPIUpdateRoleBindingForGroup(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	ctx.UserName = ctx.UserName + "(openAPI)"
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	UpdateRoleBindingForGroupImpl(c, ctx)
}

func UpdateRoleBindingForGroup(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	UpdateRoleBindingForGroupImpl(c, ctx)
}

func UpdateRoleBindingForGroupImpl(c *gin.Context, ctx *internalhandler.Context) {
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("CreateSystemRoleBinding c.GetRawData() err : %v", err)
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	projectName := c.Query("namespace")
	if projectName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("namespace is empty")
		return
	}
	groupID := c.Param("gid")
	if groupID == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("gid is empty")
		return
	}
	args := new(updateRoleBindingForUserReq)
	if err := c.ShouldBindJSON(&args); err != nil {
		ctx.RespErr = err
		return
	}

	groupInfo, err := permission.GetUserGroup(groupID, ctx.Logger)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}

	groupName := ""
	if groupInfo != nil {
		groupName = groupInfo.Name
	}
	detail := "用户组：" + groupName + "，角色名称：" + strings.Join(args.Roles, "， ")
	detailEn := "User Group: " + groupName + ", Role Name: " + strings.Join(args.Roles, ", ")
	detail = strings.Trim(detail, "，")
	detailEn = strings.Trim(detailEn, ", ")

	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, projectName, setting.OperationSceneProject, "更新", "角色绑定", detail, detailEn, string(data), types.RequestBodyTypeJSON, ctx.Logger, "")

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

	ctx.RespErr = permission.UpdateRoleBindingForUserGroup(groupID, projectName, args.Roles, ctx.Logger)
}

func OpenAPIDeleteRoleBindingForGroup(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	ctx.UserName = ctx.UserName + "(openAPI)"
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}
	DeleteRoleBindingForGroupImpl(c, ctx)
}

func DeleteRoleBindingForGroup(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}
	DeleteRoleBindingForGroupImpl(c, ctx)
}

func DeleteRoleBindingForGroupImpl(c *gin.Context, ctx *internalhandler.Context) {

	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("CreateSystemRoleBinding c.GetRawData() err : %v", err)
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	projectName := c.Query("namespace")
	if projectName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("namespace is empty")
		return
	}
	groupID := c.Param("gid")
	if groupID == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("gid is empty")
		return
	}

	groupInfo, err := permission.GetUserGroup(groupID, ctx.Logger)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}

	groupName := ""
	if groupInfo != nil {
		groupName = groupInfo.Name
	}
	detail := "用户组：" + groupName
	detailEn := "User Group: " + groupName

	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, projectName, setting.OperationSceneProject, "删除", "角色绑定", detail, detailEn, string(data), types.RequestBodyTypeJSON, ctx.Logger, "")

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

	ctx.RespErr = permission.DeleteRoleBindingForUserGroup(groupID, projectName, ctx.Logger)
}
