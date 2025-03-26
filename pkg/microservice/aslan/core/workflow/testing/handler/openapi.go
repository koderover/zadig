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
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"
	testingservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/workflow/testing/service"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/types"
)

func OpenAPICreateScanningModule(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := new(testingservice.OpenAPICreateScanningReq)
	err = c.BindJSON(args)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
	}

	isValid, err := args.Validate()
	if !isValid {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	// authorization check
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[args.ProjectName]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if !ctx.Resources.ProjectAuthInfo[args.ProjectName].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[args.ProjectName].Scanning.Create {
			ctx.UnAuthorized = true
			return
		}
	}

	ctx.RespErr = testingservice.OpenAPICreateScanningModule(ctx.UserName, args, ctx.Logger)
}

func OpenAPICreateScanningTask(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := new(testingservice.OpenAPICreateScanningTaskReq)
	err = c.BindJSON(args)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
	}
	args.ProjectName = c.Query("projectKey")
	args.ScanName = c.Param("scanName")
	isValid, err := args.Validate()
	if !isValid {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	data, err := c.GetRawData()
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, args.ProjectName, "OpenAPI"+"新增", "代码扫描任务", args.ScanName, string(data), types.RequestBodyTypeJSON, ctx.Logger)

	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[args.ProjectName]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if !ctx.Resources.ProjectAuthInfo[args.ProjectName].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[args.ProjectName].Scanning.Execute {
			ctx.UnAuthorized = true
			return
		}
	}

	taskID, err := testingservice.OpenAPICreateScanningTask(ctx.UserName, ctx.Account, ctx.UserID, args, ctx.Logger)
	ctx.Resp = testingservice.OpenAPICreateScanningTaskResp{
		TaskID: taskID,
	}
	ctx.RespErr = err
}

func OpenAPIGetScanningTaskDetail(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	scanName := c.Param("scanName")
	projectKey := c.Query("projectKey")
	taskID, err := strconv.ParseInt(c.Param("taskID"), 10, 64)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid param taskID")
		return
	}
	if taskID == 0 || projectKey == "" || scanName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid params")
		return
	}

	ctx.Resp, ctx.RespErr = testingservice.OpenAPIGetScanningTaskDetail(taskID, projectKey, scanName, ctx.Logger)
}

func OpenAPICreateTestTask(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := new(testingservice.OpenAPICreateTestTaskReq)
	data, err := c.GetRawData()
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	if err = json.Unmarshal(data, args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	isValid, err := args.Validate()
	if !isValid {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, args.ProjectName, "新增", "OpenAPI"+"测试-task", fmt.Sprintf("%s-%s", args.TestName, "job"), string(data), types.RequestBodyTypeJSON, ctx.Logger)

	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[args.ProjectName]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if !ctx.Resources.ProjectAuthInfo[args.ProjectName].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[args.ProjectName].Test.Execute {
			ctx.UnAuthorized = true
			return
		}
	}

	taskID, err := testingservice.OpenAPICreateTestTask(ctx.UserName, ctx.Account, ctx.UserID, args, ctx.Logger)
	ctx.Resp = testingservice.OpenAPICreateTestTaskResp{
		TaskID: taskID,
	}
	ctx.RespErr = err
}

func OpenAPIGetTestTaskResult(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	projectKey := c.Query("projectKey")
	testName := c.Param("testName")
	taskID, err := strconv.ParseInt(c.Param("taskID"), 10, 64)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid param taskID")
		return
	}
	if taskID == 0 || projectKey == "" || testName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid params")
		return
	}

	ctx.Resp, ctx.RespErr = testingservice.OpenAPIGetTestTaskResult(taskID, projectKey, testName, ctx.Logger)
}
