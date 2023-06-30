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

package handler

import (
	"bytes"
	"io"

	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/pkg/microservice/policy/core/service"
	"github.com/koderover/zadig/pkg/setting"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/log"
)

type deleteRoleBindingsArgs struct {
	Names []string `json:"names"`
}

func UpdateRoleBinding(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("UpdateRoleBinding c.GetRawData() err : %v", err)
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	projectName := c.Query("projectName")
	if projectName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("projectName is empty")
		return
	}
	args := &service.RoleBinding{}
	if err := c.ShouldBindJSON(&args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("bind json fail %s")
	}

	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, projectName, setting.OperationSceneProject, "更新", "角色绑定", projectName, string(data), ctx.Logger, args.Name)

	ctx.Err = service.UpdateOrCreateRoleBinding(projectName, args, ctx.Logger)
}

func CreateRoleBinding(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("CreateRoleBinding c.GetRawData() err : %v", err)
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	projectName := c.Query("projectName")
	if projectName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("projectName is empty")
		return
	}

	args := make([]*service.RoleBinding, 0)
	if c.Query("bulk") == "true" {
		if err := c.ShouldBindJSON(&args); err != nil {
			ctx.Err = err
			return
		}
	} else {
		rb := &service.RoleBinding{}
		if err := c.ShouldBindJSON(rb); err != nil {
			ctx.Err = err
			return
		}
		args = append(args, rb)
	}

	names := make([]string, 0)
	for _, arg := range args {
		names = append(names, arg.Name)
	}
	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, projectName, setting.OperationSceneProject, "创建", "角色绑定", projectName, string(data), ctx.Logger, names...)

	ctx.Err = service.CreateRoleBindings(projectName, args, ctx.Logger)
}

func ListRoleBindings(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	projectName := c.Query("projectName")
	if projectName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("projectName is empty")
		return
	}

	ctx.Resp, ctx.Err = service.ListRoleBindings(projectName, "", ctx.Logger)
}

func DeleteRoleBinding(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	name := c.Param("name")
	projectName := c.Query("projectName")
	if projectName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("projectName is empty")
		return
	}

	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, projectName, setting.OperationSceneProject, "删除", "角色绑定", projectName, "", ctx.Logger, name)

	ctx.Err = service.DeleteRoleBinding(name, projectName, ctx.Logger)
}

func DeleteRoleBindings(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	projectName := c.Query("projectName")
	userID := c.Query("userID")

	args := &deleteRoleBindingsArgs{}
	if err := c.ShouldBindJSON(args); err != nil {
		ctx.Err = err
		return
	}

	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, projectName, setting.OperationSceneProject, "删除", "角色绑定", projectName, "", ctx.Logger, args.Names...)

	ctx.Err = service.DeleteRoleBindings(args.Names, projectName, userID, ctx.Logger)
}

func UpdateRoleBindings(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("CreateSystemRoleBinding c.GetRawData() err : %v", err)
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	projectName := c.Query("projectName")
	if projectName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("projectName is empty")
		return
	}
	userID := c.Query("userID")
	if userID == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("userID is empty")
		return
	}
	args := make([]*service.RoleBinding, 0)
	if err := c.ShouldBindJSON(&args); err != nil {
		ctx.Err = err
		return
	}

	names := make([]string, 0)
	for _, arg := range args {
		names = append(names, arg.Name)
	}
	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, projectName, setting.OperationSceneProject, "更新", "角色绑定", projectName, string(data), ctx.Logger, names...)

	ctx.Err = service.UpdateRoleBindings(projectName, args, c.Query("userID"), ctx.Logger)
}

func UpdateSystemRoleBindings(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("CreateSystemRoleBinding c.GetRawData() err : %v", err)
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	userID := c.Query("userID")
	if userID == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("userID is empty")
		return
	}

	args := make([]*service.RoleBinding, 0)
	if err := c.ShouldBindJSON(&args); err != nil {
		ctx.Err = err
		return
	}
	var userId string
	for i, v := range args {
		if i == 0 {
			userId = v.UID
		}
		if v.UID == "" {
			ctx.Err = e.ErrInvalidParam.AddDesc("some roleBinding's uid is empty")
			return
		}

		if v.UID != userId {
			ctx.Err = e.ErrInvalidParam.AddDesc("roleBindings' uid is not consistent")
			return
		}
	}
	// TODO: mouuii args can be empty slice , this will delete admin binding , try to improve
	if userId == "" {
		userId = userID
	}

	names := make([]string, 0)
	for _, arg := range args {
		names = append(names, arg.Name)
	}
	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, "", setting.OperationSceneSystem, "更新", "系统角色绑定", "", string(data), ctx.Logger, names...)

	ctx.Err = service.UpdateRoleBindings(service.SystemScope, args, userId, ctx.Logger)
}

func DeleteSystemRoleBinding(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	name := c.Param("name")

	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, "", setting.OperationSceneSystem, "删除", "系统角色绑定", "", "", ctx.Logger, name)
	ctx.Err = service.DeleteRoleBinding(name, service.SystemScope, ctx.Logger)
}

func CreateSystemRoleBinding(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("CreateSystemRoleBinding c.GetRawData() err : %v", err)
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	args := &service.RoleBinding{}
	if err := c.ShouldBindJSON(args); err != nil {
		ctx.Err = err
		return
	}

	args.Preset = false

	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, "", setting.OperationSceneSystem, "创建", "系统角色绑定", "", string(data), ctx.Logger, args.Name)

	ctx.Err = service.CreateRoleBindings(service.SystemScope, []*service.RoleBinding{args}, ctx.Logger)
}

func CreateOrUpdateSystemRoleBinding(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("CreateOrUpdateSystemRoleBinding c.GetRawData() err : %v", err)
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	args := &service.RoleBinding{}
	if err := c.ShouldBindJSON(args); err != nil {
		ctx.Err = err
		return
	}

	args.Preset = false

	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, "", setting.OperationSceneSystem, "创建或更新", "系统角色绑定", "", string(data), ctx.Logger, args.Name)

	ctx.Err = service.CreateOrUpdateSystemRoleBinding(service.SystemScope, args, ctx.Logger)
}

func ListSystemRoleBindings(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = service.ListRoleBindings(service.SystemScope, "", ctx.Logger)
}

type SearchSystemRoleBindingArgs struct {
	Uids []string `json:"uids"`
}

func SearchSystemRoleBinding(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(SearchSystemRoleBindingArgs)
	if err := c.ShouldBindJSON(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}

	ctx.Resp, ctx.Err = service.SearchSystemRoleBindings(args.Uids, ctx.Logger)
}

func ListUserBindings(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	uid := c.Query("uid")
	if uid == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("uid is empty")
		return
	}
	projectName := c.Query("projectName")
	if projectName == "" {
		projectName = service.SystemScope
	}

	ctx.Resp, ctx.Err = service.ListRoleBindings(projectName, uid, ctx.Logger)
}
