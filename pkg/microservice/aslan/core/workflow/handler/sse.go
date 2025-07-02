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
	"time"

	"github.com/gin-gonic/gin"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/workflowcontroller"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
)

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
