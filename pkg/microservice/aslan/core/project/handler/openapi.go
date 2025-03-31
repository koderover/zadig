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
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/project/service"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/types"
)

func OpenAPICreateProductTemplate(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	if !ctx.Resources.IsSystemAdmin {
		ctx.UnAuthorized = true
		return
	}

	args := new(service.OpenAPICreateProductReq)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("CreateProductTemplate c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("CreateProductTemplate json.Unmarshal err : %v", err)
	}
	internalhandler.InsertOperationLog(c, ctx.UserName+"(openAPI)", args.ProjectName, "新增", "项目管理-项目", args.ProjectName, string(data), types.RequestBodyTypeJSON, ctx.Logger)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	if err := c.BindJSON(args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid ProductTmpl json args")
		return
	}

	// input validation for OpenAPI
	err = args.Validate()
	if err != nil {
		ctx.RespErr = err
		return
	}

	// finally, we create the project
	ctx.RespErr = service.CreateProjectOpenAPI(ctx.UserID, ctx.UserName, args, ctx.Logger)
}

// @Summary OpenAPI Initialize Yaml Project
// @Description OpenAPI Initialize Yaml Project
// @Tags 	OpenAPI
// @Accept 	json
// @Produce json
// @Param 	body 			body 		service.OpenAPIInitializeProjectReq 	true 	"body"
// @Success 200
// @Router /openapi/projects/project/init/yaml [post]
func OpenAPIInitializeYamlProject(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	if !ctx.Resources.IsSystemAdmin {
		ctx.UnAuthorized = true
		return
	}

	args := new(service.OpenAPIInitializeProjectReq)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("Initialize project c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("Initialize project json.Unmarshal err : %v", err)
	}
	internalhandler.InsertOperationLog(c, ctx.UserName+"(openAPI)", args.ProjectName, "初始化", "项目管理-k8s项目", args.ProjectName, string(data), types.RequestBodyTypeJSON, ctx.Logger)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	// input validation for OpenAPI
	err = args.Validate()
	if err != nil {
		ctx.RespErr = err
		return
	}

	ctx.RespErr = service.InitializeYAMLProject(ctx.UserID, ctx.UserName, ctx.RequestID, args, ctx.Logger)
}

func OpenAPIInitializeHelmProject(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	if !ctx.Resources.IsSystemAdmin {
		ctx.UnAuthorized = true
		return
	}

	args := new(service.OpenAPIInitializeProjectReq)
	data, err := c.GetRawData()
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid InitializeHelmProject params")
		return
	}
	if err = json.Unmarshal(data, args); err != nil {
		ctx.RespErr = err
		return
	}
	internalhandler.InsertOperationLog(c, ctx.UserName+"(openAPI)", args.ProjectName, "OpenAPI"+"初始化", "项目管理-helm项目", args.ProjectName, string(data), types.RequestBodyTypeJSON, ctx.Logger)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	// input validation for OpenAPI
	err = args.Validate()
	if err != nil {
		ctx.RespErr = err
		return
	}

	ctx.RespErr = service.OpenAPIInitializeHelmProject(ctx.UserID, ctx.UserName, ctx.RequestID, args, ctx.Logger)
}

func OpenAPIListProject(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	var authorizedProjectList []string

	if ctx.Resources.IsSystemAdmin {
		authorizedProjectList = []string{}
	} else {
		var found bool
		authorizedProjectList, found, err = internalhandler.ListAuthorizedProjects(ctx.UserID)
		if err != nil {
			ctx.RespErr = e.ErrInternalError.AddDesc(err.Error())
			return
		}

		if !found {
			ctx.Resp = &projectResp{
				Projects: []string{},
				Total:    0,
			}
			return
		}
	}

	args := new(service.OpenAPIListProjectReq)
	if err := c.ShouldBindQuery(args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid ListProjectOpenAPI params")
		return
	}

	ctx.Resp, ctx.RespErr = service.ListProjectOpenAPI(authorizedProjectList, args.PageSize, args.PageNum, ctx.Logger)
}

func OpenAPIGetProjectDetail(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	authorizedProjectList, found, err := internalhandler.ListAuthorizedProjects(ctx.UserID)
	if err != nil {
		ctx.RespErr = e.ErrInternalError.AddDesc(err.Error())
		return
	}

	if !found {
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Query("projectKey")
	if projectKey == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("projectKey is empty")
		return
	}

	authorized := false
	for _, authorizedProject := range authorizedProjectList {
		if projectKey == authorizedProject {
			authorized = true
			break
		}
	}

	if !authorized {
		ctx.UnAuthorized = true
		return
	}

	ctx.Resp, ctx.RespErr = service.GetProjectDetailOpenAPI(projectKey, ctx.Logger)
}

func OpenAPIDeleteProject(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Query("projectKey")
	if projectKey == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("projectKey is empty")
		return
	}

	if !ctx.Resources.IsSystemAdmin {
		if projectAuthInfo, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok || !projectAuthInfo.IsProjectAdmin {
			ctx.UnAuthorized = true
			return
		}
	}

	isDelete, err := strconv.ParseBool(c.Query("isDelete"))
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid param isDelete")
		return
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, projectKey, "OpenAPI"+"删除", "项目管理-项目", projectKey, "", types.RequestBodyTypeJSON, ctx.Logger)

	ctx.RespErr = service.DeleteProjectOpenAPI(ctx.UserName, ctx.RequestID, projectKey, isDelete, ctx.Logger)
}

func OpenAPIGetGlobalVariables(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	projectKey := c.Query("projectKey")
	if projectKey == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("projectKey is empty")
		return
	}

	ctx.Resp, ctx.RespErr = service.OpenAPIGetGlobalVariables(projectKey, ctx.Logger)
}
