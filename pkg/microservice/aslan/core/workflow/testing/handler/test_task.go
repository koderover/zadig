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
	"github.com/gin-gonic/gin/binding"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/notify"
	workflowservice "github.com/koderover/zadig/pkg/microservice/aslan/core/workflow/service/workflow"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/workflow/testing/service"
	"github.com/koderover/zadig/pkg/setting"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/log"
)

func CreateTestTask(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := new(commonmodels.TestTaskArgs)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("CreateTestTask c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("CreateTestTask json.Unmarshal err : %v", err)
	}
	projectKey := args.ProductName
	internalhandler.InsertOperationLog(c, ctx.UserName, projectKey, "新增", "测试-task", fmt.Sprintf("%s-%s", args.TestName, "job"), string(data), ctx.Logger)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	// authorization check
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectKey].Test.Execute {
			ctx.UnAuthorized = true
			return
		}
	}

	if err := c.ShouldBindWith(&args, binding.JSON); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	if args.TestTaskCreator != setting.CronTaskCreator && args.TestTaskCreator != setting.WebhookTaskCreator {
		args.TestTaskCreator = ctx.UserName
	}

	ctx.Resp, ctx.Err = service.CreateTestTask(args, ctx.Logger)
	if ctx.Err != nil {
		notify.SendFailedTaskMessage(ctx.UserName, args.ProductName, args.TestName, ctx.RequestID, config.TestType, ctx.Err, ctx.Logger)
	}
}

func RestartTestTask(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Param("productName")

	internalhandler.InsertOperationLog(c, ctx.UserName, projectKey, "重启", "测试任务", c.Param("name"), "", ctx.Logger)

	// authorization check
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectKey].Test.Execute {
			ctx.UnAuthorized = true
			return
		}
	}

	taskID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid task id")
		return
	}

	ctx.Err = workflowservice.RestartPipelineTaskV2(ctx.UserName, taskID, c.Param("name"), config.TestType, ctx.Logger)
}

func CancelTestTaskV2(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Param("productName")
	internalhandler.InsertOperationLog(c, ctx.UserName, projectKey, "取消", "测试任务", c.Param("name"), "", ctx.Logger)

	// authorization check
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectKey].Test.Execute {
			ctx.UnAuthorized = true
			return
		}
	}

	taskID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid task id")
		return
	}
	ctx.Err = commonservice.CancelTaskV2(ctx.UserName, c.Param("name"), taskID, config.TestType, ctx.RequestID, ctx.Logger)
}
