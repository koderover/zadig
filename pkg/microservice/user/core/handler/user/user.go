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

package user

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/koderover/zadig/v2/pkg/microservice/user/core/service/permission"
	"github.com/koderover/zadig/v2/pkg/types"

	"github.com/koderover/zadig/v2/pkg/microservice/user/config"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"

	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

// Deprecated
func SyncLdapUser(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	// this is local, so we simply generate user auth info from service
	err := GenerateUserAuthInfo(ctx)
	if err != nil {
		ctx.UnAuthorized = true
		ctx.RespErr = fmt.Errorf("failed to generate user authorization info, error: %s", err)
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		ctx.UnAuthorized = true
		ctx.RespErr = fmt.Errorf("failed to generate user authorization info, error: %s", err)
		return
	}

	ldapID := c.Param("ldapId")

	ctx.RespErr = permission.SearchAndSyncUser(ldapID, ctx.Logger)
}

func CountSystemUsers(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.RespErr = permission.GetUserCount(ctx.Logger)
}

func CheckDuplicateUser(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	username := c.Query("username")

	ctx.RespErr = permission.CheckDuplicateUser(username, ctx.Logger)
}

func GetUser(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	// this is local, so we simply generate user auth info from service
	err := GenerateUserAuthInfo(ctx)
	if err != nil {
		ctx.UnAuthorized = true
		ctx.RespErr = fmt.Errorf("failed to generate user authorization info, error: %s", err)
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if ctx.UserID != c.Param("uid") {
			ctx.UnAuthorized = true
			ctx.RespErr = fmt.Errorf("failed to generate user authorization info, error: %s", err)
			return
		}
	}

	ctx.Resp, ctx.RespErr = permission.GetUser(c.Param("uid"), ctx.Logger)
}

func DeleteUser(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	// this is local, so we simply generate user auth info from service
	err := GenerateUserAuthInfo(ctx)
	if err != nil {
		ctx.UnAuthorized = true
		ctx.RespErr = fmt.Errorf("failed to generate user authorization info, error: %s", err)
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		ctx.UnAuthorized = true
		return
	}

	ctx.RespErr = permission.DeleteUserByUID(c.Param("uid"), ctx.Logger)
}

func GetPersonalUser(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	uid := c.Param("uid")
	if ctx.UserID != uid {
		ctx.RespErr = e.ErrForbidden
		return
	}
	ctx.Resp, ctx.RespErr = permission.GetUser(uid, ctx.Logger)
}

func GetUserSetting(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	uid := c.Param("uid")
	if ctx.UserID != uid {
		ctx.RespErr = e.ErrForbidden
		return
	}
	ctx.Resp, ctx.RespErr = permission.GetUserSetting(uid, ctx.Logger)
}

// @Summary 获取用户列表
// @Description 获取用户列表只需要传page和per_page参数，搜索时需要再加上name参数
// @Tags 	user
// @Accept 	json
// @Produce json
// @Param 	body 		body 		permission.QueryArgs 	  true 	"body"
// @Success 200 		{object} 	types.UsersResp
// @Router /api/v1/users/search [post]
func ListUsers(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	// this is local, so we simply generate user auth info from service
	err := GenerateUserAuthInfo(ctx)
	if err != nil {
		ctx.UnAuthorized = true
		ctx.RespErr = fmt.Errorf("failed to generate user authorization info, error: %s", err)
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		ctx.Logger.Errorf("user %s is not system admin, can not list users", ctx.UserID)
		ctx.UnAuthorized = true
		return
	}

	args := &permission.QueryArgs{}
	if err := c.ShouldBindJSON(args); err != nil {
		ctx.RespErr = err
		return
	}

	if args.Page == 0 {
		args.Page = 1
	}

	if args.PerPage == 0 {
		args.PerPage = 200
	}

	if len(args.UIDs) > 0 {
		ctx.Resp, ctx.RespErr = permission.SearchUsersByUIDs(args.UIDs, ctx.Logger)
	} else if len(args.Account) > 0 {
		if len(args.IdentityType) == 0 {
			args.IdentityType = config.SystemIdentityType
		}
		ctx.Resp, ctx.RespErr = permission.SearchUserByAccount(args, ctx.Logger)
	} else {
		ctx.Resp, ctx.RespErr = permission.SearchUsers(args, ctx.Logger)
	}
}

func OpenAPIListUsersBrief(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.UnAuthorized = true
		return
	}

	if !ctx.Resources.IsSystemAdmin {
		ctx.UnAuthorized = true
		return
	}

	// this is local, so we simply generate user auth info from service
	err = GenerateUserAuthInfo(ctx)
	if err != nil {
		ctx.UnAuthorized = true
		ctx.RespErr = fmt.Errorf("failed to generate user authorization info, error: %s", err)
		return
	}

	args := &permission.OpenAPIQueryArgs{}
	if err := c.ShouldBindQuery(args); err != nil {
		ctx.RespErr = err
		return
	}

	tarnsArg := &permission.QueryArgs{
		Page:         args.PageNum,
		PerPage:      args.PageSize,
		Account:      args.Account,
		Name:         args.Name,
		Roles:        args.Roles,
		IdentityType: args.IdentityType,
	}

	var resp *types.UsersResp
	if len(args.Account) > 0 {
		if len(tarnsArg.IdentityType) == 0 {
			tarnsArg.IdentityType = config.SystemIdentityType
		}
		resp, err = permission.SearchUserByAccount(tarnsArg, ctx.Logger)
	} else {
		resp, err = permission.SearchUsers(tarnsArg, ctx.Logger)
	}

	if err != nil {
		ctx.RespErr = err
		return
	}

	briefUserList := make([]*types.UserBriefInfo, 0)
	for _, userInfo := range resp.Users {
		briefUserList = append(briefUserList, &types.UserBriefInfo{
			UID:          userInfo.Uid,
			Account:      userInfo.Account,
			IdentityType: userInfo.IdentityType,
			Name:         userInfo.Name,
		})
	}

	ctx.Resp = &types.UsersBriefResp{
		Users:      briefUserList,
		TotalCount: resp.TotalCount,
	}
}

