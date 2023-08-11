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
	"encoding/json"
	"fmt"
	"io"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/workflow/service/workflow"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/koderover/zadig/pkg/types"
)

func GetWorkflowProductName(c *gin.Context) {
	args := new(commonmodels.Workflow)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("c.GetRawData() err : %v", err)
		return
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("json.Unmarshal err : %v", err)
		return
	}
	c.Set("productName", args.ProductTmplName)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))
	c.Next()
}

func GetProductNameByWorkflow(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	workflow, err := workflow.FindWorkflow(c.Param("name"), ctx.Logger)
	if err != nil {
		log.Errorf("FindWorkflow err : %v", err)
		return
	}
	c.Set("productName", workflow.ProductTmplName)
	c.Next()
}

func AutoCreateWorkflow(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Logger.Errorf("failed to generate authorization info for user: %s, error: %s", ctx.UserID, err)
		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Param("productName")

	// TODO: Authorization leak
	// this API is sometimes used in edit/create workflow scenario, thus giving the edit/create workflow permission
	// authorization check
	permitted := false

	if ctx.Resources.IsSystemAdmin {
		permitted = true
	}

	if projectAuthInfo, ok := ctx.Resources.ProjectAuthInfo[projectKey]; ok {
		// first check if the user is projectAdmin
		if projectAuthInfo.IsProjectAdmin {
			permitted = true
		}

		// then check if user has edit workflow permission
		if projectAuthInfo.Workflow.Create ||
			projectAuthInfo.Env.EditConfig {
			permitted = true
		}

		// finally check if the permission is given by collaboration mode
		collaborationAuthorizedEdit, err := internalhandler.CheckPermissionGivenByCollaborationMode(ctx.UserID, projectKey, types.ResourceTypeEnvironment, types.EnvActionEditConfig)
		if err == nil && collaborationAuthorizedEdit {
			permitted = true
		}
	}

	if !permitted {
		ctx.UnAuthorized = true
		return
	}

	ctx.Resp = workflow.AutoCreateWorkflow(projectKey, ctx.Logger)
}

func CreateWorkflow(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Logger.Errorf("failed to generate authorization info for user: %s, error: %s", ctx.UserID, err)
		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := new(commonmodels.Workflow)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("CreateWorkflow c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("CreateWorkflow json.Unmarshal err : %v", err)
	}

	projectKey := args.ProductTmplName
	workflowName := args.Name

	internalhandler.InsertOperationLog(c, ctx.UserName, projectKey, "新增", "工作流", workflowName, string(data), ctx.Logger)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	// authorization check
	if !ctx.Resources.IsSystemAdmin {
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

	if err := c.ShouldBindWith(&args, binding.JSON); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	args.UpdateBy = ctx.UserName
	args.CreateBy = ctx.UserName
	ctx.Err = workflow.CreateWorkflow(args, ctx.Logger)
}

// UpdateWorkflow  update a workflow
func UpdateWorkflow(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Logger.Errorf("failed to generate authorization info for user: %s, error: %s", ctx.UserID, err)
		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := new(commonmodels.Workflow)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("CreateWorkflow c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("UpdateWorkflow json.Unmarshal err : %v", err)
	}

	projectKey := args.ProductTmplName
	workflowName := args.Name

	internalhandler.InsertOperationLog(c, ctx.UserName, projectKey, "更新", "工作流", workflowName, string(data), ctx.Logger)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	// authorization check
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectKey].Workflow.Edit {
			// check if the permission is given by collaboration mode
			permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, projectKey, types.ResourceTypeWorkflow, args.Name, types.WorkflowActionEdit)
			if err != nil || !permitted {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	if err := c.ShouldBindWith(&args, binding.JSON); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	args.UpdateBy = ctx.UserName
	ctx.Err = workflow.UpdateWorkflow(args, ctx.Logger)
}

func ListWorkflows(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	projects := c.QueryArray("projects")
	projectName := c.Query("projectName")
	if projectName != "" && len(projects) > 0 {
		ctx.Err = e.ErrInvalidParam.AddDesc("projects and projectName can not be set together")
		return
	}
	if projectName != "" {
		projects = []string{c.Query("projectName")}
	}

	workflowNames, found := internalhandler.GetResourcesInHeader(c)
	if found && len(workflowNames) == 0 {
		ctx.Resp = []*workflow.Workflow{}
		return
	}

	ctx.Resp, ctx.Err = workflow.ListWorkflows(projects, ctx.UserID, workflowNames, ctx.Logger)
}

// TODO: this API is used only by picket, should be removed later
func ListTestWorkflows(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = workflow.ListTestWorkflows(c.Param("testName"), c.QueryArray("projects"), ctx.Logger)
}

// FindWorkflow find a workflow
func FindWorkflow(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Logger.Errorf("failed to generate authorization info for user: %s, error: %s", ctx.UserID, err)
		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	workflowName := c.Param("name")

	w, err := workflow.FindWorkflow(workflowName, ctx.Logger)
	if err != nil {
		ctx.Logger.Errorf("FindWorkflow error: %v", err)
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}

	// authorization check
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[w.ProductTmplName]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if !ctx.Resources.ProjectAuthInfo[w.ProductTmplName].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[w.ProductTmplName].Workflow.View {
			// check if the permission is given by collaboration mode
			permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, w.ProductTmplName, types.ResourceTypeWorkflow, workflowName, types.WorkflowActionView)
			if err != nil || !permitted {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	ctx.Resp = w
}

func DeleteWorkflow(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Logger.Errorf("failed to generate authorization info for user: %s, error: %s", ctx.UserID, err)
		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.GetString("productName")
	workflowName := c.Param("name")

	// authorization check
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectKey].Workflow.Delete {
			ctx.UnAuthorized = true
			return
		}
	}

	internalhandler.InsertOperationLog(c, ctx.UserName, projectKey, "删除", "工作流", workflowName, "", ctx.Logger)
	ctx.Err = commonservice.DeleteWorkflow(workflowName, ctx.RequestID, false, ctx.Logger)
}

func PreSetWorkflow(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = workflow.PreSetWorkflow(c.Param("productName"), ctx.Logger)
}

func CopyWorkflow(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Logger.Errorf("failed to generate authorization info for user: %s, error: %s", ctx.UserID, err)
		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	workflowName := c.Param("old")

	w, err := workflow.FindWorkflow(workflowName, ctx.Logger)
	if err != nil {
		ctx.Logger.Errorf("FindWorkflow error: %v", err)
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}

	// authorization check
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[w.ProductTmplName]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if !ctx.Resources.ProjectAuthInfo[w.ProductTmplName].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[w.ProductTmplName].Workflow.Create {
			ctx.UnAuthorized = true
			return
		}
	}

	ctx.Err = workflow.CopyWorkflow(workflowName, c.Param("new"), c.Param("newDisplay"), ctx.UserName, ctx.Logger)
}

// @Summary [DONT USE]  ZadigDeployJobSpec
// @Description [DONT USE] ZadigDeployJobSpec
// @Tags 	placeholder
// @Accept 	json
// @Produce json
// @Param 	deploy_job_spec 			body 		 commonmodels.ZadigDeployJobSpec	true 	"body"
// @Success 200
// @Router /api/aslan/placeholder/deploy_job_spec [post]
func Placeholder(c *gin.Context) {

}
