/*
Copyright 2026 The KodeRover Authors.

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
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/workflow/service/workflow"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	"github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/types"
)

func ListWorkflowTemplateVersions(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	ctx.Resp, ctx.RespErr = workflow.ListWorkflowTemplateVersions(c.Param("templateID"))
}

func DiffWorkflowTemplateVersions(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	fromVersion, err := strconv.Atoi(c.Query("from_version"))
	if err != nil {
		ctx.RespErr = errors.ErrInvalidParam.AddDesc("invalid from_version")
		return
	}
	toVersion, err := strconv.Atoi(c.Query("to_version"))
	if err != nil {
		ctx.RespErr = errors.ErrInvalidParam.AddDesc("invalid to_version")
		return
	}
	ctx.Resp, ctx.RespErr = workflow.DiffWorkflowTemplateVersions(c.Param("templateID"), fromVersion, toVersion)
}

func ListWorkflowTemplateReferences(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	ctx.Resp, ctx.RespErr = workflow.ListWorkflowTemplateReferences(c.Param("templateID"))
}

func GetWorkflowTemplateBindingStatus(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}
	if !authorizeWorkflowTemplateBindingView(ctx, c.Param("name")) {
		ctx.UnAuthorized = true
		return
	}
	ctx.Resp, ctx.RespErr = workflow.GetWorkflowTemplateBindingStatus(c.Param("name"))
}

func ResolveWorkflowTemplateBinding(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}
	if !authorizeWorkflowTemplateBindingEdit(ctx, c.Param("name")) {
		ctx.UnAuthorized = true
		return
	}

	req := new(workflow.ResolveWorkflowTemplateBindingRequest)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.RespErr = errors.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	ctx.RespErr = workflow.ResolveWorkflowTemplateBinding(c.Param("name"), ctx.UserName, req, ctx.Logger)
}

func authorizeWorkflowTemplateBindingView(ctx *internalhandler.Context, workflowName string) bool {
	w, err := workflow.FindWorkflowV4Raw(workflowName, ctx.Logger)
	if err != nil {
		ctx.RespErr = errors.ErrFindWorkflow.AddErr(err)
		return false
	}
	if ctx.Resources.IsSystemAdmin {
		return true
	}
	projectAuth, ok := ctx.Resources.ProjectAuthInfo[w.Project]
	if !ok {
		return false
	}
	if projectAuth.IsProjectAdmin || projectAuth.Workflow.View || projectAuth.Workflow.Edit {
		return true
	}
	permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, w.Project, types.ResourceTypeWorkflow, w.Name, types.WorkflowActionView)
	return err == nil && permitted
}

func authorizeWorkflowTemplateBindingEdit(ctx *internalhandler.Context, workflowName string) bool {
	w, err := workflow.FindWorkflowV4Raw(workflowName, ctx.Logger)
	if err != nil {
		ctx.RespErr = errors.ErrFindWorkflow.AddErr(err)
		return false
	}
	if ctx.Resources.IsSystemAdmin {
		return true
	}
	projectAuth, ok := ctx.Resources.ProjectAuthInfo[w.Project]
	if !ok {
		return false
	}
	if projectAuth.IsProjectAdmin || projectAuth.Workflow.Edit {
		return true
	}
	permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, w.Project, types.ResourceTypeWorkflow, w.Name, types.WorkflowActionEdit)
	return err == nil && permitted
}
