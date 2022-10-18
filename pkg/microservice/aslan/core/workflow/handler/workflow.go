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
	"io"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/workflow/service/workflow"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/log"
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
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp = workflow.AutoCreateWorkflow(c.Param("productName"), ctx.Logger)
}

func CreateWorkflow(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(commonmodels.Workflow)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("CreateWorkflow c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("CreateWorkflow json.Unmarshal err : %v", err)
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, args.ProductTmplName, "新增", "工作流", args.Name, string(data), ctx.Logger)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

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
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(commonmodels.Workflow)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("CreateWorkflow c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("UpdateWorkflow json.Unmarshal err : %v", err)
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, args.ProductTmplName, "更新", "工作流", args.Name, string(data), ctx.Logger)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

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

func ListTestWorkflows(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = workflow.ListTestWorkflows(c.Param("testName"), c.QueryArray("projects"), ctx.Logger)
}

// FindWorkflow find a workflow
func FindWorkflow(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = workflow.FindWorkflow(c.Param("name"), ctx.Logger)
}

func DeleteWorkflow(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	internalhandler.InsertOperationLog(c, ctx.UserName, c.GetString("productName"), "删除", "工作流", c.Param("name"), "", ctx.Logger)
	ctx.Err = commonservice.DeleteWorkflow(c.Param("name"), ctx.RequestID, false, ctx.Logger)
}

func PreSetWorkflow(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = workflow.PreSetWorkflow(c.Param("productName"), ctx.Logger)
}

func CopyWorkflow(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Err = workflow.CopyWorkflow(c.Param("old"), c.Param("new"), c.Param("newDisplay"), ctx.UserName, ctx.Logger)
}
