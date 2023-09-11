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
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/koderover/zadig/pkg/microservice/picket/core/filter/service"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

// TODO: move this back to aslan, picket for filtering is no longer required
func ListProjects(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = service.ListProjects(c.Request.Header, c.Request.URL.Query(), ctx.Logger)
}

func CreateProject(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := &service.CreateProjectArgs{}
	if err := c.ShouldBindBodyWith(args, binding.JSON); err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err).AddDesc("invalid CreateProjectReq")
		return
	}

	// authorization checks
	permitted := false

	if ctx.Resources.IsSystemAdmin {
		permitted = true
	} else if ctx.Resources.SystemActions.Project.Create {
		permitted = true
	}

	if !permitted {
		ctx.UnAuthorized = true
		return
	}

	body := getReqBody(c)

	ctx.Resp, ctx.Err = service.CreateProject(c.Request.Header, body, c.Request.URL.Query(), args, ctx.Logger)
}

func getReqBody(c *gin.Context) (body []byte) {
	if cb, ok := c.Get(gin.BodyBytesKey); ok {
		if cbb, ok := cb.([]byte); ok {
			body = cbb
		}
	}
	return body
}

type UpdateProjectReq struct {
	Public      bool   `json:"public"`
	ProductName string `json:"product_name"`
}

func UpdateProject(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := new(UpdateProjectReq)
	if err := c.ShouldBindBodyWith(args, binding.JSON); err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err).AddDesc("invalid UpdateProject")
		return
	}

	// authorization checks
	permitted := false

	if ctx.Resources.IsSystemAdmin {
		permitted = true
	}

	if projectedAuthInfo, ok := ctx.Resources.ProjectAuthInfo[args.ProductName]; ok {
		if projectedAuthInfo.IsProjectAdmin {
			permitted = true
		}
	}

	if !permitted {
		ctx.UnAuthorized = true
		return
	}

	body := getReqBody(c)
	ctx.Resp, ctx.Err = service.UpdateProject(c.Request.Header, c.Request.URL.Query(), body, args.ProductName, args.Public, ctx.Logger)
}

func DeleteProject(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Param("name")

	// authorization checks
	permitted := false

	if ctx.Resources.IsSystemAdmin {
		permitted = true
	}

	if projectedAuthInfo, ok := ctx.Resources.ProjectAuthInfo[projectKey]; ok {
		if projectedAuthInfo.IsProjectAdmin {
			permitted = true
		}
	}

	if !permitted {
		ctx.UnAuthorized = true
		return
	}

	ctx.Resp, ctx.Err = service.DeleteProject(c.Request.Header, c.Request.URL.Query(), projectKey, ctx.Logger)
}
