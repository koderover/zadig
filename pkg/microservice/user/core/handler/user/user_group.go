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

package user

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/koderover/zadig/pkg/microservice/user/core/service/user"

	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

type createUserGroupReq struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	UIDs        []string `json:"uids"`
}

func CreateUserGroup(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	// this is local, so we simply generate user auth info from service
	err := GenerateUserAuthInfo(ctx)
	if err != nil {
		ctx.UnAuthorized = true
		ctx.Err = fmt.Errorf("failed to generate user authorization info, error: %s", err)
		return
	}

	// user needs to be an admin to create user groups
	if !ctx.Resources.IsSystemAdmin {
		ctx.Logger.Errorf("user %s is not system admin, cannot create user groups", ctx.UserID)
		ctx.UnAuthorized = true
		return
	}

	req := new(createUserGroupReq)
	err = c.BindJSON(req)
	if err != nil {
		ctx.Err = e.ErrInvalidParam
		return
	}

	ctx.Err = user.CreateUserGroup(req.Name, req.Description, req.UIDs, ctx.Logger)
}

type listUserGroupsReq struct {
	PageNum  int `json:"page_num"  form:"page_num"`
	PageSize int `json:"page_size" form:"page_size"`
}

func ListUserGroups(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	// everyone can list user groups
	query := new(listUserGroupsReq)
	err := c.BindQuery(&query)
	if err != nil {
		ctx.Err = e.ErrInvalidParam
		return
	}

	ctx.Resp, ctx.Err = user.ListUserGroups(query.PageNum, query.PageSize, ctx.Logger)
}

func GetUserGroup(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	// this is local, so we simply generate user auth info from service
	err := GenerateUserAuthInfo(ctx)
	if err != nil {
		ctx.UnAuthorized = true
		ctx.Err = fmt.Errorf("failed to generate user authorization info, error: %s", err)
		return
	}

	// user needs to be an admin to see user group details
	if !ctx.Resources.IsSystemAdmin {
		ctx.Logger.Errorf("user %s is not system admin, cannot get user group detail", ctx.UserID)
		ctx.UnAuthorized = true
		return
	}

	groupID := c.Param("id")

	ctx.Resp, ctx.Err = user.GetUserGroup(groupID, ctx.Logger)
}

func UpdateUserGroupInfo(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	// this is local, so we simply generate user auth info from service
	err := GenerateUserAuthInfo(ctx)
	if err != nil {
		ctx.UnAuthorized = true
		ctx.Err = fmt.Errorf("failed to generate user authorization info, error: %s", err)
		return
	}

	// user needs to be an admin to see update user groups
	if !ctx.Resources.IsSystemAdmin {
		ctx.Logger.Errorf("user %s is not system admin, cannot update user groups", ctx.UserID)
		ctx.UnAuthorized = true
		return
	}

	groupID := c.Param("id")

	req := new(createUserGroupReq)
	if err := c.BindJSON(req); err != nil {
		ctx.Err = e.ErrInvalidParam
		return
	}

	ctx.Err = user.UpdateUserGroupInfo(groupID, req.Name, req.Description, ctx.Logger)
}

func DeleteUserGroup(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	// this is local, so we simply generate user auth info from service
	err := GenerateUserAuthInfo(ctx)
	if err != nil {
		ctx.UnAuthorized = true
		ctx.Err = fmt.Errorf("failed to generate user authorization info, error: %s", err)
		return
	}

	// user needs to be an admin to see update user groups
	if !ctx.Resources.IsSystemAdmin {
		ctx.Logger.Errorf("user %s is not system admin, cannot delete user groups", ctx.UserID)
		ctx.UnAuthorized = true
		return
	}

	groupID := c.Param("id")

	ctx.Err = user.DeleteUserGroup(groupID, ctx.Logger)
}

type bulkUserReq struct {
	UIDs []string `json:"uids"`
}

func BulkAddUserToUserGroup(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	// this is local, so we simply generate user auth info from service
	err := GenerateUserAuthInfo(ctx)
	if err != nil {
		ctx.UnAuthorized = true
		ctx.Err = fmt.Errorf("failed to generate user authorization info, error: %s", err)
		return
	}

	// user needs to be an admin to see update user groups
	if !ctx.Resources.IsSystemAdmin {
		ctx.Logger.Errorf("user %s is not system admin, cannot delete user groups", ctx.UserID)
		ctx.UnAuthorized = true
		return
	}

	groupID := c.Param("id")
	req := new(bulkUserReq)
	err = c.BindJSON(&req)
	if err != nil {
		ctx.Err = e.ErrInvalidParam
		return
	}

	ctx.Err = user.BulkAddUserToUserGroup(groupID, req.UIDs, ctx.Logger)
}

func BulkRemoveUserFromUserGroup(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	// this is local, so we simply generate user auth info from service
	err := GenerateUserAuthInfo(ctx)
	if err != nil {
		ctx.UnAuthorized = true
		ctx.Err = fmt.Errorf("failed to generate user authorization info, error: %s", err)
		return
	}

	// user needs to be an admin to see update user groups
	if !ctx.Resources.IsSystemAdmin {
		ctx.Logger.Errorf("user %s is not system admin, cannot delete user groups", ctx.UserID)
		ctx.UnAuthorized = true
		return
	}

	groupID := c.Param("id")
	req := new(bulkUserReq)
	err = c.BindJSON(&req)
	if err != nil {
		ctx.Err = e.ErrInvalidParam
		return
	}

	ctx.Err = user.BulkRemoveUserFromUserGroup(groupID, req.UIDs, ctx.Logger)
}
