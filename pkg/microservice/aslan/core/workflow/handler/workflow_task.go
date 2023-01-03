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
	"io"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/jinzhu/copier"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/delivery/service"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/workflow/service/workflow"
	"github.com/koderover/zadig/pkg/setting"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/koderover/zadig/pkg/types/dto"
)

// GetWorkflowArgs find workflow args
func GetWorkflowArgs(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	args := []*workflow.ServiceBuildInfo{}
	if err := c.ShouldBindWith(&args, binding.JSON); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	ctx.Resp, ctx.Err = workflow.GetWorkflowArgs(c.Param("productName"), c.Param("namespace"), args, ctx.Logger)
}

// PresetWorkflowArgs find a workflow task
func PresetWorkflowArgs(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = workflow.PresetWorkflowArgs(c.Param("namespace"), c.Param("workflowName"), ctx.Logger)
}

// CreateWorkflowTask create a workflow task
func CreateWorkflowTask(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(commonmodels.WorkflowTaskArgs)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("CreateWorkflowTask c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("CreateWorkflowTask json.Unmarshal err : %v", err)
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, args.ProductTmplName, "新增", "工作流-task", args.WorkflowName, string(data), ctx.Logger)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	if err := c.ShouldBindWith(&args, binding.JSON); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	if args.WorkflowTaskCreator != setting.CronTaskCreator && args.WorkflowTaskCreator != setting.WebhookTaskCreator {
		args.WorkflowTaskCreator = ctx.UserName
	}

	ctx.Resp, ctx.Err = workflow.CreateWorkflowTask(args, args.WorkflowTaskCreator, ctx.Logger)

	// 发送通知
	if ctx.Err != nil {
		commonservice.SendFailedTaskMessage(ctx.UserName, args.ProductTmplName, args.WorkflowName, ctx.RequestID, config.WorkflowType, ctx.Err, ctx.Logger)
	}
}

// CreateArtifactWorkflowTask create a artifact workflow task
func CreateArtifactWorkflowTask(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(commonmodels.WorkflowTaskArgs)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("CreateArtifactWorkflowTask c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("CreateArtifactWorkflowTask json.Unmarshal err : %v", err)
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, args.ProductTmplName, "新增", "工作流-task", args.WorkflowName, string(data), ctx.Logger)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	if err := c.ShouldBindWith(&args, binding.JSON); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	if args.WorkflowTaskCreator != setting.CronTaskCreator && args.WorkflowTaskCreator != setting.WebhookTaskCreator {
		args.WorkflowTaskCreator = ctx.UserName
	}

	ctx.Resp, ctx.Err = workflow.CreateArtifactWorkflowTask(args, args.WorkflowTaskCreator, ctx.Logger)
}

// ListWorkflowTasksResult workflowtask分页信息
func ListWorkflowTasksResult(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

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
	filters := c.Query("filters")
	filtersList := strings.Split(filters, ",")
	ctx.Resp, ctx.Err = workflow.ListPipelineTasksV2Result(c.Param("name"), workflowTypeString, c.Query("queryType"), filtersList, maxResult, startAt, ctx.Logger)
}

func GetFiltersPipeline(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	if c.Query("workflowType") != string(config.WorkflowType) {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid workflowType")
		return
	}
	ctx.Resp, ctx.Err = workflow.GetFiltersPipelineTaskV2(c.Query("projectName"), c.Param("name"), c.Query("queryType"), config.WorkflowType, ctx.Logger)
}

func GetWorkflowTask(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

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
	task, err := workflow.GetPipelineTaskV2(taskID, c.Param("name"), workflowTypeString, ctx.Logger)
	if err != nil {
		ctx.Err = err
		return
	}
	releases, err := service.ListDeliveryVersion(&service.ListDeliveryVersionArgs{
		TaskId:       int(task.TaskID),
		ServiceName:  task.ServiceName,
		ProjectName:  task.ProductName,
		WorkflowName: task.PipelineName,
	}, ctx.Logger)
	if err != nil {
		ctx.Err = err
		return
	}

	var toReleases []dto.Release
	for _, v := range releases {
		toReleases = append(toReleases, dto.Release{
			ID:      v.VersionInfo.ID,
			Version: v.VersionInfo.Version,
		})
	}
	var toTask dto.Task
	if err := copier.Copy(&toTask, task); err != nil {
		ctx.Err = err
		return
	}
	toTask.Releases = toReleases
	ctx.Resp = toTask
	ctx.Err = err
	return
}

func RestartWorkflowTask(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	internalhandler.InsertOperationLog(c, ctx.UserName, c.Query("projectName"), "重启", "工作流-task", c.Param("name"), "", ctx.Logger)

	taskID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid task id")
		return
	}

	ctx.Err = workflow.RestartPipelineTaskV2(ctx.UserName, taskID, c.Param("name"), config.WorkflowType, ctx.Logger)
}

func CancelWorkflowTaskV2(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	internalhandler.InsertOperationLog(c, ctx.UserName, c.Query("projectName"), "取消", "工作流-task", c.Param("name"), "", ctx.Logger)

	taskID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid task id")
		return
	}
	ctx.Err = commonservice.CancelTaskV2(ctx.UserName, c.Param("name"), taskID, config.WorkflowType, ctx.RequestID, ctx.Logger)
}

func GetWorkflowTaskCallback(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	taskID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid task id")
		return
	}

	ctx.Resp, ctx.Err = commonservice.GetWorkflowTaskCallback(taskID, c.Param("name"))
}
