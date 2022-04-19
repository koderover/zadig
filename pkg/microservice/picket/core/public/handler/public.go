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
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/picket/core/public/service"
	"github.com/koderover/zadig/pkg/setting"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
)

func CreateWorkflowTask(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	req := new(models.WorkflowTaskArgs)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.Err = err
		ctx.Logger.Errorf("ShouldBindJSON err:%s", err)
		return
	}
	req.Namespace = req.EnvName
	req.RequestMode = setting.RequestModeOpenAPI

	body, err := json.Marshal(req)
	if err != nil {
		ctx.Err = err
		ctx.Logger.Errorf("Marshal err:%s", err)
		return
	}
	res, err := service.CreateWorkflowTask(c.Request.Header, c.Request.URL.Query(), req.WorkflowName, body, ctx.Logger)
	if err != nil {
		ctx.Err = err
		ctx.Logger.Errorf("CreateWorkflowTask err:%s", err)
		return
	}
	// to avoid customer feel confused ，return workflow_name instead of pipline_name
	var resp *CreateWorkflowTaskResp
	err = json.Unmarshal(res, &resp)
	if err != nil {
		ctx.Err = err
		ctx.Logger.Errorf("Unmarshal err:%s", err)
		return
	}
	resp.WorkflowName = resp.PipelineName
	resp.PipelineName = ""
	ctx.Resp = resp
}

type CreateWorkflowTaskResp struct {
	ProjectName  string `json:"project_name"`
	PipelineName string `json:"pipeline_name,omitempty"`
	WorkflowName string `json:"workflow_name"`
	TaskID       int64  `json:"task_id"`
}

type EndpointResponse struct {
	ResultCode int    `json:"resultCode"`
	ErrorMsg   string `json:"errorMsg"`
}

func CancelWorkflowTask(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	id := c.Param("id")
	name := c.Param("name")
	statusCode, _ := service.CancelWorkflowTask(c.Request.Header, c.Request.URL.Query(), id, name, ctx.Logger)
	var code int
	var errorMsg string
	if statusCode == http.StatusOK {
		code = 0
		errorMsg = "success"
	} else if statusCode == http.StatusForbidden {
		code = statusCode
		errorMsg = "forbidden"
	} else {
		code = statusCode
		errorMsg = "fail"
	}
	ctx.Resp = EndpointResponse{
		ResultCode: code,
		ErrorMsg:   errorMsg,
	}
}

func RestartWorkflowTask(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	id := c.Param("id")
	name := c.Param("name")
	statusCode, _ := service.RestartWorkflowTask(c.Request.Header, c.Request.URL.Query(), id, name, ctx.Logger)
	var code int
	var errorMsg string
	if statusCode == http.StatusOK {
		code = 0
		errorMsg = "success"
	} else if statusCode == http.StatusForbidden {
		code = statusCode
		errorMsg = "forbidden"
	} else {
		code = statusCode
		errorMsg = "fail"
	}
	ctx.Resp = EndpointResponse{
		ResultCode: code,
		ErrorMsg:   errorMsg,
	}
}

func ListWorkflowTask(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	commitId := c.Query("commitId")
	ctx.Resp, ctx.Err = service.ListWorkflowTask(c.Request.Header, c.Request.URL.Query(), commitId, ctx.Logger)
}

func GetWorkflowDetail(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	id := c.Param("id")
	name := c.Param("name")
	c.Header("Content-Type", "application/json")
	ctx.Resp, ctx.Err = service.GetDetailedWorkflowTask(c.Request.Header, c.Request.URL.Query(), id, name, ctx.Logger)
}

func ListDelivery(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	productName := c.Query("projectName")
	workflowName := c.Query("workflowName")
	taskIDStr := c.Query("taskId")
	perPageStr := c.Query("perPage")
	pageStr := c.Query("page")
	ctx.Resp, ctx.Err = service.ListDelivery(c.Request.Header, c.Request.URL.Query(), productName, workflowName, taskIDStr, perPageStr, pageStr, ctx.Logger)
}

func GetArtifactInfo(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	image := c.Query("image")
	ctx.Resp, ctx.Err = service.GetArtifactInfo(c.Request.Header, c.Request.URL.Query(), image, ctx.Logger)
}
