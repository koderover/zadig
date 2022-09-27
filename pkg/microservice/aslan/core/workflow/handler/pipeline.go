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

	"github.com/gin-gonic/gin"

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/workflow/service/workflow"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/log"
)

func GetPipelineProductName(c *gin.Context) {
	args := new(commonmodels.Pipeline)
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

func GetProductNameByPipeline(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	pipelineName := c.Param("old")
	if pipelineName == "" {
		pipelineName = c.Param("name")
	}
	pipeline, err := workflow.GetPipeline(ctx.UserID, pipelineName, ctx.Logger)
	if err != nil {
		log.Errorf("GetProductNameByPipeline err : %v", err)
		return
	}
	c.Set("productName", pipeline.ProductName)
	c.Next()
}

// ListPipelines
// @Router /workflow/v2/pipelines [GET]
// @Summary Return all workflows (also called pipelines)
// @Produce json
// @Success 200 {object} interface{} "response type follows list of microservice/aslan/core/common/repository/models#Pipeline"
func ListPipelines(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = workflow.ListPipelines(ctx.Logger)
}

// GetPipeline
// @Router /workflow/v2/pipelines/{name} [GET]
// @Summary Get the relevant workflow (also called pipeline) information with the specified workflow name
// @Param name path string true "Name of the workflow"
// @Produce json
// @Success 200 {object} interface{} "response type follows microservice/aslan/core/common/repository/models#Pipeline"
func GetPipeline(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = workflow.GetPipeline(ctx.UserID, c.Param("name"), ctx.Logger)
}

// UpsertPipeline create a new pipeline
func UpsertPipeline(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(commonmodels.Pipeline)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("UpsertPipeline c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("UpsertPipeline json.Unmarshal err : %v", err)
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, args.ProductName, "新增", "单服务-工作流", args.Name, string(data), ctx.Logger)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	if err := c.BindJSON(args); err != nil || len(args.Name) == 0 {
		log.Error(err)
		ctx.Err = e.ErrInvalidParam.AddDesc(fmt.Sprintf("invalid pipeline json args: %v", err))
		return
	}
	args.UpdateBy = ctx.UserName
	ctx.Err = workflow.UpsertPipeline(args, ctx.Logger)
}

// CopyPipeline duplicate pipeline
func CopyPipeline(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	internalhandler.InsertOperationLog(c, ctx.UserName, c.GetString("productName"), "复制", "单服务-工作流", fmt.Sprintf("old:%s,new:%s", c.Param("old"), c.Param("new")), "", ctx.Logger)
	ctx.Err = workflow.CopyPipeline(c.Param("old"), c.Param("new"), ctx.UserName, ctx.Logger)
}

// RenamePipeline rename pipeline
func RenamePipeline(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	internalhandler.InsertOperationLog(c, ctx.UserName, c.GetString("productName"), "修改", "单服务-工作流", fmt.Sprintf("old:%s,new:%s", c.Param("old"), c.Param("new")), "", ctx.Logger)
	ctx.Err = workflow.RenamePipeline(c.Param("old"), c.Param("new"), ctx.Logger)
}

// DeletePipeline delete pipeline
func DeletePipeline(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	internalhandler.InsertOperationLog(c, ctx.UserName, c.GetString("productName"), "删除", "单服务-工作流", c.Param("name"), "", ctx.Logger)
	ctx.Err = commonservice.DeletePipeline(c.Param("name"), ctx.RequestID, false, ctx.Logger)
}
