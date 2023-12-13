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
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/project/service"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

// TODO: no authorization, fix this
func ListVariableSets(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	option := &service.VariableSetFindOption{}
	err := c.BindQuery(option)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	ctx.Resp, ctx.Err = service.ListVariableSets(option, ctx.Logger)
}

func GetVariableSet(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = service.GetVariableSet(c.Param("id"), ctx.Logger)
}

func CreateVariableSet(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Query("projectName")

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if projectAuthInfo, ok := ctx.Resources.ProjectAuthInfo[projectKey]; ok {
			// first check if the user is projectAdmin
			if !projectAuthInfo.IsProjectAdmin {
				ctx.UnAuthorized = true
				return
			}
		} else {
			ctx.UnAuthorized = true
			return
		}
	}

	// license checks
	err = util.CheckZadigXLicenseStatus()
	if err != nil {
		ctx.Err = err
		return
	}

	args := &service.CreateVariableSetRequest{}
	err = c.BindJSON(args)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	args.UserName = ctx.UserName
	args.ProjectName = projectKey

	ctx.Err = service.CreateVariableSet(args)
}

func UpdateVariableSet(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Query("projectName")

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if projectAuthInfo, ok := ctx.Resources.ProjectAuthInfo[projectKey]; ok {
			// first check if the user is projectAdmin
			if !projectAuthInfo.IsProjectAdmin {
				ctx.UnAuthorized = true
				return
			}
		} else {
			ctx.UnAuthorized = true
			return
		}
	}

	// license checks
	err = util.CheckZadigXLicenseStatus()
	if err != nil {
		ctx.Err = err
		return
	}

	args := &service.CreateVariableSetRequest{}
	err = c.BindJSON(args)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	args.UserName = ctx.UserName
	args.ID = c.Param("id")
	args.ProjectName = projectKey

	ctx.Err = service.UpdateVariableSet(args, ctx.RequestID, ctx.Logger)
}

func DeleteVariableSet(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Query("projectName")

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if projectAuthInfo, ok := ctx.Resources.ProjectAuthInfo[projectKey]; ok {
			// first check if the user is projectAdmin
			if !projectAuthInfo.IsProjectAdmin {
				ctx.UnAuthorized = true
				return
			}
		} else {
			ctx.UnAuthorized = true
			return
		}
	}

	// license checks
	err = util.CheckZadigXLicenseStatus()
	if err != nil {
		ctx.Err = err
		return
	}

	ctx.Err = service.DeleteVariableSet(c.Param("id"), projectKey, ctx.Logger)
}
