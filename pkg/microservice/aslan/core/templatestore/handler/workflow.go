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
	templateservice "github.com/koderover/zadig/pkg/microservice/aslan/core/templatestore/service"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

func GetWorkflowTemplateByID(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)

	if err != nil {

		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		internalhandler.JSONResponse(c, ctx)
		return
	}

	// authorization check
	if !ctx.Resources.IsSystemAdmin {
		if !ctx.Resources.SystemActions.Template.View {
			projectKey := c.Param("projectName")
			if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
				ctx.UnAuthorized = true
				return
			}
			if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
				!ctx.Resources.ProjectAuthInfo[projectKey].Workflow.Create {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	resp, err := templateservice.GetWorkflowTemplateByID(c.Param("id"), ctx.Logger)
	if err != nil {
		c.JSON(e.ErrorMessage(err))
		c.Abort()
		return
	}
	c.YAML(200, resp)
}

func ListWorkflowTemplate(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	// authorization check
	if !ctx.Resources.IsSystemAdmin {
		if !ctx.Resources.SystemActions.Template.View {
			projectKey := c.Param("projectName")
			if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
				ctx.UnAuthorized = true
				return
			}
			if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
				!ctx.Resources.ProjectAuthInfo[projectKey].Workflow.Create {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	excludeBuildIn := false
	if c.Query("excludeBuildIn") == "true" {
		excludeBuildIn = true
	}

	ctx.Resp, ctx.Err = templateservice.ListWorkflowTemplate(c.Query("category"), excludeBuildIn, ctx.Logger)
}

func CreateWorkflowTemplate(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	// authorization check
	if !ctx.Resources.IsSystemAdmin {
		if !ctx.Resources.SystemActions.Template.Create {
			ctx.UnAuthorized = true
			return
		}
	}

	args := new(commonmodels.WorkflowV4Template)

	if err := c.ShouldBindYAML(&args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	ctx.Err = templateservice.CreateWorkflowTemplate(ctx.UserName, args, ctx.Logger)
}

func UpdateWorkflowTemplate(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	// authorization check
	if !ctx.Resources.IsSystemAdmin {
		if !ctx.Resources.SystemActions.Template.Edit {
			ctx.UnAuthorized = true
			return
		}
	}

	args := new(commonmodels.WorkflowV4Template)

	if err := c.ShouldBindYAML(&args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	ctx.Err = templateservice.UpdateWorkflowTemplate(ctx.UserName, args, ctx.Logger)
}

func DeleteWorkflowTemplateByID(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	// authorization check
	if !ctx.Resources.IsSystemAdmin {
		if !ctx.Resources.SystemActions.Template.Delete {
			ctx.UnAuthorized = true
			return
		}
	}

	ctx.Err = templateservice.DeleteWorkflowTemplateByID(c.Param("id"), ctx.Logger)
}
