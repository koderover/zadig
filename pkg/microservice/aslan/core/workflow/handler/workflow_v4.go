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

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"

	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/types"

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/workflow/service/workflow"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	"github.com/koderover/zadig/pkg/tool/errors"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/log"
)

type listWorkflowV4Query struct {
	PageSize int64  `json:"page_size"    form:"page_size,default=20"`
	PageNum  int64  `json:"page_num"     form:"page_num,default=1"`
	Project  string `json:"project"      form:"project"`
	ViewName string `json:"view_name"    form:"view_name"`
}

type filterDeployServiceVarsQuery struct {
	WorkflowName string   `json:"workflow_name"`
	JobName      string   `json:"job_name"`
	EnvName      string   `json:"env_name"`
	ServiceNames []string `json:"service_names"`
}

type getHelmValuesDifferenceReq struct {
	ServiceName           string            `json:"service_name"`
	VariableYaml          string            `json:"variable_yaml"`
	EnvName               string            `json:"env_name"`
	IsProduction          bool              `json:"production"`
	IsHelmChartDeploy     bool              `json:"is_helm_chart_deploy"`
	UpdateServiceRevision bool              `json:"update_service_revision"`
	ServiceModules        []*ModuleAndImage `json:"service_modules"`
}

type ModuleAndImage struct {
	Image string `json:"image"`
	Name  string `json:"name"`
}

type listWorkflowV4Resp struct {
	WorkflowList []*workflow.Workflow `json:"workflow_list"`
	Total        int64                `json:"total"`
}

func CreateWorkflowV4(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := new(commonmodels.WorkflowV4)
	data := getBody(c)
	if err := yaml.Unmarshal([]byte(data), args); err != nil {
		log.Errorf("CreateWorkflowv4 yaml.Unmarshal err : %s", err)
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, args.Project, "新增", "自定义工作流", args.Name, data, ctx.Logger)

	// authorization check
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[args.Project]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if !ctx.Resources.ProjectAuthInfo[args.Project].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[args.Project].Workflow.Create {
			// check if the permission is given by collaboration mode
			ctx.UnAuthorized = true
			return
		}
	}

	if err := workflow.CreateWorkflowV4(ctx.UserName, args, ctx.Logger); err != nil {
		ctx.Err = err
		return
	}

	if view := c.Query("viewName"); view != "" {
		workflow.AddWorkflowToView(args.Project, view, []*commonmodels.WorkflowViewDetail{
			{
				WorkflowName:        args.Name,
				WorkflowDisplayName: args.DisplayName,
				WorkflowType:        setting.CustomWorkflowType,
			},
		}, ctx.Logger)
	}
}

func SetWorkflowTasksCustomFields(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	workflowName := c.Param("name")
	projectKey := c.Query("projectName")
	args := new(commonmodels.CustomField)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("SetWorkflowTasksCustomFields c.GetRawData() err : %v", err)
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("SetWorkflowTasksCustomFields json.Unmarshal err : %v", err)
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

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

	ctx.Err = workflow.SetWorkflowTasksCustomFields(projectKey, workflowName, args, ctx.Logger)
}

func GetWorkflowTasksCustomFields(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	workflowName := c.Param("name")
	projectName := c.Query("projectName")

	ctx.Resp, ctx.Err = workflow.GetWorkflowTasksCustomFields(projectName, workflowName, ctx.Logger)
}

func LintWorkflowV4(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(commonmodels.WorkflowV4)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("CreateWorkflowv4 c.GetRawData() err : %s", err)
	}
	if err = yaml.Unmarshal(data, args); err != nil {
		log.Errorf("CreateWorkflowv4 json.Unmarshal err : %s", err)
	}

	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	if err := c.ShouldBindYAML(&args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	ctx.Err = workflow.LintWorkflowV4(args, ctx.Logger)
}

