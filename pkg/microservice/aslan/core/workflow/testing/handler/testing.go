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
	"io"
	"strconv"

	"github.com/gin-gonic/gin"

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/workflow/testing/service"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/koderover/zadig/pkg/util/ginzap"
)

func GetTestProductName(c *gin.Context) {
	args := new(commonmodels.Testing)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("c.GetRawData() err : %v", err)
		return
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("json.Unmarshal err : %v", err)
		return
	}
	c.Set("productName", args.ProductName)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))
	c.Next()
}

func CreateTestModule(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(commonmodels.Testing)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("CreateTestModule c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("CreateTestModule json.Unmarshal err : %v", err)
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, args.ProductName, "新增", "项目管理-测试", args.Name, string(data), ctx.Logger)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	err = c.BindJSON(args)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid Test args")
		return
	}

	ctx.Err = service.CreateTesting(ctx.UserName, args, ctx.Logger)
}

func UpdateTestModule(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(commonmodels.Testing)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("UpdateTestModule c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("UpdateTestModule json.Unmarshal err : %v", err)
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, args.ProductName, "更新", "项目管理-测试", args.Name, string(data), ctx.Logger)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	err = c.BindJSON(args)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid Test args")
		return
	}

	ctx.Err = service.UpdateTesting(ctx.UserName, args, ctx.Logger)
}

func ListTestModules(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	projects := c.QueryArray("projects")
	projectName := c.Query("projectName")
	if len(projects) == 0 && len(projectName) > 0 {
		projects = []string{projectName}
	}
	ctx.Resp, ctx.Err = service.ListTestingOpt(projects, c.Query("testType"), ctx.Logger)
}

func GetTestModule(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	name := c.Param("name")

	if name == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("empty Name")
		return
	}
	ctx.Resp, ctx.Err = service.GetTesting(name, c.Query("projectName"), ctx.Logger)
}

func DeleteTestModule(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	internalhandler.InsertOperationLog(c, ctx.UserName, c.Query("projectName"), "删除", "项目管理-测试", c.Param("name"), "", ctx.Logger)

	name := c.Param("name")
	if name == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("empty Name")
		return
	}

	ctx.Err = commonservice.DeleteTestModule(name, c.Query("projectName"), ctx.RequestID, ctx.Logger)
}

func GetHTMLTestReport(c *gin.Context) {
	content, err := service.GetHTMLTestReport(
		c.Query("pipelineName"),
		c.Query("pipelineType"),
		c.Query("taskID"),
		c.Query("testName"),
		ginzap.WithContext(c).Sugar(),
	)
	if err != nil {
		c.JSON(500, gin.H{"err": err})
		return
	}

	c.Header("content-type", "text/html")
	c.String(200, content)
}

func GetWorkflowV4HTMLTestReport(c *gin.Context) {
	taskID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(500, gin.H{"err": err})
		return
	}
	content, err := service.GetWorkflowV4HTMLTestReport(c.Param("workflowName"), c.Param("jobName"), taskID, ginzap.WithContext(c).Sugar())
	if err != nil {
		c.JSON(500, gin.H{"err": err})
		return
	}

	c.Header("content-type", "text/html")
	c.String(200, content)
}
