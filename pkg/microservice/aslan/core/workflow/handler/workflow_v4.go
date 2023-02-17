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
	"io"
	"strings"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"

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

type listWorkflowV4Resp struct {
	WorkflowList []*workflow.Workflow `json:"workflow_list"`
	Total        int64                `json:"total"`
}

func CreateWorkflowV4(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(commonmodels.WorkflowV4)
	data := getBody(c)
	if err := yaml.Unmarshal([]byte(data), args); err != nil {
		log.Errorf("CreateWorkflowv4 yaml.Unmarshal err : %s", err)
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, args.Project, "新增", "自定义工作流", args.Name, data, ctx.Logger)
	ctx.Err = workflow.CreateWorkflowV4(ctx.UserName, args, ctx.Logger)
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
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	args := &listWorkflowV4Query{}
	if err := c.ShouldBindQuery(args); err != nil {
		ctx.Err = err
		return
	}
	workflowNames, found := internalhandler.GetResourcesInHeader(c)

	ctx.Logger.Infof("workflowNames:%s found:%v", workflowNames, found)
	var workflowV4Names, names []string
	for _, name := range workflowNames {
		if strings.HasPrefix(name, "common##") {
			workflowV4Names = append(workflowV4Names, strings.Split(name, "##")[1])
		} else {
			names = append(names, name)
		}
	}
	workflowList, err := workflow.ListWorkflowV4(args.Project, args.ViewName, ctx.UserID, names, workflowV4Names, found, ctx.Logger)
	resp := listWorkflowV4Resp{
		WorkflowList: workflowList,
		Total:        0,
	}
	ctx.Resp = resp
	ctx.Err = err
}

