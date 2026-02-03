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

	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
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
		logservice.ContainerLogStream(ctx1, streamChan, envName, productName, podName, containerName, follow, tailLines, nil, ctx.Logger)
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

// @Summary Get Delivery Version logs
// @Description Get Delivery Version logs
// @Tags 	logs
// @Accept 	json
// @Produce json
// @Param 	projectName 		query 		string							true	"项目标识"
// @Param 	version 			query 		string							true	"版本名称"
// @Param 	serviceName 		query 		string							true	"服务名"
// @Param 	serviceModule 		query 		string							true	"服务组件"
// @Success 200     			{string} 	string
// @Router /api/aslan/logs/log/delivery [get]
func GetDeliveryVersionLogs(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	projectName := c.Query("projectName")
	if projectName == "" {
		ctx.RespErr = fmt.Errorf("projectName must be provided")
		return
	}

	versionName := c.Query("version")
	if versionName == "" {
		ctx.RespErr = fmt.Errorf("version must be provided")
		return
	}

	serviceName := c.Query("serviceName")
	if serviceName == "" {
		ctx.RespErr = fmt.Errorf("serviceName must be provided")
		return
	}

	serviceModule := c.Query("serviceModule")
	if serviceModule == "" {
		ctx.RespErr = fmt.Errorf("serviceModule must be provided")
		return
	}

	version, err := commonrepo.NewDeliveryVersionV2Coll().Find(projectName, versionName)
	if err != nil {
		ctx.RespErr = fmt.Errorf("failed to find delivery version: %s", err)
		return
	}

	jobName, err := findDeliveryVersionJobName(ctx, version.WorkflowName, version.TaskID, serviceName, serviceModule)
	if err != nil {
		ctx.RespErr = fmt.Errorf("failed to find job name: %s", err)
		return
	}

	// Use all lowercase job names to avoid subdomain errors
	ctx.Resp, ctx.RespErr = logservice.GetWorkflowV4JobContainerLogs(strings.ToLower(version.WorkflowName), jobName, version.TaskID, ctx.Logger)
}
