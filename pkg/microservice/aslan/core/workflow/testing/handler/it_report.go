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

	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/workflow/testing/service"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

func GetLocalTestSuite(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	taskID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid task id")
		return
	}
	testType := c.Query("testType")
	if testType == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid testType")
		return
	}

	ctx.Resp, ctx.Err = commonservice.GetLocalTestSuite(c.Param("pipelineName"), "", testType, taskID, c.Param("testName"), config.SingleType, ctx.Logger)
}

func GetWorkflowV4LocalTestSuite(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	taskID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid task id")
		return
	}

	ctx.Resp, ctx.Err = commonservice.GetWorkflowV4LocalTestSuite(c.Param("workflowName"), c.Param("jobName"), taskID, ctx.Logger)
}

func GetWorkflowLocalTestSuite(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	taskID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid task id")
		return
	}
	testType := c.Query("testType")
	if testType == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid testType")
		return
	}

	workflowTypeString := config.WorkflowType
	workflowType := c.Query("workflowType")
	if workflowType == string(config.TestType) {
		workflowTypeString = config.TestType
	}
	ctx.Resp, ctx.Err = commonservice.GetLocalTestSuite(c.Param("pipelineName"), c.Param("serviceName"), testType, taskID, c.Param("testName"), workflowTypeString, ctx.Logger)
}

func GetTestLocalTestSuite(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = service.GetTestLocalTestSuite(c.Param("serviceName"), ctx.Logger)
}
