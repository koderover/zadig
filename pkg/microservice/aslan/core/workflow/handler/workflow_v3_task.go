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
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/workflow/service/workflow"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/log"
)

func CreateWorkflowTaskV3(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(commonmodels.WorkflowV3Args)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("CreateWorkflowTaskV3 c.GetRawData() err : %s", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("CreateWorkflowTaskV3 json.Unmarshal err : %s", err)
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, args.ProjectName, "创建", "工作流v3-task", args.Name, string(data), ctx.Logger)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	if err := c.ShouldBindJSON(&args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	ctx.Resp, ctx.Err = workflow.CreateWorkflowTaskV3(args, ctx.UserName, ctx.RequestID, ctx.Logger)
}

func RestartWorkflowTaskV3(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	internalhandler.InsertOperationLog(c, ctx.UserName, "", "重启", "工作流taskV3", c.Param("name"), "", ctx.Logger)

	taskID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid task id")
		return
	}

	ctx.Err = workflow.RestartWorkflowTaskV3(ctx.UserName, taskID, c.Param("name"), config.WorkflowTypeV3, ctx.Logger)
}

func CancelWorkflowTaskV3(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	internalhandler.InsertOperationLog(c, ctx.UserName, c.Query("projectName"), "取消", "工作流taskV3", c.Param("name"), "", ctx.Logger)

	taskID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid task id")
		return
	}
	ctx.Err = commonservice.CancelWorkflowTaskV3(ctx.UserName, c.Param("name"), taskID, config.WorkflowTypeV3, ctx.RequestID, ctx.Logger)
}

// ListWorkflowV3TasksResult workflowtask分页信息
func ListWorkflowV3TasksResult(c *gin.Context) {
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

	ctx.Resp, ctx.Err = workflow.ListWorkflowTasksV3Result(c.Param("name"), config.WorkflowTypeV3, maxResult, startAt, ctx.Logger)
}

func GetWorkflowTaskV3(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	taskID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid task id")
		return
	}

	ctx.Resp, ctx.Err = workflow.GetWorkflowTaskV3(taskID, c.Param("name"), config.WorkflowTypeV3, ctx.Logger)
}

type WebhookPayload struct {
	EventName   string        `json:"event_name"`
	ProjectName string        `json:"project_name"`
	TaskName    string        `json:"task_name"`
	TaskID      int64         `json:"task_id"`
	TaskOutput  []*TaskOutput `json:"task_output"`
	TaskEnvs    []*KeyVal     `json:"task_envs"`
}

type TaskOutput struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type KeyVal struct {
	Key          string `bson:"key"                 json:"key"`
	Value        string `bson:"value"               json:"value"`
	IsCredential bool   `bson:"is_credential"       json:"is_credential"`
}

func GetWorkflowTaskV3Callback(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	taskID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid task id")
		return
	}

	ctx.Resp, ctx.Err = workflow.GetWorkflowTaskV3Callback(taskID, c.Param("name"), ctx.Logger)
}