func ListUsersBrief(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	// this is local, so we simply generate user auth info from service
	err := GenerateUserAuthInfo(ctx)
	if err != nil {
		ctx.UnAuthorized = true
		ctx.RespErr = fmt.Errorf("failed to generate user authorization info, error: %s", err)
		return
	}

	args := &permission.QueryArgs{}
	if err := c.ShouldBindJSON(args); err != nil {
		ctx.RespErr = err
		return
	}

	if args.Page == 0 {
		args.Page = 1
	}

	if args.PerPage == 0 {
		args.PerPage = 200
	}

	var resp *types.UsersResp
	if len(args.UIDs) > 0 {
		resp, err = permission.SearchUsersByUIDs(args.UIDs, ctx.Logger)
	} else if len(args.Account) > 0 {
		if len(args.IdentityType) == 0 {
			args.IdentityType = config.SystemIdentityType
		}
		resp, err = permission.SearchUserByAccount(args, ctx.Logger)
	} else {
		resp, err = permission.SearchUsers(args, ctx.Logger)
	}

	if err != nil {
		ctx.RespErr = err
		return
	}

	briefUserList := make([]*types.UserBriefInfo, 0)
	for _, userInfo := range resp.Users {
		briefUserList = append(briefUserList, &types.UserBriefInfo{
			UID:          userInfo.Uid,
			Account:      userInfo.Account,
			IdentityType: userInfo.IdentityType,
			Name:         userInfo.Name,
		})
	}

	ctx.Resp = &types.UsersBriefResp{
		Users:      briefUserList,
		TotalCount: resp.TotalCount,
	}
}

func CreateUser(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	// this is local, so we simply generate user auth info from service
	err := GenerateUserAuthInfo(ctx)
	if err != nil {
		ctx.UnAuthorized = true
		ctx.RespErr = fmt.Errorf("failed to generate user authorization info, error: %s", err)
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		ctx.UnAuthorized = true
		return
	}

	args := &permission.User{}
	if err := c.ShouldBindJSON(args); err != nil {
		ctx.RespErr = err
		return
	}
	ctx.Resp, ctx.RespErr = permission.CreateUser(args, ctx.Logger)
}

func UpdateUser(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	// this is local, so we simply generate user auth info from service
	err := GenerateUserAuthInfo(ctx)
	if err != nil {
		ctx.UnAuthorized = true
		ctx.RespErr = fmt.Errorf("failed to generate user authorization info, error: %s", err)
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		ctx.UnAuthorized = true
		return
	}

	args := &permission.UpdateUserInfo{}
	if err := c.ShouldBindJSON(args); err != nil {
		ctx.RespErr = err
		return
	}
	uid := c.Param("uid")
	ctx.RespErr = permission.UpdateUser(uid, args, ctx.Logger)
}

func UpdatePersonalUser(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	args := &permission.UpdateUserInfo{}
	if err := c.ShouldBindJSON(args); err != nil {
		ctx.RespErr = err
		return
	}
	uid := c.Param("uid")
	if ctx.UserID != uid {
		ctx.RespErr = e.ErrForbidden
		return
	}
	ctx.RespErr = permission.UpdateUser(uid, args, ctx.Logger)
}

func UpdateUserSetting(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	args := &permission.UserSetting{}
	if err := c.ShouldBindJSON(args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}
	uid := c.Param("uid")
	if ctx.UserID != uid {
		ctx.RespErr = e.ErrForbidden
		return
	}
	ctx.RespErr = permission.UpdateUserSetting(uid, args)
}

func SignUp(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	args := &permission.User{}
	if err := c.ShouldBindJSON(args); err != nil {
		ctx.RespErr = err
		return
	}
	ctx.Resp, ctx.RespErr = permission.CreateUser(args, ctx.Logger)
}

func Retrieve(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	ctx.Resp, ctx.RespErr = permission.Retrieve(c.Query("account"), ctx.Logger)
}

func UpdatePassword(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	args := &permission.Password{}
	if err := c.ShouldBindJSON(args); err != nil {
		ctx.RespErr = err
		return
	}
	args.Uid = c.Param("uid")
	ctx.RespErr = permission.UpdatePassword(args, ctx.Logger)
}

func Reset(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	args := &permission.ResetParams{}
	if err := c.ShouldBindJSON(args); err != nil {
		ctx.RespErr = err
		return
	}
	args.Uid = ctx.UserID
	ctx.RespErr = permission.Reset(args, ctx.Logger)
}
