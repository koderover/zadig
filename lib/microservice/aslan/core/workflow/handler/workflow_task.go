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
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"

	"github.com/qiniu/x/log.v7"

	"github.com/koderover/zadig/lib/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
	commonservice "github.com/koderover/zadig/lib/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/lib/microservice/aslan/core/workflow/service/workflow"
	internalhandler "github.com/koderover/zadig/lib/microservice/aslan/internal/handler"
	"github.com/koderover/zadig/lib/setting"
	e "github.com/koderover/zadig/lib/tool/errors"
	"github.com/koderover/zadig/lib/types/permission"
)

func GetWorkflowTaskProductName(c *gin.Context) {
	args := new(commonmodels.WorkflowTaskArgs)
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

func GetWorkflowTaskProductNameByTask(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	taskID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid task id")
		return
	}

	pipelineTask, err := workflow.GetPipelineTaskV2(taskID, c.Param("name"), config.WorkflowType, ctx.Logger)
	if err != nil {
		log.Errorf("GetPipelineTaskV2 err : %v", err)
		return
	}
	c.Set("productName", pipelineTask.WorkflowArgs.ProductTmplName)
	c.Next()
}

// GetWorkflowArgs find workflow args
func GetWorkflowArgs(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()

	ctx.Resp, ctx.Err = workflow.GetWorkflowArgs(c.Param("productName"), c.Param("namespace"), ctx.Logger)
}

// PresetWorkflowArgs find a workflow task
func PresetWorkflowArgs(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()

	ctx.Resp, ctx.Err = workflow.PresetWorkflowArgs(c.Param("namespace"), c.Param("workflowName"), ctx.Logger)
}

// CreateWorkflowTask create a workflow task
func CreateWorkflowTask(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()

	args := new(commonmodels.WorkflowTaskArgs)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("CreateWorkflowTask c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("CreateWorkflowTask json.Unmarshal err : %v", err)
	}
	internalhandler.InsertOperationLog(c, ctx.Username, args.ProductTmplName, "新增", "工作流-task", args.WorkflowName, permission.WorkflowTaskUUID, string(data), ctx.Logger)
	c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(data))

	if err := c.ShouldBindWith(&args, binding.JSON); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	if args.WorklowTaskCreator != setting.CronTaskCreator && args.WorklowTaskCreator != setting.WebhookTaskCreator {
		args.WorklowTaskCreator = ctx.Username
	}

	ctx.Resp, ctx.Err = workflow.CreateWorkflowTask(args, args.WorklowTaskCreator, ctx.User.ID, ctx.User.IsSuperUser, ctx.Logger)

	// 发送通知
	if ctx.Err != nil {
		commonservice.SendFailedTaskMessage(ctx.Username, args.ProductTmplName, args.WorkflowName, config.WorkflowType, ctx.Err, ctx.Logger)
	}
}

// CreateArtifactWorkflowTask create a artifact workflow task
func CreateArtifactWorkflowTask(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()

	args := new(commonmodels.WorkflowTaskArgs)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("CreateArtifactWorkflowTask c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("CreateArtifactWorkflowTask json.Unmarshal err : %v", err)
	}
	internalhandler.InsertOperationLog(c, ctx.Username, args.ProductTmplName, "新增", "工作流-task", args.WorkflowName, permission.WorkflowTaskUUID, string(data), ctx.Logger)
	c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(data))

	if err := c.ShouldBindWith(&args, binding.JSON); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	if args.WorklowTaskCreator != setting.CronTaskCreator && args.WorklowTaskCreator != setting.WebhookTaskCreator {
		args.WorklowTaskCreator = ctx.Username
	}

	ctx.Resp, ctx.Err = workflow.CreateArtifactWorkflowTask(args, args.WorklowTaskCreator, ctx.User.ID, ctx.User.IsSuperUser, ctx.Logger)
}

// ListWorkflowTasksResult workflowtask分页信息
func ListWorkflowTasksResult(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()

	maxResult, err := strconv.Atoi(c.Param("max"))
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid max result number")
		return
	}
	startAt, err := strconv.Atoi(c.Param("start"))
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid start at number")
		return
	}
	workflowTypeString := config.WorkflowType
	workflowType := c.Query("workflowType")
	if workflowType == string(config.TestType) {
		workflowTypeString = config.TestType
	}
	ctx.Resp, ctx.Err = workflow.ListPipelineTasksV2Result(c.Param("name"), workflowTypeString, maxResult, startAt, ctx.Logger)
}

func GetWorkflowTask(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()

	taskID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid task id")
		return
	}
	workflowTypeString := config.WorkflowType
	workflowType := c.Query("workflowType")
	if workflowType == string(config.TestType) {
		workflowTypeString = config.TestType
	}
	ctx.Resp, ctx.Err = workflow.GetPipelineTaskV2(taskID, c.Param("name"), workflowTypeString, ctx.Logger)
}

func RestartWorkflowTask(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()
	internalhandler.InsertOperationLog(c, ctx.Username, c.GetString("productName"), "重启", "工作流-task", c.Param("name"), permission.WorkflowTaskUUID, "", ctx.Logger)

	taskID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid task id")
		return
	}

	ctx.Err = workflow.RestartPipelineTaskV2(ctx.Username, taskID, c.Param("name"), config.WorkflowType, ctx.Logger)
}

func CancelWorkflowTaskV2(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()
	internalhandler.InsertOperationLog(c, ctx.Username, c.GetString("productName"), "取消", "工作流-task", c.Param("name"), permission.WorkflowTaskUUID, "", ctx.Logger)

	taskID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid task id")
		return
	}
	ctx.Err = commonservice.CancelTaskV2(ctx.Username, c.Param("name"), taskID, config.WorkflowType, ctx.Logger)
}
