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
	"bytes"
	"io/ioutil"

	"github.com/gin-gonic/gin"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/workflow/service/workflow"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/log"
	"gopkg.in/yaml.v3"
)

type listWorkflowV4Query struct {
	PageSize int64  `json:"page_size"    form:"page_size,default=20"`
	PageNum  int64  `json:"page_num"     form:"page_num,default=1"`
	Project  string `json:"project"      form:"project"`
}

type listWorkflowV4Resp struct {
	WorkflowList []*workflow.Workflow `json:"workflow_list"`
	Total        int64                `json:"total"`
}

func CreateWorkflowV4(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(commonmodels.WorkflowV4)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("CreateWorkflowv4 c.GetRawData() err : %s", err)
	}
	if err = yaml.Unmarshal(data, args); err != nil {
		log.Errorf("CreateWorkflowv4 json.Unmarshal err : %s", err)
	}

	c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(data))

	if err := c.ShouldBindYAML(&args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	ctx.Err = workflow.CreateWorkflowV4(ctx.UserName, args, ctx.Logger)
}

func LintWorkflowV4(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(commonmodels.WorkflowV4)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("CreateWorkflowv4 c.GetRawData() err : %s", err)
	}
	if err = yaml.Unmarshal(data, args); err != nil {
		log.Errorf("CreateWorkflowv4 json.Unmarshal err : %s", err)
	}

	c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(data))

	if err := c.ShouldBindYAML(&args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	ctx.Err = workflow.LintWorkflowV4(args, ctx.Logger)
}

func ListWorkflowV4(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	args := &listWorkflowV4Query{}
	if err := c.ShouldBindQuery(args); err != nil {
		ctx.Err = err
		return
	}

	workflowList, err := workflow.ListWorkflowV4(args.Project, ctx.UserName, ctx.Logger)
	resp := listWorkflowV4Resp{
		WorkflowList: workflowList,
		Total:        0,
	}
	ctx.Resp = resp
	ctx.Err = err
}

func UpdateWorkflowV4(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(commonmodels.WorkflowV4)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("UpdateWorkflowV4 c.GetRawData() err : %s", err)
	}
	if err = yaml.Unmarshal(data, args); err != nil {
		log.Errorf("UpdateWorkflowV4 json.Unmarshal err : %s", err)
	}

	c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(data))

	if err := c.ShouldBindYAML(&args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	ctx.Err = workflow.UpdateWorkflowV4(c.Param("name"), ctx.UserName, args, ctx.Logger)
}

func DeleteWorkflowV4(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Err = workflow.DeleteWorkflowV4(c.Param("name"), ctx.Logger)
}

func FindWorkflowV4(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	resp, err := workflow.FindWorkflowV4(c.Param("name"), ctx.Logger)
	if err != nil {
		c.JSON(e.ErrorMessage(err))
		c.Abort()
		return
	}
	c.YAML(200, resp)
}

func GetWorkflowV4Preset(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = workflow.GetWorkflowv4Preset(c.Param("name"), ctx.Logger)
}
