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
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/microservice/user/core/service/permission"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

type createUserGroupReq struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	UIDs        []string `json:"uids"`
}

// @Summary 创建用户组
// @Description 创建用户组
// @Tags 	user
// @Accept 	json
// @Produce json
// @Param 	body 			body 		createUserGroupReq 	true 	"body"
// @Success 200             {object} models.UserGroup
// @Router /api/v1/user-group [post]
func CreateUserGroup(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	// this is local, so we simply generate user auth info from service
	err := GenerateUserAuthInfo(ctx)
	if err != nil {
		ctx.UnAuthorized = true
		ctx.RespErr = fmt.Errorf("failed to generate user authorization info, error: %s", err)
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
		ctx.RespErr = e.ErrInvalidParam
		return
	}

	if err = commonutil.CheckZadigEnterpriseLicense(); err != nil {
		ctx.RespErr = err
		return
	}

	ctx.Resp, ctx.RespErr = permission.CreateUserGroup(req.Name, req.Description, req.UIDs, ctx.Logger)
}

type listUserGroupsReq struct {
	PageNum  int    `json:"page_num"  form:"page_num"`
	PageSize int    `json:"page_size" form:"page_size"`
	Name     string `json:"name"      form:"name"`
	Uid      string `json:"uid"       form:"uid"`
}

type openAPIListUserGroupReq struct {
	PageNum  int    `json:"page_num"  form:"pageNum"`
	PageSize int    `json:"page_size" form:"pageSize"`
	Name     string `json:"name"      form:"name"`
}

type listUserGroupResp struct {
	GroupList []*permission.UserGroupResp `json:"group_list"`
	Count     int64                       `json:"total"`
}

func OpenApiListUserGroups(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	err := GenerateUserAuthInfo(ctx)
	if err != nil {
		ctx.UnAuthorized = true
		ctx.RespErr = fmt.Errorf("failed to generate user authorization info, error: %s", err)
		return
	}

	if !ctx.Resources.IsSystemAdmin {
		ctx.UnAuthorized = true
		return
	}

	// everyone can list user groups
	query := new(openAPIListUserGroupReq)
	err = c.BindQuery(&query)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam
		return
	}

	err = commonutil.CheckZadigEnterpriseLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	groupList, count, err := permission.ListUserGroups(query.Name, query.PageNum, query.PageSize, ctx.Logger)
	if err != nil {
		ctx.RespErr = err
		return
	}

	ctx.Resp = &listUserGroupResp{
		GroupList: groupList,
		Count:     count,
	}
}

func ListUserGroups(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	// everyone can list user groups
	query := new(listUserGroupsReq)
	err := c.BindQuery(&query)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam
		return
	}

	if query.PageNum == 0 {
		query.PageNum = 1
	}

	if query.PageSize == 0 {
		query.PageSize = 200
	}

	if len(query.Uid) > 0 {
		groupList, count, err := permission.ListUserGroupsByUid(query.Uid, ctx.Logger)

		if err != nil {
			ctx.RespErr = err
			return
		}

		ctx.Resp = &listUserGroupResp{
			GroupList: groupList,
			Count:     count,
		}
	} else {
		groupList, count, err := permission.ListUserGroups(query.Name, query.PageNum, query.PageSize, ctx.Logger)

		if err != nil {
			ctx.RespErr = err
			return
		}

		ctx.Resp = &listUserGroupResp{
			GroupList: groupList,
			Count:     count,
		}
	}
}

func GetUserGroup(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	// this is local, so we simply generate user auth info from service
	err := GenerateUserAuthInfo(ctx)
	if err != nil {
		ctx.UnAuthorized = true
		ctx.RespErr = fmt.Errorf("failed to generate user authorization info, error: %s", err)
		return
	}

	// user needs to be an admin to see user group details
	if !ctx.Resources.IsSystemAdmin {
		ctx.Logger.Errorf("user %s is not system admin, cannot get user group detail", ctx.UserID)
		ctx.UnAuthorized = true
		return
	}

	groupID := c.Param("id")

	ctx.Resp, ctx.RespErr = permission.GetUserGroup(groupID, ctx.Logger)
}

func UpdateUserGroupInfo(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	// this is local, so we simply generate user auth info from service
	err := GenerateUserAuthInfo(ctx)
	if err != nil {
		ctx.UnAuthorized = true
		ctx.RespErr = fmt.Errorf("failed to generate user authorization info, error: %s", err)
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
		ctx.RespErr = e.ErrInvalidParam
		return
	}

	if err = commonutil.CheckZadigEnterpriseLicense(); err != nil {
		ctx.RespErr = err
		return
	}

	ctx.RespErr = permission.UpdateUserGroupInfo(groupID, req.Name, req.Description, ctx.Logger)
}

func DeleteUserGroup(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	// this is local, so we simply generate user auth info from service
	err := GenerateUserAuthInfo(ctx)
	if err != nil {
		ctx.UnAuthorized = true
		ctx.RespErr = fmt.Errorf("failed to generate user authorization info, error: %s", err)
		return
	}

	// user needs to be an admin to see update user groups
	if !ctx.Resources.IsSystemAdmin {
		ctx.Logger.Errorf("user %s is not system admin, cannot delete user groups", ctx.UserID)
		ctx.UnAuthorized = true
		return
	}

	groupID := c.Param("id")

	ctx.RespErr = permission.DeleteUserGroup(groupID, ctx.Logger)
}

type bulkUserReq struct {
	UIDs []string `json:"uids"`
}

// @Summary 添加用户到用户组
// @Description 添加用户到用户组
// @Tags 	user
// @Accept 	json
// @Produce json
// @Param 	id 				path 		string 			true 	"用户组ID"
// @Param 	body 			body 		bulkUserReq 	true 	"body"
// @Success 200
// @Router /api/v1/user-group/{id}/bulk-create-users [post]
func BulkAddUserToUserGroup(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	// this is local, so we simply generate user auth info from service
	err := GenerateUserAuthInfo(ctx)
	if err != nil {
		ctx.UnAuthorized = true
		ctx.RespErr = fmt.Errorf("failed to generate user authorization info, error: %s", err)
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
		ctx.RespErr = e.ErrInvalidParam
		return
	}

	if err = commonutil.CheckZadigEnterpriseLicense(); err != nil {
		ctx.RespErr = err
		return
	}

	ctx.RespErr = permission.BulkAddUserToUserGroup(groupID, req.UIDs, ctx.Logger)
}

func BulkRemoveUserFromUserGroup(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	// this is local, so we simply generate user auth info from service
	err := GenerateUserAuthInfo(ctx)
	if err != nil {
		ctx.UnAuthorized = true
		ctx.RespErr = fmt.Errorf("failed to generate user authorization info, error: %s", err)
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
		ctx.RespErr = e.ErrInvalidParam
		return
	}

	if err = commonutil.CheckZadigEnterpriseLicense(); err != nil {
		ctx.RespErr = err
		return
	}

	ctx.RespErr = permission.BulkRemoveUserFromUserGroup(groupID, req.UIDs, ctx.Logger)
}
