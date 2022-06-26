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
	"encoding/json"
	"io/ioutil"

	"github.com/gin-gonic/gin"

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/workflow/service/workflow"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/log"
)

type listWorkflowTaskV4Query struct {
	PageSize     int64  `json:"page_size"    form:"page_size,default=20"`
	PageNum      int64  `json:"page_num"     form:"page_num,default=1"`
	WorkflowName string `json:"workflow_name" form:"workflow_name"`
}

type listWorkflowTaskV4Resp struct {
	WorkflowList []*commonmodels.WorkflowTask `json:"workflow_list"`
	Total        int64                        `json:"total"`
}

func CreateWorkflowTaskV4(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(commonmodels.WorkflowV4)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("CreateWorkflowTaskv4 c.GetRawData() err : %s", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("CreateWorkflowTaskv4 json.Unmarshal err : %s", err)
	}

	c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(data))

	if err := c.ShouldBindJSON(&args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	ctx.Err = workflow.CreateWorkflowTaskV4(ctx.UserName, args, ctx.Logger)
}

func ListWorkflowTaskV4(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	args := &listWorkflowTaskV4Query{}
	if err := c.ShouldBindQuery(args); err != nil {
		ctx.Err = err
		return
	}

	taskList, total, err := workflow.ListWorkflowTaskV4(args.WorkflowName, args.PageNum, args.PageSize, ctx.Logger)
	resp := listWorkflowTaskV4Resp{
		WorkflowList: taskList,
		Total:        total,
	}
	ctx.Resp = resp
	ctx.Err = err
}