func ListWorkflowV4(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := &listWorkflowV4Query{}
	if err := c.ShouldBindQuery(args); err != nil {
		ctx.Err = err
		return
	}

	var authorizedWorkflow, authorizedWorkflowV4 []string
	var enableFilter bool

	// authorization checks
	if ctx.Resources.IsSystemAdmin {
		enableFilter = false
		authorizedWorkflow = make([]string, 0)
		authorizedWorkflowV4 = make([]string, 0)
		ctx.Logger.Infof("user is admin, disabling filter")
	} else if projectAuth, ok := ctx.Resources.ProjectAuthInfo[args.Project]; ok {
		if projectAuth.IsProjectAdmin || projectAuth.Workflow.View {
			enableFilter = false
			authorizedWorkflow = make([]string, 0)
			authorizedWorkflowV4 = make([]string, 0)
			ctx.Logger.Infof("user is project admin or has workflow view authorization, disabling filter")
		} else {
			var err error
			authorizedWorkflow, authorizedWorkflowV4, enableFilter, err = internalhandler.ListAuthorizedWorkflows(ctx.UserID, args.Project)
			if err != nil {
				ctx.Logger.Errorf("failed to list authorized workflow resource, error: %s", err)
				// something wrong when getting workflow authorization info, returning empty
				ctx.Resp = listWorkflowV4Resp{
					WorkflowList: make([]*workflow.Workflow, 0),
					Total:        0,
				}
				return
			}
		}
	} else {
		// if a user does not have a role in a project, it must also not have a collaboration mode
		// thus return nothing
		ctx.Logger.Warnf("failed to get project auth info, returning empty workflow")
		ctx.Resp = listWorkflowV4Resp{
			WorkflowList: make([]*workflow.Workflow, 0),
			Total:        0,
		}
		return
	}

	workflowList, err := workflow.ListWorkflowV4(args.Project, args.ViewName, ctx.UserID, authorizedWorkflow, authorizedWorkflowV4, enableFilter, ctx.Logger)
	resp := listWorkflowV4Resp{
		WorkflowList: workflowList,
		Total:        int64(len(workflowList)),
	}
	ctx.Resp = resp
	ctx.Err = err
}

func ListWorkflowV4CanTrigger(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	ctx.Resp, ctx.Err = workflow.ListWorkflowV4CanTrigger(ctx)
}

