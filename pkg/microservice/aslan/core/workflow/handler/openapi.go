package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/gin-gonic/gin"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"

	workflowservice "github.com/koderover/zadig/pkg/microservice/aslan/core/workflow/service/workflow"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/log"
)

func CreateCustomWorkflowTask(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(workflowservice.OpenAPICreateCustomWorkflowTaskArgs)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("CreateWorkflowTaskv4 c.GetRawData() err : %s", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("CreateWorkflowTaskv4 json.Unmarshal err : %s", err)
	}

	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	if err := c.ShouldBindJSON(&args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	ctx.Resp, ctx.Err = workflowservice.CreateCustomWorkflowTask(ctx.UserName, args, ctx.Logger)
}

type openAPICreateWorkflowViewReq struct {
	ProjectName  string                             `json:"project_name"`
	Name         string                             `json:"name"`
	WorkflowList []*commonmodels.WorkflowViewDetail `json:"workflow_list"`
}

func (req *openAPICreateWorkflowViewReq) Validate() (bool, error) {
	if req.ProjectName == "" {
		return false, fmt.Errorf("project name cannot be empty")
	}
	if req.Name == "" {
		return false, fmt.Errorf("view name cannot be empty")
	}
	return true, nil
}

func OpenAPICreateWorkflowView(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(openAPICreateWorkflowViewReq)
	err := c.BindJSON(args)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
	}

	isValid, err := args.Validate()
	if !isValid {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	ctx.Err = workflowservice.CreateWorkflowViewOpenAPI(args.Name, args.ProjectName, args.WorkflowList, ctx.UserName, ctx.Logger)
}

type getworkflowTaskReq struct {
	TaskID       int64  `json:"task_id"`
	WorkflowName string `json:"workflow_name"`
}

func OpenAPIGetWorkflowTaskV4(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(getworkflowTaskReq)
	err := c.BindJSON(args)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	ctx.Resp, ctx.Err = workflowservice.GetWorkflowTaskV4(args.WorkflowName, args.TaskID, ctx.Logger)
}

func OpenAPICancelWorkflowTaskV4(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(getworkflowTaskReq)
	err := c.BindJSON(args)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	ctx.Err = workflowservice.CancelWorkflowTaskV4(ctx.UserName, args.WorkflowName, args.TaskID, ctx.Logger)
}

func OpenAPICreateProductWorkflowTask(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(workflowservice.OpenAPICreateProductWorkflowTaskArgs)
	err := c.BindJSON(args)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	isValid, err := args.Validate()
	if !isValid {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	ctx.Resp, ctx.Err = workflowservice.OpenAPICreateProductWorkflowTask(ctx.UserName, args, ctx.Logger)
}
