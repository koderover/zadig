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
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/workflow/service/workflow"
	"github.com/koderover/zadig/pkg/setting"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/log"
)

func GetProductNameByPipelineTask(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	args := new(commonmodels.TaskArgs)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("c.GetRawData() err : %v", err)
		return
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("json.Unmarshal err : %v", err)
		return
	}

	pipeline, err := workflow.GetPipeline(ctx.UserID, args.PipelineName, ctx.Logger)
	if err != nil {
		log.Errorf("GetProductNameByPipelineTask err : %v", err)
		return
	}
	c.Set("productName", pipeline.ProductName)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))
	c.Next()
}

// CreatePipelineTask ...
func CreatePipelineTask(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(commonmodels.TaskArgs)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("CreatePipelineTask c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("CreatePipelineTask json.Unmarshal err : %v", err)
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, c.GetString("productName"), "新增", "单服务-工作流task", args.PipelineName, string(data), ctx.Logger)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	if err := c.BindJSON(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid PipelineTaskArgs")
		return
	}
	if args.TaskCreator == "" {
		args.TaskCreator = ctx.UserName
	}

	args.ReqID = ctx.RequestID
	ctx.Resp, ctx.Err = workflow.CreatePipelineTask(args, ctx.Logger)

	// 发送通知
	if ctx.Err != nil {
		commonservice.SendFailedTaskMessage(ctx.UserName, args.ProductName, args.PipelineName, ctx.RequestID, config.SingleType, ctx.Err, ctx.Logger)
	}

}

// ListPipelineTasksResult pipelinetask分页信息
func ListPipelineTasksResult(c *gin.Context) {
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
	ctx.Resp, ctx.Err = workflow.ListPipelineTasksV2Result(c.Param("name"), config.SingleType, "", []string{}, maxResult, startAt, ctx.Logger)
}

func GetPipelineTask(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	taskID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid task id")
		return
	}

	ctx.Resp, ctx.Err = workflow.GetPipelineTaskV2(taskID, c.Param("name"), config.SingleType, ctx.Logger)
}

func RestartPipelineTask(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	internalhandler.InsertOperationLog(c, ctx.UserName, c.Query("projectName"), "重启", "单服务-工作流task", c.Param("name"), "", ctx.Logger)

	taskID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid task id")
		return
	}

	ctx.Err = workflow.RestartPipelineTaskV2(ctx.UserName, taskID, c.Param("name"), config.SingleType, ctx.Logger)
}

func CancelTaskV2(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	internalhandler.InsertOperationLog(c, ctx.UserName, c.Query("projectName"), "取消", "单服务-工作流task", c.Param("name"), "", ctx.Logger)

	taskID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid task id")
		return
	}
	ctx.Err = commonservice.CancelTaskV2(ctx.UserName, c.Param("name"), taskID, config.SingleType, ctx.RequestID, ctx.Logger)
}

// ListPipelineUpdatableProductNames 启动任务时检查部署环境
func ListPipelineUpdatableProductNames(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = workflow.ListPipelineUpdatableProductNames(ctx.UserName, c.Param("name"), ctx.Logger)
}

func GetPackageFile(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	pipelineName := c.Query("pipelineName")
	if pipelineName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid pipelineName")
		return
	}

	taskID, err := strconv.ParseInt(c.Query("taskId"), 10, 64)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid taskId")
		return
	}

	resp, pkgFile, err := workflow.GePackageFileContent(pipelineName, taskID, ctx.Logger)
	if err != nil {
		ctx.Err = err
		return
	}
	c.Writer.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, pkgFile))
	c.Data(200, "application/octet-stream", resp)
}

func GetArtifactFile(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	taskID, err := strconv.ParseInt(c.Param("taskId"), 10, 64)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid task id")
		return
	}
	notHistoryFileFlag, err := strconv.ParseBool(c.DefaultQuery("notHistoryFileFlag", "false"))
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid notHistoryFileFlag")
		return
	}

	resp, err := workflow.GetArtifactFileContent(c.Param("pipelineName"), taskID, notHistoryFileFlag, ctx.Logger)
	if err != nil {
		ctx.Err = err
		return
	}
	if notHistoryFileFlag {
		c.Writer.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, setting.ArtifactResultOut))
	} else {
		c.Writer.Header().Set("Content-Disposition", `attachment; filename="artifact.tar.gz"`)
	}

	c.Data(200, "application/octet-stream", resp)
}
