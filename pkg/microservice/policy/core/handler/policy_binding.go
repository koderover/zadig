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
	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/pkg/microservice/policy/core/service"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

type deletePolicyBindingsArgs struct {
	Names []string `json:"names"`
}

func UpdatePolicyBinding(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	projectName := c.Query("projectName")
	if projectName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("projectName is empty")
		return
	}
	args := &service.PolicyBinding{}
	if err := c.ShouldBindJSON(&args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("bind json fail %s")
	}
	ctx.Err = service.UpdateOrCreatePolicyBinding(projectName, args, ctx.Logger)
}

func CreatePolicyBinding(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	projectName := c.Query("projectName")
	if projectName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("projectName is empty")
		return
	}

	args := make([]*service.PolicyBinding, 0)
	if c.Query("bulk") == "true" {
		if err := c.ShouldBindJSON(&args); err != nil {
			ctx.Err = err
			return
		}
	} else {
		rb := &service.PolicyBinding{}
		if err := c.ShouldBindJSON(rb); err != nil {
			ctx.Err = err
			return
		}
		args = append(args, rb)
	}

	ctx.Err = service.CreatePolicyBindings(projectName, args, ctx.Logger)
}

func ListPolicyBindings(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	projectName := c.Query("projectName")
	if projectName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("projectName is empty")
		return
	}

	ctx.Resp, ctx.Err = service.ListPolicyBindings(projectName, "", ctx.Logger)
}

func DeletePolicyBinding(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	name := c.Param("name")
	projectName := c.Query("projectName")
	if projectName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("projectName is empty")
		return
	}

	ctx.Err = service.DeletePolicyBinding(name, projectName, ctx.Logger)
}

func DeletePolicyBindings(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	projectName := c.Query("projectName")
	userID := c.Query("userID")

	args := &deletePolicyBindingsArgs{}
	if err := c.ShouldBindJSON(args); err != nil {
		ctx.Err = err
		return
	}

	ctx.Err = service.DeletePolicyBindings(args.Names, projectName, userID, ctx.Logger)
}

func DeleteSystemPolicyBinding(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	name := c.Param("name")
	ctx.Err = service.DeletePolicyBinding(name, service.SystemScope, ctx.Logger)
}

func CreateSystemPolicyBinding(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := &service.PolicyBinding{}
	if err := c.ShouldBindJSON(args); err != nil {
		ctx.Err = err
		return
	}

	args.Public = false

	ctx.Err = service.CreatePolicyBindings(service.SystemScope, []*service.PolicyBinding{args}, ctx.Logger)
}

func CreateOrUpdateSystemPolicyBinding(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := &service.PolicyBinding{}
	if err := c.ShouldBindJSON(args); err != nil {
		ctx.Err = err
		return
	}

	args.Public = false

	ctx.Err = service.CreateOrUpdateSystemPolicyBinding(service.SystemScope, args, ctx.Logger)
}

func ListSystemPolicyBindings(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = service.ListPolicyBindings(service.SystemScope, "", ctx.Logger)
}

func ListUserPolicyBindings(c *gin.Context) {
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

	ctx.Resp, ctx.Err = service.ListPolicyBindings(projectName, uid, ctx.Logger)
}
