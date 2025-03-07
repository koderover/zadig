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
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	logservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/log/service"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

func GetWorkflowV4JobContainerLogs(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	taskID, err := strconv.ParseInt(c.Param("taskID"), 10, 64)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid task id")
		return
	}
	// Use all lowercase job names to avoid subdomain errors
	ctx.Resp, ctx.RespErr = logservice.GetWorkflowV4JobContainerLogs(strings.ToLower(c.Param("workflowName")), c.Param("jobName"), taskID, ctx.Logger)
}

func GetContainerLogs(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	podName := c.Param("name")
	containerName := c.Query("container")
	envName := c.Query("envName")
	productName := c.Query("projectName")

	tailLines, err := strconv.ParseInt(c.Query("tailLines"), 10, 64)
	if err != nil {
		tailLines = int64(-1)
	}

	follow, err := strconv.ParseBool(c.Query("follow"))
	if err != nil {
		follow = false
	}

	if !follow {
		ctx.Resp, ctx.RespErr = logservice.GetCurrentContainerLogs(podName, containerName, envName, productName, tailLines, ctx.Logger)
		return
	}

	internalhandler.Stream(c, func(ctx1 context.Context, streamChan chan interface{}) {
		logservice.ContainerLogStream(ctx1, streamChan, envName, productName, podName, containerName, follow, tailLines, ctx.Logger)
	}, ctx.Logger)
}

func GetScanningContainerLogs(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	id := c.Param("id")
	if id == "" {
		ctx.RespErr = fmt.Errorf("id must be provided")
		return
	}

	taskIDStr := c.Param("scan_id")
	if taskIDStr == "" {
		ctx.RespErr = fmt.Errorf("scan_id must be provided")
		return
	}

	taskID, err := strconv.ParseInt(taskIDStr, 10, 64)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid task id")
		return
	}

	ctx.Resp, ctx.RespErr = logservice.GetScanningContainerLogs(id, taskID, ctx.Logger)
}

func GetTestingContainerLogs(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	testName := c.Param("test_name")
	if testName == "" {
		ctx.RespErr = fmt.Errorf("testName must be provided")
		return
	}

	taskIDStr := c.Param("task_id")
	if taskIDStr == "" {
		ctx.RespErr = fmt.Errorf("task_id must be provided")
		return
	}

	taskID, err := strconv.ParseInt(taskIDStr, 10, 64)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid task id")
		return
	}

	ctx.Resp, ctx.RespErr = logservice.GetTestingContainerLogs(testName, taskID, ctx.Logger)
}

func OpenAPIGetWorkflowV4JobContainerLogs(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	taskID, err := strconv.ParseInt(c.Param("taskID"), 10, 64)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid task id")
		return
	}
	// Use all lowercase job names to avoid subdomain errors
	ctx.Resp, ctx.RespErr = logservice.GetWorkflowV4JobContainerLogs(strings.ToLower(c.Param("workflowName")), c.Param("jobName"), taskID, ctx.Logger)
}