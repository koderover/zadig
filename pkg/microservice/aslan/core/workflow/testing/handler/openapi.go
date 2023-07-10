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

	testingservice "github.com/koderover/zadig/pkg/microservice/aslan/core/workflow/testing/service"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	e "github.com/koderover/zadig/pkg/tool/errors"
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

func OpenAPICreateTestTask(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(testingservice.OpenAPICreateTestTaskReq)
	data, err := c.GetRawData()
	if err != nil {
		ctx.Logger.Errorf("CreateTestTask c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		ctx.Logger.Errorf("CreateTestTask json.Unmarshal err : %v", err)
	}

	isValid, err := args.Validate()
	if !isValid {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, args.ProjectKey, "新增", "OpenAPI"+"测试-task", fmt.Sprintf("%s-%s", args.TestName, "job"), string(data), ctx.Logger)

	taskID, err := testingservice.OpenAPICreateTestTask(ctx.UserName, args, ctx.Logger)
	ctx.Resp = testingservice.OpenAPICreateTestTaskResp{
		TaskID: taskID,
	}
	ctx.Err = err
}

func OpenAPIGetTestTaskResult(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	taskID, err := strconv.ParseInt(c.Param("taskID"), 10, 64)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid taskID")
		return
	}
	// projectName is display name in db and productName is project unique key in db
	productName := c.Query("projectName")
	testName := c.Query("testName")
	if taskID == 0 || productName == "" || testName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid params")
		return
	}

	ctx.Resp, ctx.Err = testingservice.OpenAPIGetTestTaskResult(taskID, productName, testName, ctx.Logger)
}
