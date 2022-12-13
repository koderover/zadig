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
	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/pkg/microservice/user/config"
	"github.com/koderover/zadig/pkg/microservice/user/core/service/user"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"

	e "github.com/koderover/zadig/pkg/tool/errors"
)

func SyncLdapUser(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	ldapID := c.Param("ldapId")

	ctx.Err = user.SearchAndSyncUser(ldapID, ctx.Logger)
}

func CountSystemUsers(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = user.GetUserCount(ctx.Logger)
}

func GetUser(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	ctx.Resp, ctx.Err = user.GetUser(c.Param("uid"), ctx.Logger)
}

func DeleteUser(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	ctx.Err = user.DeleteUserByUID(c.Param("uid"), ctx.Logger)
}

func GetPersonalUser(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	uid := c.Param("uid")
	if ctx.UserID != uid {
		ctx.Err = e.ErrForbidden
		return
	}
	ctx.Resp, ctx.Err = user.GetUser(uid, ctx.Logger)
}

func GetUserSetting(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	uid := c.Param("uid")
	if ctx.UserID != uid {
		ctx.Err = e.ErrForbidden
		return
	}
	ctx.Resp, ctx.Err = user.GetUserSetting(uid, ctx.Logger)
}

func ListUsers(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	args := &user.QueryArgs{}
	if err := c.ShouldBindJSON(args); err != nil {
		ctx.Err = err
		return
	}
	if len(args.UIDs) > 0 {
		ctx.Resp, ctx.Err = user.SearchUsersByUIDs(args.UIDs, ctx.Logger)
	} else if len(args.Account) > 0 {
		if len(args.IdentityType) == 0 {
			args.IdentityType = config.SystemIdentityType
		}
		ctx.Resp, ctx.Err = user.SearchUserByAccount(args, ctx.Logger)
	} else {
		ctx.Resp, ctx.Err = user.SearchUsers(args, ctx.Logger)
	}
}

func CreateUser(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	args := &user.User{}
	if err := c.ShouldBindJSON(args); err != nil {
		ctx.Err = err
		return
	}
	ctx.Resp, ctx.Err = user.CreateUser(args, ctx.Logger)
}

func UpdateUser(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	args := &user.UpdateUserInfo{}
	if err := c.ShouldBindJSON(args); err != nil {
		ctx.Err = err
		return
	}
	uid := c.Param("uid")
	ctx.Err = user.UpdateUser(uid, args, ctx.Logger)
}

func UpdatePersonalUser(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	args := &user.UpdateUserInfo{}
	if err := c.ShouldBindJSON(args); err != nil {
		ctx.Err = err
		return
	}
	uid := c.Param("uid")
	if ctx.UserID != uid {
		ctx.Err = e.ErrForbidden
		return
	}
	ctx.Err = user.UpdateUser(uid, args, ctx.Logger)
}

func UpdateUserSetting(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	args := &user.UserSetting{}
	if err := c.ShouldBindJSON(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}
	uid := c.Param("uid")
	if ctx.UserID != uid {
		ctx.Err = e.ErrForbidden
		return
	}
	ctx.Err = user.UpdateUserSetting(uid, args)
}

func SignUp(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	args := &user.User{}
	if err := c.ShouldBindJSON(args); err != nil {
		ctx.Err = err
		return
	}
	ctx.Resp, ctx.Err = user.CreateUser(args, ctx.Logger)
}

func Retrieve(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	ctx.Resp, ctx.Err = user.Retrieve(c.Query("account"), ctx.Logger)
}

func UpdatePassword(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	args := &user.Password{}
	if err := c.ShouldBindJSON(args); err != nil {
		ctx.Err = err
		return
	}
	args.Uid = c.Param("uid")
	ctx.Err = user.UpdatePassword(args, ctx.Logger)
}

func Reset(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	args := &user.ResetParams{}
	if err := c.ShouldBindJSON(args); err != nil {
		ctx.Err = err
		return
	}
	args.Uid = ctx.UserID
	ctx.Err = user.Reset(args, ctx.Logger)
}
