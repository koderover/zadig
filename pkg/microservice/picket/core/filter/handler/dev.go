package handler

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/pkg/microservice/picket/core/filter/service"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
)

func CreateWorkflowTask(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	body, _ := ioutil.ReadAll(c.Request.Body)
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
