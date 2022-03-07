/*
Copyright 2022 The KodeRover Authors.

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

type deletePoliciesArgs struct {
	Names []string `json:"names"`
}
type createPoliciesArgs struct {
	Policies []*service.Policy `json:"policies"`
}

func CreatePolicies(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := &createPoliciesArgs{}
	if err := c.ShouldBindJSON(args); err != nil {
		ctx.Err = err
		return
	}

	projectName := c.Query("projectName")
	if projectName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("projectName is empty")
		return
	}
	ctx.Err = service.CreatePolicies(projectName, args.Policies, ctx.Logger)
}

func UpdatePolicy(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := &service.Policy{}
	if err := c.ShouldBindJSON(args); err != nil {
		ctx.Err = err
		return
	}

	projectName := c.Query("projectName")
	if projectName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("projectName is empty")
		return
	}
	name := c.Param("name")
	args.Name = name

	ctx.Err = service.UpdatePolicy(projectName, args, ctx.Logger)
}

func UpdateOrCreatePolicy(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := &service.Policy{}
	if err := c.ShouldBindJSON(args); err != nil {
		ctx.Err = err
		return
	}

	projectName := c.Query("projectName")
	if projectName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("projectName is empty")
		return
	}
	args.Name = c.Param("name")

	ctx.Err = service.UpdateOrCreatePolicy(projectName, args, ctx.Logger)
}

func UpdatePublicPolicy(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := &service.Policy{}
	if err := c.ShouldBindJSON(args); err != nil {
		ctx.Err = err
		return
	}
	name := c.Param("name")
	args.Name = name
	ctx.Err = service.UpdatePolicy(service.PresetScope, args, ctx.Logger)
}

func UpdateOrCreatePublicPolicy(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := &service.Policy{}
	if err := c.ShouldBindJSON(args); err != nil {
		ctx.Err = err
		return
	}
	name := c.Param("name")
	args.Name = name
	ctx.Err = service.UpdateOrCreatePolicy(service.PresetScope, args, ctx.Logger)
}

func ListPolicies(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	projectName := c.Query("projectName")

	ctx.Resp, ctx.Err = service.ListPolicies(projectName, ctx.Logger)
}

func GetPolicy(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	projectName := c.Query("projectName")

	ctx.Resp, ctx.Err = service.GetPolicy(projectName, c.Param("name"), ctx.Logger)
}

func GetPolicies(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = service.GetPolicies(c.Query("names"), ctx.Logger)
}

func CreatePublicPolicy(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := &service.Policy{}
	if err := c.ShouldBindJSON(args); err != nil {
		ctx.Err = err
		return
	}

	ctx.Err = service.CreatePolicy(service.PresetScope, args, ctx.Logger)
}

func ListPublicPolicies(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = service.ListPolicies(service.PresetScope, ctx.Logger)
	return
}

func GetPublicPolicy(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = service.GetPolicy(service.PresetScope, c.Param("name"), ctx.Logger)
}

func DeletePolicy(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	name := c.Param("name")
	projectName := c.Query("projectName")

	ctx.Err = service.DeletePolicy(name, projectName, ctx.Logger)
}

func DeletePolicies(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	projectName := c.Query("projectName")

	args := &deletePoliciesArgs{}
	if err := c.ShouldBindJSON(args); err != nil {
		ctx.Err = err
		return
	}

	ctx.Err = service.DeletePolicies(args.Names, projectName, ctx.Logger)
}

func DeletePublicPolicies(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	name := c.Param("name")
	ctx.Err = service.DeletePolicy(name, service.PresetScope, ctx.Logger)
	return
}

func CreateSystemPolicy(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := &service.Policy{}
	if err := c.ShouldBindJSON(args); err != nil {
		ctx.Err = err
		return
	}

	ctx.Err = service.CreatePolicy(service.SystemScope, args, ctx.Logger)
}

func UpdateOrCreateSystemPolicy(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := &service.Policy{}
	if err := c.ShouldBindJSON(args); err != nil {
		ctx.Err = err
		return
	}
	name := c.Param("name")
	args.Name = name
	ctx.Err = service.UpdateOrCreatePolicy(service.SystemScope, args, ctx.Logger)
}

func ListSystemPolicies(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = service.ListPolicies(service.SystemScope, ctx.Logger)
	return
}

func DeleteSystemPolicy(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	name := c.Param("name")
	ctx.Err = service.DeletePolicy(name, service.SystemScope, ctx.Logger)
	return
}
