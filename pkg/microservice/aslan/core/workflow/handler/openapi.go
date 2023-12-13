package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"

	"github.com/gin-gonic/gin"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	workflowservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/workflow/service/workflow"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/log"
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

func OpenAPICreateWorkflowView(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(workflowservice.OpenAPICreateWorkflowViewReq)
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

	projectKey := c.Query("projectKey")
	if projectKey == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("projectKey is required")
		return
	}

	ctx.Resp, ctx.Err = workflowservice.OpenAPIGetWorkflowViews(projectKey, ctx.Logger)
}

func OpenAPIUpdateWorkflowView(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	viewName := c.Param("name")
	if viewName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("view name is required")
		return
	}
	args := new(workflowservice.OpenAPICreateWorkflowViewReq)
	err := c.BindJSON(args)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
	}
	args.Name = viewName
	args.ProjectName = c.Query("projectKey")
	isValid, err := args.Validate()
	if !isValid {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, args.ProjectName, "(OpenAPI)"+"更新", "工作流视图", viewName, "", ctx.Logger)

	list := make([]*commonmodels.WorkflowViewDetail, 0)
	for _, workflowInfo := range args.WorkflowList {
		list = append(list, &commonmodels.WorkflowViewDetail{
			WorkflowName:        workflowInfo.WorkflowName,
			WorkflowDisplayName: workflowInfo.WorkflowDisplayName,
			WorkflowType:        workflowInfo.WorkflowType,
			Enabled:             workflowInfo.Enabled,
		})
	}

	ctx.Err = workflowservice.UpdateWorkflowViewOpenAPI(viewName, args.ProjectName, list, ctx.UserName, ctx.Logger)
}

func OpenAPIDeleteWorkflowView(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	viewName := c.Param("name")
	if viewName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("view name is required")
		return
	}

	projectKey := c.Query("projectKey")
	if projectKey == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("projectKey is required")
		return
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, projectKey, "(OpenAPI)"+"删除", "工作流视图", viewName, "", ctx.Logger)

	ctx.Err = workflowservice.DeleteWorkflowView(projectKey, viewName, ctx.Logger)
}

type getworkflowTaskReq struct {
	TaskID       int64  `json:"task_id"       form:"taskId"`
	WorkflowName string `json:"workflow_key" form:"workflowKey"`
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

	workflowKey := c.Query("workflowKey")
	projectKey := c.Query("projectKey")
	if workflowKey == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("workflowKey is required")
		return
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, projectKey, "(OpenAPI)"+"删除", "自定义工作流", workflowKey, "", ctx.Logger)

	ctx.Err = workflowservice.OpenAPIDeleteCustomWorkflowV4(workflowKey, projectKey, ctx.Logger)
}

func OpenAPIGetCustomWorkflowV4(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	workflowName := c.Param("name")
	projectName := c.Query("projectKey")
	if workflowName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("workflowKey is required")
		return
	}
	if projectName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("projectKey is required")
		return
	}

	ctx.Resp, ctx.Err = workflowservice.OpenAPIGetCustomWorkflowV4(workflowName, projectName, ctx.Logger)
}

func OpenAPIDeleteProductWorkflowV4(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	workflowKey := c.Query("workflowKey")
	projectKey := c.Query("projectKey")
	if workflowKey == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("workflowKey is required")
		return
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, projectKey, "(OpenAPI)"+"删除", "产品工作流", workflowKey, "", ctx.Logger)

	ctx.Err = workflowservice.OpenAPIDeleteProductWorkflowV4(workflowKey, ctx.RequestID, ctx.RequestID, ctx.Logger)
}

func OpenAPIGetProductWorkflowTasksV4(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	projectKey := c.Query("projectKey")
	if projectKey == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("projectKey is required")
		return
	}
	workflowKey := c.Param("name")
	if workflowKey == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("workflowKey is required")
		return
	}

	args := new(workflowservice.OpenAPIPageParamsFromReq)
	err := c.BindQuery(args)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	ctx.Resp, ctx.Err = workflowservice.OpenAPIGetProductWorkflowTasksV4(projectKey, workflowKey, args.PageNum, args.PageSize, ctx.Logger)
}

func OpenAPIGetProductWorkflowTaskV4(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	projectKey := c.Query("projectKey")
	if projectKey == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("projectKey is required")
		return
	}
	workflowName := c.Param("name")
	if workflowName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("workflowKey is required")
		return
	}
	taskID, err := strconv.ParseInt(c.Param("taskID"), 10, 64)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("taskID is required")
		return
	}

	ctx.Resp, ctx.Err = workflowservice.OpenAPIGetProductWorkflowTaskV4(projectKey, workflowName, taskID, ctx.Logger)
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
	if args.ProjectKey == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("projectKey is required")
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
	internalhandler.InsertOperationLog(c, ctx.UserName, c.Query("projectKey"), "OpenAPI"+"重试", "自定义工作流任务", name, fmt.Sprintf("%d", taskID), ctx.Logger)

	ctx.Err = workflowservice.OpenAPIRetryCustomWorkflowTaskV4(name, c.Query("projectKey"), taskID, ctx.Logger)
}

func OpenAPIGetCustomWorkflowTaskV4(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	workflowKey := c.Param("name")
	if workflowKey == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("workflowKey is required")
		return
	}

	args := new(workflowservice.OpenAPIPageParamsFromReq)
	err := c.BindQuery(args)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	ctx.Resp, ctx.Err = workflowservice.OpenAPIGetCustomWorkflowTaskV4(workflowKey, args.ProjectKey, args.PageNum, args.PageSize, ctx.Logger)
}

func OpenAPIApproveStage(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	args := &workflowservice.OpenAPIApproveRequest{}

	data, err := c.GetRawData()
	if err != nil {
		ctx.Err = fmt.Errorf("failed to get raw data from request, error: %s", err)
		return
	}
	if err = json.Unmarshal(data, args); err != nil {
		ctx.Err = fmt.Errorf("failed to unmarshal request data, error: %s", err)
		return
	}

	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	if err := c.ShouldBindJSON(&args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	ctx.Err = workflowservice.ApproveStage(args.WorkflowName, args.StageName, ctx.UserName, ctx.UserID, args.Comment, args.TaskID, args.Approve, ctx.Logger)
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
