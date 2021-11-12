package handler

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/pkg/microservice/picket/core/public/service"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
)

func CreateWorkflowTask(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	body, _ := c.GetRawData()
	res, err := service.CreateWorkflowTask(c.Request.Header, c.Request.URL.Query(), body, ctx.Logger)
	if err != nil {
		ctx.Err = err
		return
	}
	// to avoid customer feel confused ，return workflow_name instead of pipline_name
	var resp *CreateWorkflowTaskResp
	err = json.Unmarshal(res, &resp)
	if err != nil {
		ctx.Err = err
		return
	}
	resp.WorkflowName = resp.PipelineName
	resp.PipelineName = ""
	ctx.Resp = resp
}

type CreateWorkflowTaskResp struct {
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