func UpdateWorkflowV4(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := new(commonmodels.WorkflowV4)
	data := getBody(c)
	if err := yaml.Unmarshal([]byte(data), args); err != nil {
		log.Errorf("UpdateWorkflowV4 yaml.Unmarshal err : %s", err)
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	internalhandler.InsertOperationLog(c, ctx.UserName, args.Project, "更新", "自定义工作流", args.Name, string(data), ctx.Logger)

	// authorization check
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[args.Project]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if !ctx.Resources.ProjectAuthInfo[args.Project].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[args.Project].Workflow.Edit {
			// check if the permission is given by collaboration mode
			permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, args.Project, types.ResourceTypeWorkflow, args.Name, types.WorkflowActionEdit)
			if err != nil || !permitted {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	ctx.Err = workflow.UpdateWorkflowV4(c.Param("name"), ctx.UserName, args, ctx.Logger)
}

func DeleteWorkflowV4(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	w, err := workflow.FindWorkflowV4Raw(c.Param("name"), ctx.Logger)
	if err != nil {
		ctx.Logger.Errorf("DeleteWorkflowV4 error: %v", err)
		ctx.Err = e.ErrDeleteWorkflow.AddErr(err)
		return
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, w.Project, "(OpenAPI)"+"删除", "自定义工作流", c.Param("name"), "", ctx.Logger)

	// authorization check
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[w.Project]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if !ctx.Resources.ProjectAuthInfo[w.Project].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[w.Project].Workflow.Edit {
			ctx.UnAuthorized = true
			return
		}
	}

	ctx.Err = workflow.DeleteWorkflowV4(c.Param("name"), ctx.Logger)
}

func FindWorkflowV4(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	resp, err := workflow.FindWorkflowV4(c.Query("encryptedKey"), c.Param("name"), ctx.Logger)
	if err != nil {
		c.JSON(e.ErrorMessage(err))
		c.Abort()
		return
	}

	// authorization check
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[resp.Project]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if !ctx.Resources.ProjectAuthInfo[resp.Project].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[resp.Project].Workflow.Edit {
			// check if the permission is given by collaboration mode
			permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, resp.Project, types.ResourceTypeWorkflow, resp.Name, types.WorkflowActionView)
			if err != nil || !permitted {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	c.YAML(200, resp)
}

func GetWorkflowV4Preset(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = workflow.GetWorkflowv4Preset(c.Query("encryptedKey"), c.Param("name"), ctx.UserID, ctx.UserName, ctx.Logger)
}

func GetWebhookForWorkflowV4Preset(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = workflow.GetWebhookForWorkflowV4Preset(c.Query("workflowName"), c.Query("triggerName"), ctx.Logger)
}

func CheckWorkflowV4Approval(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Err = workflow.CheckWorkflowV4ApprovalInitiator(c.Param("name"), ctx.UserID, ctx.Logger)
}

func ListWebhookForWorkflowV4(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = workflow.ListWebhookForWorkflowV4(c.Query("workflowName"), ctx.Logger)
}

func CreateWebhookForWorkflowV4(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	req := new(commonmodels.WorkflowV4Hook)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	w, err := workflow.FindWorkflowV4Raw(c.Param("workflowName"), ctx.Logger)
	if err != nil {
		ctx.Logger.Errorf("CreateWebhookForWorkflowV4 error: %v", err)
		ctx.Err = e.ErrCreateWebhook.AddErr(err)
		return
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, w.Project, "新建", "自定义工作流-webhook", w.Name, getBody(c), ctx.Logger)

	// authorization check
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[w.Project]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if !ctx.Resources.ProjectAuthInfo[w.Project].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[w.Project].Workflow.Edit {
			// check if the permission is given by collaboration mode
			permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, w.Project, types.ResourceTypeWorkflow, w.Name, types.WorkflowActionEdit)
			if err != nil || !permitted {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	ctx.Err = workflow.CreateWebhookForWorkflowV4(c.Param("workflowName"), req, ctx.Logger)
}

func UpdateWebhookForWorkflowV4(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	req := new(commonmodels.WorkflowV4Hook)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	w, err := workflow.FindWorkflowV4Raw(c.Param("workflowName"), ctx.Logger)
	if err != nil {
		ctx.Logger.Errorf("UpdateWebhookForWorkflowV4 error: %v", err)
		ctx.Err = e.ErrUpdateWebhook.AddErr(err)
		return
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, w.Project, "更新", "自定义工作流-webhook", w.Name, getBody(c), ctx.Logger)

	// authorization check
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[w.Project]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if !ctx.Resources.ProjectAuthInfo[w.Project].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[w.Project].Workflow.Edit {
			// check if the permission is given by collaboration mode
			permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, w.Project, types.ResourceTypeWorkflow, w.Name, types.WorkflowActionEdit)
			if err != nil || !permitted {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	ctx.Err = workflow.UpdateWebhookForWorkflowV4(c.Param("workflowName"), req, ctx.Logger)
}

func DeleteWebhookForWorkflowV4(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	w, err := workflow.FindWorkflowV4Raw(c.Param("workflowName"), ctx.Logger)
	if err != nil {
		ctx.Logger.Errorf("DeleteWebhookForWorkflowV4 error: %v", err)
		ctx.Err = e.ErrDeleteWebhook.AddErr(err)
		return
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, w.Project, "删除", "自定义工作流-webhook", w.Name, "", ctx.Logger)

	// authorization check
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[w.Project]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if !ctx.Resources.ProjectAuthInfo[w.Project].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[w.Project].Workflow.Edit {
			// check if the permission is given by collaboration mode
			permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, w.Project, types.ResourceTypeWorkflow, w.Name, types.WorkflowActionEdit)
			if err != nil || !permitted {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	ctx.Err = workflow.DeleteWebhookForWorkflowV4(c.Param("workflowName"), c.Param("triggerName"), ctx.Logger)
}

func CreateJiraHookForWorkflowV4(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	jira := new(commonmodels.JiraHook)
	if err := c.ShouldBindJSON(jira); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	w, err := workflow.FindWorkflowV4Raw(c.Param("workflowName"), ctx.Logger)
	if err != nil {
		ctx.Logger.Errorf("CreateJiraHookForWorkflowV4 error: %v", err)
		ctx.Err = e.ErrCreateJiraHook.AddErr(err)
		return
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, w.Project, "新建", "自定义工作流-jirahook", w.Name, getBody(c), ctx.Logger)

	// authorization check
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[w.Project]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if !ctx.Resources.ProjectAuthInfo[w.Project].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[w.Project].Workflow.Edit {
			// check if the permission is given by collaboration mode
			permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, w.Project, types.ResourceTypeWorkflow, w.Name, types.WorkflowActionEdit)
			if err != nil || !permitted {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	ctx.Err = workflow.CreateJiraHookForWorkflowV4(c.Param("workflowName"), jira, ctx.Logger)
}

func GetJiraHookForWorkflowV4Preset(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	ctx.Resp, ctx.Err = workflow.GetJiraHookForWorkflowV4Preset(c.Query("workflowName"), c.Query("hookName"), ctx.Logger)
}

func ListJiraHookForWorkflowV4(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	ctx.Resp, ctx.Err = workflow.ListJiraHookForWorkflowV4(c.Param("workflowName"), ctx.Logger)
}

func UpdateJiraHookForWorkflowV4(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	jira := new(commonmodels.JiraHook)
	if err := c.ShouldBindJSON(jira); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	w, err := workflow.FindWorkflowV4Raw(c.Param("workflowName"), ctx.Logger)
	if err != nil {
		ctx.Logger.Errorf("UpdateJiraHookForWorkflowV4 error: %v", err)
		ctx.Err = e.ErrUpdateJiraHook.AddErr(err)
		return
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, w.Project, "更新", "自定义工作流-jirahook", w.Name, getBody(c), ctx.Logger)

	// authorization check
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[w.Project]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if !ctx.Resources.ProjectAuthInfo[w.Project].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[w.Project].Workflow.Edit {
			// check if the permission is given by collaboration mode
			permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, w.Project, types.ResourceTypeWorkflow, w.Name, types.WorkflowActionEdit)
			if err != nil || !permitted {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	ctx.Err = workflow.UpdateJiraHookForWorkflowV4(c.Param("workflowName"), jira, ctx.Logger)
}

func DeleteJiraHookForWorkflowV4(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	w, err := workflow.FindWorkflowV4Raw(c.Param("workflowName"), ctx.Logger)
	if err != nil {
		ctx.Logger.Errorf("DeleteJiraHookForWorkflowV4 error: %v", err)
		ctx.Err = e.ErrDeleteJiraHook.AddErr(err)
		return
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, w.Project, "删除", "自定义工作流-jirahook", w.Name, "", ctx.Logger)

	// authorization check
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[w.Project]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if !ctx.Resources.ProjectAuthInfo[w.Project].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[w.Project].Workflow.Edit {
			// check if the permission is given by collaboration mode
			permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, w.Project, types.ResourceTypeWorkflow, w.Name, types.WorkflowActionEdit)
			if err != nil || !permitted {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	ctx.Err = workflow.DeleteJiraHookForWorkflowV4(c.Param("workflowName"), c.Param("hookName"), ctx.Logger)
}

func CreateMeegoHookForWorkflowV4(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	jira := new(commonmodels.MeegoHook)
	if err := c.ShouldBindJSON(jira); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	w, err := workflow.FindWorkflowV4Raw(c.Param("workflowName"), ctx.Logger)
	if err != nil {
		ctx.Logger.Errorf("CreateMeegoHookForWorkflowV4 error: %v", err)
		ctx.Err = e.ErrCreateMeegoHook.AddErr(err)
		return
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, w.Project, "新建", "自定义工作流-meegohook", w.Name, getBody(c), ctx.Logger)

	// authorization check
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[w.Project]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if !ctx.Resources.ProjectAuthInfo[w.Project].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[w.Project].Workflow.Edit {
			// check if the permission is given by collaboration mode
			permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, w.Project, types.ResourceTypeWorkflow, w.Name, types.WorkflowActionEdit)
			if err != nil || !permitted {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	ctx.Err = workflow.CreateMeegoHookForWorkflowV4(c.Param("workflowName"), jira, ctx.Logger)
}

func GetMeegoHookForWorkflowV4Preset(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	ctx.Resp, ctx.Err = workflow.GetMeegoHookForWorkflowV4Preset(c.Query("workflowName"), c.Query("hookName"), ctx.Logger)
}

func ListMeegoHookForWorkflowV4(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	ctx.Resp, ctx.Err = workflow.ListMeegoHookForWorkflowV4(c.Param("workflowName"), ctx.Logger)
}

func UpdateMeegoHookForWorkflowV4(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	jira := new(commonmodels.MeegoHook)
	if err := c.ShouldBindJSON(jira); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	w, err := workflow.FindWorkflowV4Raw(c.Param("workflowName"), ctx.Logger)
	if err != nil {
		ctx.Logger.Errorf("UpdateMeegoHookForWorkflowV4 error: %v", err)
		ctx.Err = e.ErrUpdateMeegoHook.AddErr(err)
		return
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, w.Project, "更新", "自定义工作流-meegohook", w.Name, getBody(c), ctx.Logger)

	// authorization check
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[w.Project]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if !ctx.Resources.ProjectAuthInfo[w.Project].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[w.Project].Workflow.Edit {
			// check if the permission is given by collaboration mode
			permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, w.Project, types.ResourceTypeWorkflow, w.Name, types.WorkflowActionEdit)
			if err != nil || !permitted {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	ctx.Err = workflow.UpdateMeegoHookForWorkflowV4(c.Param("workflowName"), jira, ctx.Logger)
}

func DeleteMeegoHookForWorkflowV4(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	w, err := workflow.FindWorkflowV4Raw(c.Param("workflowName"), ctx.Logger)
	if err != nil {
		ctx.Logger.Errorf("DeleteMeegoHookForWorkflowV4 error: %v", err)
		ctx.Err = e.ErrDeleteMeegoHook.AddErr(err)
		return
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, w.Project, "删除", "自定义工作流-meegohook", w.Name, "", ctx.Logger)

	// authorization check
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[w.Project]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if !ctx.Resources.ProjectAuthInfo[w.Project].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[w.Project].Workflow.Edit {
			// check if the permission is given by collaboration mode
			permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, w.Project, types.ResourceTypeWorkflow, w.Name, types.WorkflowActionEdit)
			if err != nil || !permitted {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	ctx.Err = workflow.DeleteMeegoHookForWorkflowV4(c.Param("workflowName"), c.Param("hookName"), ctx.Logger)
}

func CreateGeneralHookForWorkflowV4(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	hook := new(commonmodels.GeneralHook)
	if err := c.ShouldBindJSON(hook); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	w, err := workflow.FindWorkflowV4Raw(c.Param("workflowName"), ctx.Logger)
	if err != nil {
		ctx.Logger.Errorf("CreateGeneralHookForWorkflowV4 error: %v", err)
		ctx.Err = e.ErrCreateGeneralHook.AddErr(err)
		return
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, w.Project, "新建", "自定义工作流-generalhook", w.Name, getBody(c), ctx.Logger)

	// authorization check
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[w.Project]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if !ctx.Resources.ProjectAuthInfo[w.Project].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[w.Project].Workflow.Edit {
			// check if the permission is given by collaboration mode
			permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, w.Project, types.ResourceTypeWorkflow, w.Name, types.WorkflowActionEdit)
			if err != nil || !permitted {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	ctx.Err = workflow.CreateGeneralHookForWorkflowV4(c.Param("workflowName"), hook, ctx.Logger)
}

func GetGeneralHookForWorkflowV4Preset(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	ctx.Resp, ctx.Err = workflow.GetGeneralHookForWorkflowV4Preset(c.Query("workflowName"), c.Query("hookName"), ctx.Logger)
}

func ListGeneralHookForWorkflowV4(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	ctx.Resp, ctx.Err = workflow.ListGeneralHookForWorkflowV4(c.Param("workflowName"), ctx.Logger)
}

func UpdateGeneralHookForWorkflowV4(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	hook := new(commonmodels.GeneralHook)
	if err := c.ShouldBindJSON(hook); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	w, err := workflow.FindWorkflowV4Raw(c.Param("workflowName"), ctx.Logger)
	if err != nil {
		ctx.Logger.Errorf("UpdateGeneralHookForWorkflowV4 error: %v", err)
		ctx.Err = e.ErrUpdateGeneralHook.AddErr(err)
		return
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, w.Project, "更新", "自定义工作流-generalhook", w.Name, getBody(c), ctx.Logger)

	// authorization check
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[w.Project]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if !ctx.Resources.ProjectAuthInfo[w.Project].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[w.Project].Workflow.Edit {
			// check if the permission is given by collaboration mode
			permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, w.Project, types.ResourceTypeWorkflow, w.Name, types.WorkflowActionEdit)
			if err != nil || !permitted {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	ctx.Err = workflow.UpdateGeneralHookForWorkflowV4(c.Param("workflowName"), hook, ctx.Logger)
}

func DeleteGeneralHookForWorkflowV4(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	w, err := workflow.FindWorkflowV4Raw(c.Param("workflowName"), ctx.Logger)
	if err != nil {
		ctx.Logger.Errorf("DeleteGeneralHookForWorkflowV4 error: %v", err)
		ctx.Err = e.ErrDeleteGeneralHook.AddErr(err)
		return
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, w.Project, "删除", "自定义工作流-generalhook", w.Name, "", ctx.Logger)

	// authorization check
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[w.Project]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if !ctx.Resources.ProjectAuthInfo[w.Project].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[w.Project].Workflow.Edit {
			// check if the permission is given by collaboration mode
			permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, w.Project, types.ResourceTypeWorkflow, w.Name, types.WorkflowActionEdit)
			if err != nil || !permitted {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	ctx.Err = workflow.DeleteGeneralHookForWorkflowV4(c.Param("workflowName"), c.Param("hookName"), ctx.Logger)
}

func GeneralHookEventHandler(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	ctx.Err = workflow.GeneralHookEventHandler(c.Param("workflowName"), c.Param("hookName"), ctx.Logger)
}

func GetCronForWorkflowV4Preset(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = workflow.GetCronForWorkflowV4Preset(c.Query("workflowName"), c.Query("cronID"), ctx.Logger)
}

func ListCronForWorkflowV4(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = workflow.ListCronForWorkflowV4(c.Query("workflowName"), ctx.Logger)
}

func CreateCronForWorkflowV4(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	req := new(commonmodels.Cronjob)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	w, err := workflow.FindWorkflowV4Raw(c.Param("workflowName"), ctx.Logger)
	if err != nil {
		ctx.Logger.Errorf("CreateCronForWorkflowV4 error: %v", err)
		ctx.Err = e.ErrUpsertCronjob.AddErr(err)
		return
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, w.Project, "新建", "自定义工作流-cron", w.Name, getBody(c), ctx.Logger)

	// authorization check
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[w.Project]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if !ctx.Resources.ProjectAuthInfo[w.Project].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[w.Project].Workflow.Edit {
			// check if the permission is given by collaboration mode
			permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, w.Project, types.ResourceTypeWorkflow, w.Name, types.WorkflowActionEdit)
			if err != nil || !permitted {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	ctx.Err = workflow.CreateCronForWorkflowV4(c.Param("workflowName"), req, ctx.Logger)
}

func UpdateCronForWorkflowV4(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	req := new(commonmodels.Cronjob)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, req.WorkflowV4Args.Project, "更新", "自定义工作流-cron", req.WorkflowV4Args.Name, getBody(c), ctx.Logger)

	// authorization check
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[req.WorkflowV4Args.Project]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if !ctx.Resources.ProjectAuthInfo[req.WorkflowV4Args.Project].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[req.WorkflowV4Args.Project].Workflow.Edit {
			// check if the permission is given by collaboration mode
			permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, req.WorkflowV4Args.Project, types.ResourceTypeWorkflow, req.WorkflowV4Args.Name, types.WorkflowActionEdit)
			if err != nil || !permitted {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	ctx.Err = workflow.UpdateCronForWorkflowV4(req, ctx.Logger)
}

func DeleteCronForWorkflowV4(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	w, err := workflow.FindWorkflowV4Raw(c.Param("workflowName"), ctx.Logger)
	if err != nil {
		ctx.Logger.Errorf("CreateCronForWorkflowV4 error: %v", err)
		ctx.Err = e.ErrUpsertCronjob.AddErr(err)
		return
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, w.Project, "删除", "自定义工作流-cron", w.Name, "", ctx.Logger)

	// authorization check
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[w.Project]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if !ctx.Resources.ProjectAuthInfo[w.Project].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[w.Project].Workflow.Edit {
			// check if the permission is given by collaboration mode
			permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, w.Project, types.ResourceTypeWorkflow, w.Name, types.WorkflowActionEdit)
			if err != nil || !permitted {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	ctx.Err = workflow.DeleteCronForWorkflowV4(c.Param("workflowName"), c.Param("cronID"), ctx.Logger)
}

func GetPatchParams(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	req := &commonmodels.PatchItem{}
	if err := c.ShouldBindJSON(&req); err != nil {
		ctx.Err = errors.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	ctx.Resp, ctx.Err = workflow.GetPatchParams(req, ctx.Logger)
}

func GetWorkflowGlobalVars(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(commonmodels.WorkflowV4)

	if err := c.ShouldBindYAML(&args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	ctx.Resp = workflow.GetWorkflowGlabalVars(args, c.Param("jobName"), ctx.Logger)
}

func GetWorkflowRepoIndex(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(commonmodels.WorkflowV4)

	if err := c.ShouldBindYAML(&args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	ctx.Resp = workflow.GetWorkflowRepoIndex(args, c.Param("jobName"), ctx.Logger)
}

func CheckShareStorageEnabled(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = workflow.CheckShareStorageEnabled(c.Query("id"), c.Query("type"), c.Query("name"), c.Query("project"), ctx.Logger)
}

func ListAllAvailableWorkflows(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = workflow.ListAllAvailableWorkflows(c.QueryArray("projects"), ctx.Logger)
}

// @Summary Get filtered env services
// @Description Get filtered env services
// @Tags 	workflow
// @Accept 	json
// @Produce json
// @Param 	body 		body 		filterDeployServiceVarsQuery	 	true 	"body"
// @Success 200 		{array} 	commonmodels.DeployService
// @Router /api/aslan/workflow/v4/filterEnv [post]
func GetFilteredEnvServices(c *gin.Context) {
	// TODO: fix the authorization problem for this.
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	req := new(filterDeployServiceVarsQuery)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	ctx.Resp, ctx.Err = workflow.GetFilteredEnvServices(req.WorkflowName, req.JobName, req.EnvName, req.ServiceNames, ctx.Logger)
}

// @Summary Compare Helm Service Yaml In Env
// @Description Compare Helm Service Yaml In Env
// @Tags 	workflow
// @Accept 	json
// @Produce json
// @Param 	body 		body 		getHelmValuesDifferenceReq	 	true 	"body"
// @Success 200 		{object} 	workflow.GetHelmValuesDifferenceResp
// @Router /api/aslan/workflow/v4/yamlComparison [post]
func CompareHelmServiceYamlInEnv(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	req := new(getHelmValuesDifferenceReq)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	projectName := c.Query("projectName")

	images := make([]string, 0)
	for _, imageInfos := range req.ServiceModules {
		images = append(images, imageInfos.Image)
	}
	ctx.Resp, ctx.Err = workflow.CompareHelmServiceYamlInEnv(req.ServiceName, req.VariableYaml, req.EnvName, projectName, images, req.IsProduction, req.UpdateServiceRevision, req.IsHelmChartDeploy, ctx.Logger)
}

type YamlResponse struct {
	Yaml string `json:"yaml"`
}

func RenderMseServiceYaml(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	type RenderMseServiceYamlReq struct {
		commonmodels.MseGrayReleaseService `json:",inline"`
		LastGrayTag                        string `json:"last_gray_tag"`
		GrayTag                            string `json:"gray_tag"`
		EnvName                            string `json:"env_name"`
	}

	req := new(RenderMseServiceYamlReq)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	if req.MseGrayReleaseService.YamlContent == "" {
		mseServiceYaml, err := workflow.GetMseOriginalServiceYaml(c.Query("projectName"), req.EnvName, req.MseGrayReleaseService.ServiceName, req.GrayTag)
		if err != nil {
			ctx.Err = err
			return
		}
		ctx.Resp = YamlResponse{Yaml: mseServiceYaml}
		return
	}

	mseServiceYaml, err := workflow.RenderMseServiceYaml(c.Query("projectName"), req.EnvName, req.LastGrayTag, req.GrayTag, &req.MseGrayReleaseService)
	if err != nil {
		ctx.Err = err
		return
	}
	ctx.Resp = YamlResponse{Yaml: mseServiceYaml}
}

func GetMseOfflineResources(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	services, err := workflow.GetMseOfflineResources(c.Query("grayTag"), c.Query("envName"), c.Query("projectName"))
	if err != nil {
		ctx.Err = err
		return
	}
	ctx.Resp = struct {
		Services []string `json:"services"`
	}{
		Services: services,
	}
}

func GetBlueGreenServiceK8sServiceYaml(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	blueGreenServiceYaml, err := workflow.GetBlueGreenServiceK8sServiceYaml(c.Query("projectName"), c.Param("envName"), c.Param("serviceName"))
	if err != nil {
		ctx.Err = err
		return
	}
	ctx.Resp = YamlResponse{Yaml: blueGreenServiceYaml}
}

func GetMseTagsInEnv(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	tags, err := workflow.GetMseTagsInEnv(c.Param("envName"), c.Query("projectName"))
	if err != nil {
		ctx.Err = err
		return
	}
	ctx.Resp = struct {
		Tags []string `json:"tags"`
	}{
		Tags: tags,
	}
}

func getBody(c *gin.Context) string {
	b, err := c.GetRawData()
	if err != nil {
		return ""
	}
	return string(b)
}
