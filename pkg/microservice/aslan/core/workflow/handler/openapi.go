package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"

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

	for _, workflow := range req.WorkflowList {
		if workflow.WorkflowType != "product" && workflow.WorkflowType != "custom" {
			return false, fmt.Errorf("workflow type must be custom or product")
		}
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

func OpenAPIGetWorkflowViews(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	projectName := c.Query("projectName")
	if projectName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("project name is required")
		return
	}

	ctx.Resp, ctx.Err = workflowservice.OpenAPIGetWorkflowViews(projectName, ctx.Logger)
}

func OpenAPIUpdateWorkflowView(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	viewName := c.Param("name")
	if viewName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("view name is required")
		return
	}
	args := new(openAPICreateWorkflowViewReq)
	err := c.BindJSON(args)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
	}
	args.Name = viewName
	args.ProjectName = c.Query("projectName")
	isValid, err := args.Validate()
	if !isValid {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, args.ProjectName, "(OpenAPI)"+"更新", "工作流视图", viewName, "", ctx.Logger)

	ctx.Err = workflowservice.UpdateWorkflowViewOpenAPI(viewName, args.ProjectName, args.WorkflowList, ctx.UserName, ctx.Logger)
}

func OpenAPIDeleteWorkflowView(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	viewName := c.Param("name")
	if viewName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("view name is required")
		return
	}

	projectName := c.Query("projectName")
	if projectName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("project name is required")
		return
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, projectName, "(OpenAPI)"+"删除", "工作流视图", viewName, "", ctx.Logger)

	ctx.Err = workflowservice.DeleteWorkflowView(projectName, viewName, ctx.Logger)
}

type getworkflowTaskReq struct {
	TaskID       int64  `json:"task_id"       form:"task_id"`
	WorkflowName string `json:"workflow_name" form:"workflow_name"`
}

func OpenAPIGetWorkflowTaskV4(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(getworkflowTaskReq)
	err := c.BindQuery(args)
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

func OpenAPIDeleteCustomWorkflowV4(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	workflowName := c.Query("workflowName")
	projectName := c.Query("projectName")
	if workflowName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("workflow name is required")
		return
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, projectName, "(OpenAPI)"+"删除", "自定义工作流", workflowName, "", ctx.Logger)

	ctx.Err = workflowservice.OpenAPIDeleteCustomWorkflowV4(workflowName, projectName, ctx.Logger)
}

func OpenAPIGetCustomWorkflowV4(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	workflowName := c.Param("name")
	projectName := c.Query("projectName")
	if workflowName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("workflow name is required")
		return
	}
	if projectName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("project name is required")
		return
	}

	ctx.Resp, ctx.Err = workflowservice.OpenAPIGetCustomWorkflowV4(workflowName, projectName, ctx.Logger)
}

func OpenAPIDeleteProductWorkflowV4(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	workflowName := c.Query("workflowName")
	projectName := c.Query("projectName")
	if workflowName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("workflow name is required")
		return
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, projectName, "(OpenAPI)"+"删除", "产品工作流", workflowName, "", ctx.Logger)

	ctx.Err = workflowservice.OpenAPIDeleteProductWorkflowV4(workflowName, ctx.RequestID, ctx.RequestID, ctx.Logger)
}

func OpenAPIGetProductWorkflowTasksV4(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	projectName := c.Query("projectName")
	if projectName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("project name is required")
		return
	}
	workflowName := c.Param("name")
	if workflowName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("workflow name is required")
		return
	}

	args := new(workflowservice.OpenAPIPageParamsFromReq)
	err := c.BindQuery(args)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	ctx.Resp, ctx.Err = workflowservice.OpenAPIGetProductWorkflowTasksV4(projectName, workflowName, args.PageNum, args.PageSize, ctx.Logger)
}

func OpenAPIGetProductWorkflowTaskV4(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	projectName := c.Query("projectName")
	if projectName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("project name is required")
		return
	}
	workflowName := c.Param("name")
	if workflowName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("workflow name is required")
		return
	}
	taskID, err := strconv.ParseInt(c.Param("taskID"), 10, 64)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("taskID is required")
		return
	}

	ctx.Resp, ctx.Err = workflowservice.OpenAPIGetProductWorkflowTaskV4(projectName, workflowName, taskID, ctx.Logger)
}

func OpenAPIGetWorkflowV4List(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(workflowservice.OpenAPIWorkflowV4ListReq)
	err := c.ShouldBindQuery(args)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	if args.ProjectName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("project name is required")
		return
	}

	ctx.Resp, ctx.Err = workflowservice.OpenAPIGetCustomWorkflowV4List(args, ctx.Logger)
}

func OpenAPIRetryCustomWorkflowTaskV4(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	name, taskID, err := generalRequestValidate(c)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, c.Query("projectName"), "OpenAPI"+"重试", "自定义工作流任务", name, fmt.Sprintf("%d", taskID), ctx.Logger)

	ctx.Err = workflowservice.OpenAPIRetryCustomWorkflowTaskV4(name, c.Query("projectName"), taskID, ctx.Logger)
}

func OpenAPIGetCustomWorkflowTaskV4(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	workflowName := c.Param("name")
	if workflowName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("workflow name is required")
		return
	}

	args := new(workflowservice.OpenAPIPageParamsFromReq)
	err := c.BindQuery(args)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	ctx.Resp, ctx.Err = workflowservice.OpenAPIGetCustomWorkflowTaskV4(workflowName, c.Query("projectName"), args.PageNum, args.PageSize, ctx.Logger)
}

func generalRequestValidate(c *gin.Context) (string, int64, error) {
	name := c.Param("name")
	if name == "" {
		return "", 0, errors.New("workflowName can't be empty")
	}

	taskIDstr := c.Param("taskID")
	if taskIDstr == "" {
		return "", 0, errors.New("takstaskIDID can't be empty")
	}
	taskID, err := strconv.ParseInt(taskIDstr, 10, 64)
	if err != nil {
		return "", 0, errors.New("taksID is invalid")
	}
	return name, taskID, nil
}
