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
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	logservice "github.com/koderover/zadig/lib/microservice/aslan/core/log/service"
	internalhandler "github.com/koderover/zadig/lib/microservice/aslan/internal/handler"
	e "github.com/koderover/zadig/lib/tool/errors"
)

func GetBuildJobContainerLogs(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()

	taskID, err := strconv.ParseInt(c.Param("taskId"), 10, 64)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid task id")
		return
	}
	// job名称使用全小写，避免出现subdomain错误
	ctx.Resp, ctx.Err = logservice.GetBuildJobContainerLogs(
		c.Param("pipelineName"),
		c.Param("serviceName"),
		taskID,
		ctx.Logger,
	)
}

func GetWorkflowBuildJobContainerLogs(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()

	taskID, err := strconv.ParseInt(c.Param("taskId"), 10, 64)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid task id")
		return
	}
	// job名称使用全小写，避免出现subdomain错误
	ctx.Resp, ctx.Err = logservice.GetWorkflowBuildJobContainerLogs(strings.ToLower(c.Param("pipelineName")), c.Param("serviceName"), c.Query("type"), taskID, ctx.Logger)
}

func GetTestJobContainerLogs(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()

	taskID, err := strconv.ParseInt(c.Param("taskId"), 10, 64)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid task id")
		return
	}

	// job名称使用全小写，避免出现subdomain错误
	ctx.Resp, ctx.Err = logservice.GetTestJobContainerLogs(c.Param("pipelineName"), c.Param("testName"), taskID, ctx.Logger)
}

func GetWorkflowTestJobContainerLogs(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JsonResponse(c, ctx) }()

	taskID, err := strconv.ParseInt(c.Param("taskId"), 10, 64)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid task id")
		return
	}

	// job名称使用全小写，避免出现subdomain错误
	ctx.Resp, ctx.Err = logservice.GetWorkflowTestJobContainerLogs(c.Param("pipelineName"), c.Param("serviceName"), c.Query("workflowType"), taskID, ctx.Logger)
}
