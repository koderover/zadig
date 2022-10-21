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
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("UpdateWorkflowV4 c.GetRawData() err : %s", err)
	}
	if err = yaml.Unmarshal(data, args); err != nil {
		log.Errorf("UpdateWorkflowV4 json.Unmarshal err : %s", err)
	}

	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	if err := c.ShouldBindYAML(&args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	ctx.Err = workflow.UpdateWorkflowV4(c.Param("name"), ctx.UserName, args, ctx.Logger)
}

func DeleteWorkflowV4(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

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

	ctx.Resp, ctx.Err = workflow.GetWorkflowv4Preset(c.Query("encryptedKey"), c.Param("name"), ctx.Logger)
}

func GetWebhookForWorkflowV4Preset(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = workflow.GetWebhookForWorkflowV4Preset(c.Query("workflowName"), c.Query("triggerName"), ctx.Logger)
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
	ctx.Err = workflow.UpdateWebhookForWorkflowV4(c.Param("workflowName"), req, ctx.Logger)
}

func DeleteWebhookForWorkflowV4(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Err = workflow.DeleteWebhookForWorkflowV4(c.Param("workflowName"), c.Param("triggerName"), ctx.Logger)
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
	ctx.Err = workflow.UpdateCronForWorkflowV4(req, ctx.Logger)
}

func DeleteCronForWorkflowV4(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

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
