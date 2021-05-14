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
	"strconv"

	"github.com/gin-contrib/sse"
	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/lib/microservice/aslan/config"
	logservice "github.com/koderover/zadig/lib/microservice/aslan/core/log/service"
	internalhandler "github.com/koderover/zadig/lib/microservice/aslan/internal/handler"
	e "github.com/koderover/zadig/lib/tool/errors"
)

func GetContainerLogsSSE(c *gin.Context) {
	logger := internalhandler.Logger(c)

	tails, err := strconv.ParseInt(c.Query("tails"), 10, 64)
	if err != nil {
		tails = int64(10)
	}

	envName := c.Query("envName")
	productName := c.Query("productName")

	internalhandler.Stream(c, func(ctx context.Context, streamChan chan interface{}) {
		logservice.ContainerLogStream(ctx, streamChan, envName, productName, c.Param("podName"), c.Param("containerName"), true, tails, logger)
	}, logger)
}

func GetBuildJobContainerLogsSSE(c *gin.Context) {
	ctx := internalhandler.NewContext(c)

	taskID, err := strconv.ParseInt(c.Param("taskId"), 10, 64)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid task id")
		internalhandler.JsonResponse(c, ctx)
		return
	}

	tails, err := strconv.ParseInt(c.Param("lines"), 10, 64)
	if err != nil {
		tails = int64(10)
	}
	subTask := c.Query("subTask")

	internalhandler.Stream(c, func(ctx1 context.Context, streamChan chan interface{}) {
		logservice.TaskContainerLogStream(
			ctx1, streamChan,
			&logservice.GetContainerOptions{
				Namespace:    config.Namespace(),
				PipelineName: c.Param("pipelineName"),
				SubTask:      subTask,
				TaskID:       taskID,
				TailLines:    tails,
				PipelineType: string(config.SingleType),
			},
			ctx.Logger)
	}, ctx.Logger)
}

func GetWorkflowBuildJobContainerLogsSSE(c *gin.Context) {
	ctx := internalhandler.NewContext(c)

	taskID, err := strconv.ParseInt(c.Param("taskId"), 10, 64)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid task id")
		internalhandler.JsonResponse(c, ctx)
		return
	}

	tails, err := strconv.ParseInt(c.Param("lines"), 10, 64)
	if err != nil {
		tails = int64(10)
	}

	subTask := c.Query("subTask")
	options := &logservice.GetContainerOptions{
		Namespace:    config.Namespace(),
		PipelineName: c.Param("pipelineName"),
		SubTask:      subTask,
		TailLines:    tails,
		TaskID:       taskID,
		ServiceName:  c.Param("serviceName"),
		PipelineType: string(config.WorkflowType),
		EnvName:      c.Query("envName"),
		ProductName:  c.Query("productName"),
	}

	internalhandler.Stream(c, func(ctx1 context.Context, streamChan chan interface{}) {
		logservice.TaskContainerLogStream(
			ctx1, streamChan,
			options,
			ctx.Logger)
	}, ctx.Logger)
}

func GetTestJobContainerLogsSSE(c *gin.Context) {
	ctx := internalhandler.NewContext(c)

	taskID, err := strconv.ParseInt(c.Param("taskId"), 10, 64)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid task id")
		internalhandler.JsonResponse(c, ctx)
		return
	}

	tails, err := strconv.ParseInt(c.Param("lines"), 10, 64)
	if err != nil {
		tails = int64(10)
	}

	options := &logservice.GetContainerOptions{
		Namespace:    config.Namespace(),
		PipelineName: c.Param("pipelineName"),
		TailLines:    tails,
		TaskID:       taskID,
		PipelineType: string(config.SingleType),
		TestName:     c.Param("testName"),
	}

	internalhandler.Stream(c, func(ctx1 context.Context, streamChan chan interface{}) {
		logservice.TestJobContainerLogStream(
			ctx1, streamChan,
			options,
			ctx.Logger)
	}, ctx.Logger)
}

func GetWorkflowTestJobContainerLogsSSE(c *gin.Context) {
	ctx := internalhandler.NewContext(c)

	taskID, err := strconv.ParseInt(c.Param("taskId"), 10, 64)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid task id")
		internalhandler.JsonResponse(c, ctx)
		return
	}

	tails, err := strconv.ParseInt(c.Param("lines"), 10, 64)
	if err != nil {
		tails = int64(10)
	}

	workflowTypeString := config.WorkflowType
	workflowType := c.Query("workflowType")
	if workflowType == string(config.TestType) {
		workflowTypeString = config.TestType
	}
	options := &logservice.GetContainerOptions{
		Namespace:    config.Namespace(),
		PipelineName: c.Param("pipelineName"),
		TailLines:    tails,
		TaskID:       taskID,
		PipelineType: string(workflowTypeString),
		ServiceName:  c.Param("serviceName"),
		TestName:     c.Param("testName"),
	}

	internalhandler.Stream(c, func(ctx1 context.Context, streamChan chan interface{}) {
		logservice.TestJobContainerLogStream(
			ctx1, streamChan,
			options,
			ctx.Logger)
	}, ctx.Logger)
}

func GetServiceJobContainerLogsSSE(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() {
		c.Render(-1, sse.Event{
			Event: "job-status",
			Data:  "completed",
		})
	}()

	tails, err := strconv.ParseInt(c.Query("lines"), 10, 64)
	if err != nil {
		tails = int64(10)
	}

	subTask := c.Query("subTask")
	options := &logservice.GetContainerOptions{
		Namespace:    config.Namespace(),
		SubTask:      subTask,
		TailLines:    tails,
		ServiceName:  c.Param("serviceName"),
		PipelineType: string(config.ServiceType),
		EnvName:      c.Param("envName"),
		ProductName:  c.Param("productName"),
	}

	internalhandler.Stream(c, func(ctx1 context.Context, streamChan chan interface{}) {
		logservice.TaskContainerLogStream(
			ctx1, streamChan,
			options,
			ctx.Logger)
	}, ctx.Logger)
}
