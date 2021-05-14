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
	"io/ioutil"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"

	"github.com/qiniu/x/log.v7"

	commonmodels "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
	"github.com/koderover/zadig/lib/microservice/aslan/core/workflow/service/workflow"
	internalhandler "github.com/koderover/zadig/lib/microservice/aslan/internal/handler"
	e "github.com/koderover/zadig/lib/tool/errors"
	"github.com/koderover/zadig/lib/types/permission"
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
	c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(data))
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
	defer func() { internalhandler.JsonResponse(c, ctx) }()

	ctx.Resp = workflow.AutoCreateWorkflow(c.Param("productName"), ctx.Logger)
}

func CreateWorkflow(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()

	args := new(commonmodels.Workflow)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("CreateWorkflow c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("CreateWorkflow json.Unmarshal err : %v", err)
	}
	internalhandler.InsertOperationLog(c, ctx.Username, args.ProductTmplName, "新增", "工作流", args.Name, permission.WorkflowCreateUUID, string(data), ctx.Logger)
	c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(data))

	if err := c.ShouldBindWith(&args, binding.JSON); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	args.UpdateBy = ctx.Username
	args.CreateBy = ctx.Username
	ctx.Err = workflow.CreateWorkflow(args, ctx.Logger)
}

// UpdateWorkflow  update a workflow
func UpdateWorkflow(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()

	args := new(commonmodels.Workflow)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("CreateWorkflow c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("UpdateWorkflow json.Unmarshal err : %v", err)
	}
	internalhandler.InsertOperationLog(c, ctx.Username, args.ProductTmplName, "更新", "工作流", args.Name, permission.WorkflowUpdateUUID, string(data), ctx.Logger)
	c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(data))

	if err := c.ShouldBindWith(&args, binding.JSON); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	args.UpdateBy = ctx.Username
	ctx.Err = workflow.UpdateWorkflow(args, ctx.Logger)
}

func ListWorkflows(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()

	ctx.Resp, ctx.Err = workflow.ListWorkflows(c.Query("type"), ctx.User.ID, ctx.Logger)
}

func ListAllWorkflows(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()

	ctx.Resp, ctx.Err = workflow.ListAllWorkflows(c.Param("testName"), ctx.User.ID, ctx.User.IsSuperUser, ctx.Logger)
}

// FindWorkflow find a workflow
func FindWorkflow(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()

	ctx.Resp, ctx.Err = workflow.FindWorkflow(c.Param("name"), ctx.Logger)
}

func DeleteWorkflow(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()

	internalhandler.InsertOperationLog(c, ctx.Username, c.GetString("productName"), "删除", "工作流", c.Param("name"), permission.WorkflowDeleteUUID, "", ctx.Logger)
	ctx.Err = workflow.DeleteWorkflow(c.Param("name"), false, ctx.Logger)
}

func PreSetWorkflow(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()

	ctx.Resp, ctx.Err = workflow.PreSetWorkflow(c.Param("productName"), ctx.Logger)
}

func CopyWorkflow(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()

	ctx.Err = workflow.CopyWorkflow(c.Param("old"), c.Param("new"), ctx.Username, ctx.Logger)
}
