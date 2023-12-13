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
)

func OpenAPICreateScanningModule(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(testingservice.OpenAPICreateScanningReq)
	err := c.BindJSON(args)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
	}

	isValid, err := args.Validate()
	if !isValid {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	ctx.Err = testingservice.OpenAPICreateScanningModule(ctx.UserName, args, ctx.Logger)
}

func OpenAPICreateScanningTask(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(testingservice.OpenAPICreateScanningTaskReq)
	err := c.BindJSON(args)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
	}
	args.ProjectName = c.Query("projectKey")
	args.ScanName = c.Param("scanName")
	isValid, err := args.Validate()
	if !isValid {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	data, err := c.GetRawData()
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, args.ProjectName, "OpenAPI"+"新增", "代码扫描任务", args.ScanName, string(data), ctx.Logger)

	taskID, err := testingservice.OpenAPICreateScanningTask(ctx.UserName, args, ctx.Logger)
	ctx.Resp = testingservice.OpenAPICreateScanningTaskResp{
		TaskID: taskID,
	}
	ctx.Err = err
}

func OpenAPIGetScanningTaskDetail(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	scanName := c.Param("scanName")
	projectKey := c.Query("projectKey")
	taskID, err := strconv.ParseInt(c.Param("taskID"), 10, 64)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid param taskID")
		return
	}
	if taskID == 0 || projectKey == "" || scanName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid params")
		return
	}

	ctx.Resp, ctx.Err = testingservice.OpenAPIGetScanningTaskDetail(taskID, projectKey, scanName, ctx.Logger)
}

func OpenAPICreateTestTask(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(testingservice.OpenAPICreateTestTaskReq)
	data, err := c.GetRawData()
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	if err = json.Unmarshal(data, args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	isValid, err := args.Validate()
	if !isValid {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, args.ProjectName, "新增", "OpenAPI"+"测试-task", fmt.Sprintf("%s-%s", args.TestName, "job"), string(data), ctx.Logger)

	taskID, err := testingservice.OpenAPICreateTestTask(ctx.UserName, args, ctx.Logger)
	ctx.Resp = testingservice.OpenAPICreateTestTaskResp{
		TaskID: taskID,
	}
	ctx.Err = err
}

func OpenAPIGetTestTaskResult(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	projectKey := c.Query("projectKey")
	testName := c.Param("testName")
	taskID, err := strconv.ParseInt(c.Param("taskID"), 10, 64)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid param taskID")
		return
	}
	if taskID == 0 || projectKey == "" || testName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid params")
		return
	}

	ctx.Resp, ctx.Err = testingservice.OpenAPIGetTestTaskResult(taskID, projectKey, testName, ctx.Logger)
}