func UpdateWorkflowV4(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(commonmodels.WorkflowV4)
	data := getBody(c)
	if err := yaml.Unmarshal([]byte(data), args); err != nil {
		log.Errorf("UpdateWorkflowV4 yaml.Unmarshal err : %s", err)
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	internalhandler.InsertOperationLog(c, ctx.UserName, args.Project, "更新", "自定义工作流", args.Name, string(data), ctx.Logger)
	ctx.Err = workflow.UpdateWorkflowV4(c.Param("name"), ctx.UserName, args, ctx.Logger)
}

func DeleteWorkflowV4(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	w, err := workflow.FindWorkflowV4Raw(c.Param("name"), ctx.Logger)
	if err != nil {
		ctx.Logger.Errorf("DeleteWorkflowV4 error: %v", err)
		ctx.Err = e.ErrDeleteWorkflow.AddErr(err)
		return
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, w.Project, "删除", "自定义工作流", c.Param("name"), "", ctx.Logger)
	ctx.Err = workflow.DeleteWorkflowV4(c.Param("name"), ctx.Logger)
}

func FindWorkflowV4(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	resp, err := workflow.FindWorkflowV4(c.Query("encryptedKey"), c.Param("name"), ctx.Logger)
	if err != nil {
		c.JSON(e.ErrorMessage(err))
		c.Abort()
		return
	}
	c.YAML(200, resp)
}

func GetWorkflowV4Preset(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = workflow.GetWorkflowv4Preset(c.Query("encryptedKey"), c.Param("name"), ctx.UserID, ctx.Logger)
}

func GetWebhookForWorkflowV4Preset(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = workflow.GetWebhookForWorkflowV4Preset(c.Query("workflowName"), c.Query("triggerName"), ctx.Logger)
}

func CheckWorkflowV4LarkApproval(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Err = workflow.CheckWorkflowV4LarkApproval(c.Param("name"), ctx.UserID, ctx.Logger)
}

func ListWebhookForWorkflowV4(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = workflow.ListWebhookForWorkflowV4(c.Query("workflowName"), ctx.Logger)
}

func CreateWebhookForWorkflowV4(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

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
	ctx.Err = workflow.CreateWebhookForWorkflowV4(c.Param("workflowName"), req, ctx.Logger)
}

func UpdateWebhookForWorkflowV4(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

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
	ctx.Err = workflow.UpdateWebhookForWorkflowV4(c.Param("workflowName"), req, ctx.Logger)
}

func DeleteWebhookForWorkflowV4(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	w, err := workflow.FindWorkflowV4Raw(c.Param("workflowName"), ctx.Logger)
	if err != nil {
		ctx.Logger.Errorf("DeleteWebhookForWorkflowV4 error: %v", err)
		ctx.Err = e.ErrDeleteWebhook.AddErr(err)
		return
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, w.Project, "删除", "自定义工作流-webhook", w.Name, "", ctx.Logger)
	ctx.Err = workflow.DeleteWebhookForWorkflowV4(c.Param("workflowName"), c.Param("triggerName"), ctx.Logger)
}

func CreateJiraHookForWorkflowV4(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

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
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
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
	ctx.Err = workflow.UpdateJiraHookForWorkflowV4(c.Param("workflowName"), jira, ctx.Logger)
}

func DeleteJiraHookForWorkflowV4(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	w, err := workflow.FindWorkflowV4Raw(c.Param("workflowName"), ctx.Logger)
	if err != nil {
		ctx.Logger.Errorf("DeleteJiraHookForWorkflowV4 error: %v", err)
		ctx.Err = e.ErrDeleteJiraHook.AddErr(err)
		return
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, w.Project, "删除", "自定义工作流-jirahook", w.Name, "", ctx.Logger)
	ctx.Err = workflow.DeleteJiraHookForWorkflowV4(c.Param("workflowName"), c.Param("hookName"), ctx.Logger)
}

func CreateMeegoHookForWorkflowV4(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

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
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
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
	ctx.Err = workflow.UpdateMeegoHookForWorkflowV4(c.Param("workflowName"), jira, ctx.Logger)
}

func DeleteMeegoHookForWorkflowV4(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	w, err := workflow.FindWorkflowV4Raw(c.Param("workflowName"), ctx.Logger)
	if err != nil {
		ctx.Logger.Errorf("DeleteMeegoHookForWorkflowV4 error: %v", err)
		ctx.Err = e.ErrDeleteMeegoHook.AddErr(err)
		return
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, w.Project, "删除", "自定义工作流-meegohook", w.Name, "", ctx.Logger)
	ctx.Err = workflow.DeleteMeegoHookForWorkflowV4(c.Param("workflowName"), c.Param("hookName"), ctx.Logger)
}

func CreateGeneralHookForWorkflowV4(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

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
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
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
	ctx.Err = workflow.UpdateGeneralHookForWorkflowV4(c.Param("workflowName"), hook, ctx.Logger)
}

func DeleteGeneralHookForWorkflowV4(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	w, err := workflow.FindWorkflowV4Raw(c.Param("workflowName"), ctx.Logger)
	if err != nil {
		ctx.Logger.Errorf("DeleteGeneralHookForWorkflowV4 error: %v", err)
		ctx.Err = e.ErrDeleteGeneralHook.AddErr(err)
		return
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, w.Project, "删除", "自定义工作流-generalhook", w.Name, "", ctx.Logger)
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
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

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
	ctx.Err = workflow.CreateCronForWorkflowV4(c.Param("workflowName"), req, ctx.Logger)
}

func UpdateCronForWorkflowV4(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	req := new(commonmodels.Cronjob)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, req.WorkflowV4Args.Project, "更新", "自定义工作流-cron", req.WorkflowV4Args.Name, getBody(c), ctx.Logger)
	ctx.Err = workflow.UpdateCronForWorkflowV4(req, ctx.Logger)
}

func DeleteCronForWorkflowV4(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	w, err := workflow.FindWorkflowV4Raw(c.Param("workflowName"), ctx.Logger)
	if err != nil {
		ctx.Logger.Errorf("CreateCronForWorkflowV4 error: %v", err)
		ctx.Err = e.ErrUpsertCronjob.AddErr(err)
		return
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, w.Project, "删除", "自定义工作流-cron", w.Name, "", ctx.Logger)
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

func GetWorkflowGlabalVars(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(commonmodels.WorkflowV4)

	if err := c.ShouldBindYAML(&args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	ctx.Resp = workflow.GetWorkflowGlabalVars(args, c.Param("jobName"), ctx.Logger)
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

func getBody(c *gin.Context) string {
	b, err := c.GetRawData()
	if err != nil {
		return ""
	}
	return string(b)
}
