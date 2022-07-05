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
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/copier"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/delivery/service"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/workflow/service/workflow"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/workflow/service/workflowcontroller"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/types/dto"
)

func GetPipelineTaskSSE(c *gin.Context) {
	ctx := internalhandler.NewContext(c)

	taskID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("Invalid id Args")
		internalhandler.JSONResponse(c, ctx)
		return
	}

	internalhandler.Stream(c, func(ctx1 context.Context, msgChan chan interface{}) {
		startTime := time.Now()
		err := wait.PollImmediateUntil(time.Second, func() (bool, error) {
			res, err := workflow.GetPipelineTaskV2(taskID, c.Param("name"), config.SingleType, ctx.Logger)
			if err != nil {
				ctx.Logger.Errorf("[%s] GetPipelineTaskSSE error: %v", ctx.UserName, err)
				return false, err
			}

			msgChan <- res

			if time.Since(startTime).Minutes() == float64(60) {
				ctx.Logger.Warnf("[%s] Query GetPipelineTaskSSE API over 60 minutes", ctx.UserName)
			}

			return false, nil
		}, ctx1.Done())

		if err != nil && err != wait.ErrWaitTimeout {
			ctx.Logger.Error(err)
		}
	}, ctx.Logger)
}

func RunningPipelineTasksSSE(c *gin.Context) {
	ctx := internalhandler.NewContext(c)

	internalhandler.Stream(c, func(ctx1 context.Context, msgChan chan interface{}) {
		startTime := time.Now()
		wait.NonSlidingUntilWithContext(ctx1, func(_ context.Context) {
			msgChan <- workflow.RunningPipelineTasks()

			if time.Since(startTime).Minutes() == float64(60) {
				ctx.Logger.Warnf("[%s] Query RunningPipelineTasksSSE API over 60 minutes", ctx.UserName)
			}
		}, time.Second)
	}, ctx.Logger)
}

func PendingPipelineTasksSSE(c *gin.Context) {
	ctx := internalhandler.NewContext(c)

	internalhandler.Stream(c, func(ctx1 context.Context, msgChan chan interface{}) {
		startTime := time.Now()
		wait.NonSlidingUntilWithContext(ctx1, func(_ context.Context) {
			msgChan <- workflow.PendingPipelineTasks()

			if time.Since(startTime).Minutes() == float64(60) {
				ctx.Logger.Warnf("[%s] Query PendingPipelineTasksSSE API over 60 minutes", ctx.UserName)
			}
		}, time.Second)
	}, ctx.Logger)
}

func RunningWorkflowTasksSSE(c *gin.Context) {
	ctx := internalhandler.NewContext(c)

	internalhandler.Stream(c, func(ctx1 context.Context, msgChan chan interface{}) {
		startTime := time.Now()
		wait.NonSlidingUntilWithContext(ctx1, func(_ context.Context) {
			msgChan <- workflowcontroller.RunningTasks()

			if time.Since(startTime).Minutes() == float64(60) {
				ctx.Logger.Warnf("[%s] Query RunningPipelineTasksSSE API over 60 minutes", ctx.UserName)
			}
		}, time.Second)
	}, ctx.Logger)
}

func PendingWorkflowTasksSSE(c *gin.Context) {
	ctx := internalhandler.NewContext(c)

	internalhandler.Stream(c, func(ctx1 context.Context, msgChan chan interface{}) {
		startTime := time.Now()
		wait.NonSlidingUntilWithContext(ctx1, func(_ context.Context) {
			msgChan <- workflowcontroller.PendingTasks()

			if time.Since(startTime).Minutes() == float64(60) {
				ctx.Logger.Warnf("[%s] Query PendingPipelineTasksSSE API over 60 minutes", ctx.UserName)
			}
		}, time.Second)
	}, ctx.Logger)
}

func GetWorkflowTaskSSE(c *gin.Context) {
	ctx := internalhandler.NewContext(c)

	taskID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("Invalid id Args")
		internalhandler.JSONResponse(c, ctx)
		return
	}

	workflowTypeString := config.WorkflowType
	workflowType := c.Query("workflowType")
	if workflowType == string(config.TestType) {
		workflowTypeString = config.TestType
	}

	internalhandler.Stream(c, func(ctx1 context.Context, msgChan chan interface{}) {
		startTime := time.Now()
		err := wait.PollImmediateUntil(time.Second, func() (bool, error) {
			res, err := workflow.GetPipelineTaskV2(taskID, c.Param("name"), workflowTypeString, ctx.Logger)
			if err != nil {
				ctx.Logger.Errorf("[%s] GetPipelineTaskSSE error: %v", ctx.UserName, err)
				return false, err
			}
			releases, err := service.ListDeliveryVersion(&service.ListDeliveryVersionArgs{
				TaskId:       int(res.TaskID),
				ServiceName:  res.ServiceName,
				ProjectName:  res.ProductName,
				WorkflowName: res.PipelineName,
			}, ctx.Logger)

			re := []dto.Release{}
			for _, v := range releases {
				re = append(re, dto.Release{
					ID:      v.VersionInfo.ID,
					Version: v.VersionInfo.Version,
				})
			}
			var task dto.Task
			if err := copier.Copy(&task, res); err != nil {
				return false, err
			}
			task.Releases = re

			msgChan <- task

			if time.Since(startTime).Minutes() == float64(60) {
				ctx.Logger.Warnf("[%s] Query GetPipelineTaskSSE API over 60 minutes", ctx.UserName)
			}

			return false, nil
		}, ctx1.Done())

		if err != nil && err != wait.ErrWaitTimeout {
			ctx.Logger.Error(err)
		}
	}, ctx.Logger)
}

func GetWorkflowTaskV3SSE(c *gin.Context) {
	ctx := internalhandler.NewContext(c)

	taskID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("Invalid id Args")
		internalhandler.JSONResponse(c, ctx)
		return
	}

	internalhandler.Stream(c, func(ctx1 context.Context, msgChan chan interface{}) {
		startTime := time.Now()
		err := wait.PollImmediateUntil(time.Second, func() (bool, error) {
			res, err := workflow.GetWorkflowTaskV3(taskID, c.Param("name"), config.WorkflowTypeV3, ctx.Logger)
			if err != nil {
				ctx.Logger.Errorf("[%s] GetWorkflowTaskV3SSE error: %s", ctx.UserName, err)
				return false, err
			}

			msgChan <- res

			if time.Since(startTime).Minutes() == float64(60) {
				ctx.Logger.Warnf("[%s] Query GetWorkflowTaskV3SSE API over 60 minutes", ctx.UserName)
			}

			return false, nil
		}, ctx1.Done())

		if err != nil && err != wait.ErrWaitTimeout {
			ctx.Logger.Error(err)
		}
	}, ctx.Logger)
}
