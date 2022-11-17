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

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	workflowservice "github.com/koderover/zadig/pkg/microservice/aslan/core/workflow/service/workflow"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	"github.com/koderover/zadig/pkg/tool/errors"
)

func CreateWorkflowView(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	req := &commonmodels.WorkflowView{}
	if err := c.ShouldBindJSON(&req); err != nil {
		ctx.Err = errors.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	if req.Name == "" {
		ctx.Err = fmt.Errorf("view name cannot be empty")
		return
	}
	if req.ProjectName == "" {
		ctx.Err = fmt.Errorf("project name cannot be empty")
		return
	}
	ctx.Err = workflowservice.CreateWorkflowView(req.Name, req.ProjectName, req.Workflows, ctx.UserName, ctx.Logger)
}

func UpdateWorkflowView(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	req := &commonmodels.WorkflowView{}
	if err := c.ShouldBindJSON(&req); err != nil {
		ctx.Err = errors.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	ctx.Err = workflowservice.UpdateWorkflowView(req, ctx.UserName, ctx.Logger)
}

func GetWorkflowViewPreset(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = workflowservice.GetWorkflowViewPreset(c.Query("projectName"), c.Query("viewName"), ctx.Logger)
}

func ListWorkflowViewNames(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = workflowservice.ListWorkflowViewNames(c.Query("projectName"), ctx.Logger)
}

func DeleteWorkflowView(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Err = workflowservice.DeleteWorkflowView(c.Param("projectName"), c.Param("viewName"), ctx.Logger)
}
