package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/koderover/zadig/v2/pkg/types"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	workflowservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/workflow/service/workflow"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

func CreateCustomWorkflowTask(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := new(workflowservice.OpenAPICreateCustomWorkflowTaskArgs)
	data := getBody(c)
	if err := json.Unmarshal([]byte(data), args); err != nil {
		log.Errorf("CreateWorkflowTaskv4 json.Unmarshal err : %s", err)
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	internalhandler.InsertOperationLog(c, ctx.UserName, args.ProjectName, "新建", "工作流任务", args.WorkflowName, args.WorkflowName, data, types.RequestBodyTypeJSON, ctx.Logger)

	// authorization check
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[args.ProjectName]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if !ctx.Resources.ProjectAuthInfo[args.ProjectName].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[args.ProjectName].Workflow.Execute {
			// check if the permission is given by collaboration mode
			permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, args.ProjectName, types.ResourceTypeWorkflow, args.WorkflowName, types.WorkflowActionRun)
			if err != nil || !permitted {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	ctx.Resp, ctx.RespErr = workflowservice.CreateCustomWorkflowTask(ctx.UserName, args, ctx.Logger)
}

func OpenAPICreateWorkflowView(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := new(workflowservice.OpenAPICreateWorkflowViewReq)
	err = c.BindJSON(args)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
	}

	isValid, err := args.Validate()
	if !isValid {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	// authorization check
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[args.ProjectName]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if !ctx.Resources.ProjectAuthInfo[args.ProjectName].IsProjectAdmin {
			// check if the permission is given by collaboration mode
			ctx.UnAuthorized = true
			return
		}
	}

	ctx.RespErr = workflowservice.CreateWorkflowViewOpenAPI(args.Name, args.ProjectName, args.WorkflowList, ctx.UserName, ctx.Logger)
}

func OpenAPIGetWorkflowViews(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	projectKey := c.Query("projectKey")
	if projectKey == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("projectKey is required")
		return
	}

	ctx.Resp, ctx.RespErr = workflowservice.OpenAPIGetWorkflowViews(projectKey, ctx.Logger)
}

func OpenAPIUpdateWorkflowView(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	viewName := c.Param("name")
	if viewName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("view name is required")
		return
	}
	args := new(workflowservice.OpenAPICreateWorkflowViewReq)
	err = c.BindJSON(args)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
	}
	args.Name = viewName
	args.ProjectName = c.Query("projectKey")
	isValid, err := args.Validate()
	if !isValid {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, args.ProjectName, "(OpenAPI)"+"更新", "工作流视图", viewName, viewName, "", types.RequestBodyTypeJSON, ctx.Logger)

	// authorization check
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[args.ProjectName]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if !ctx.Resources.ProjectAuthInfo[args.ProjectName].IsProjectAdmin {
			// check if the permission is given by collaboration mode
			ctx.UnAuthorized = true
			return
		}
	}

	list := make([]*commonmodels.WorkflowViewDetail, 0)
	for _, workflowInfo := range args.WorkflowList {
		list = append(list, &commonmodels.WorkflowViewDetail{
			WorkflowName:        workflowInfo.WorkflowName,
			WorkflowDisplayName: workflowInfo.WorkflowDisplayName,
			WorkflowType:        workflowInfo.WorkflowType,
			Enabled:             workflowInfo.Enabled,
		})
	}

	ctx.RespErr = workflowservice.UpdateWorkflowViewOpenAPI(viewName, args.ProjectName, list, ctx.UserName, ctx.Logger)
}

func OpenAPIDeleteWorkflowView(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	viewName := c.Param("name")
	if viewName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("view name is required")
		return
	}

	projectKey := c.Query("projectKey")
	if projectKey == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("projectKey is required")
		return
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, projectKey, "(OpenAPI)"+"删除", "工作流视图", viewName, viewName, "", types.RequestBodyTypeJSON, ctx.Logger)

	// authorization check
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin {
			// check if the permission is given by collaboration mode
			ctx.UnAuthorized = true
			return
		}
	}

	ctx.RespErr = workflowservice.DeleteWorkflowView(projectKey, viewName, ctx.Logger)
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
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	ctx.Resp, ctx.RespErr = workflowservice.GetWorkflowTaskV4(args.WorkflowName, args.TaskID, ctx.Logger)
}

func OpenAPICancelWorkflowTaskV4(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(getworkflowTaskReq)
	err := c.BindQuery(args)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	ctx.RespErr = workflowservice.CancelWorkflowTaskV4(ctx.UserName, args.WorkflowName, args.TaskID, ctx.Logger)
}

func OpenAPIDeleteCustomWorkflowV4(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	workflowKey := c.Query("workflowKey")
	projectKey := c.Query("projectKey")
	if workflowKey == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("workflowKey is required")
		return
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, projectKey, "(OpenAPI)"+"删除", "工作流", workflowKey, workflowKey, "", types.RequestBodyTypeJSON, ctx.Logger)

	// authorization check
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectKey].Workflow.Edit {
			ctx.UnAuthorized = true
			return
		}
	}

	ctx.RespErr = workflowservice.OpenAPIDeleteCustomWorkflowV4(workflowKey, projectKey, ctx.Logger)
}

func OpenAPIGetCustomWorkflowV4(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	workflowName := c.Param("name")
	projectName := c.Query("projectKey")
	if workflowName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("workflowKey is required")
		return
	}
	if projectName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("projectKey is required")
		return
	}

	ctx.Resp, ctx.RespErr = workflowservice.OpenAPIGetCustomWorkflowV4(workflowName, projectName, ctx.Logger)
}

func OpenAPIGetWorkflowV4List(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(workflowservice.OpenAPIWorkflowV4ListReq)
	err := c.ShouldBindQuery(args)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	if args.ProjectKey == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("projectKey is required")
		return
	}

	ctx.Resp, ctx.RespErr = workflowservice.OpenAPIGetCustomWorkflowV4List(args, ctx.Logger)
}

func OpenAPIRetryCustomWorkflowTaskV4(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	name, taskID, err := generalRequestValidate(c)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, c.Query("projectKey"), "OpenAPI"+"重试", "工作流任务", name, name, fmt.Sprintf("%d", taskID), types.RequestBodyTypeJSON, ctx.Logger)

	ctx.RespErr = workflowservice.OpenAPIRetryCustomWorkflowTaskV4(name, c.Query("projectKey"), taskID, ctx.Logger)
}

func OpenAPIUpdateWorkflowV4TaskRemark(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	workflowName := c.Param("name")

	w, err := workflowservice.FindWorkflowV4Raw(workflowName, ctx.Logger)
	if err != nil {
		ctx.Logger.Errorf("EnableDebugWorkflowTaskV4 error: %v", err)
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}

	args := new(updateWorkflowV4TaskRemarkReq)
	data := getBody(c)
	if err := json.Unmarshal([]byte(data), args); err != nil {
		log.Errorf("CreateWorkflowTaskv4 json.Unmarshal err : %s", err)
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	internalhandler.InsertOperationLog(c, ctx.UserName, w.Project, "OpenAPI"+"更新", "工作流任务", workflowName, workflowName, data, types.RequestBodyTypeJSON, ctx.Logger)

	// authorization check
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[w.Project]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if !ctx.Resources.ProjectAuthInfo[w.Project].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[w.Project].Workflow.Execute {
			// check if the permission is given by collaboration mode
			permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, w.Project, types.ResourceTypeWorkflow, workflowName, types.WorkflowActionRun)
			if err != nil || !permitted {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	taskID, err := strconv.ParseInt(c.Param("taskID"), 10, 64)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid task id")
		return
	}

	ctx.RespErr = workflowservice.UpdateWorkflowV4TaskRemark(workflowName, taskID, args.Remark, ctx.Logger)
}

func OpenAPIGetCustomWorkflowTaskV4(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	workflowKey := c.Param("name")
	if workflowKey == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("workflowKey is required")
		return
	}

	args := new(workflowservice.OpenAPIPageParamsFromReq)
	err := c.BindQuery(args)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	ctx.Resp, ctx.RespErr = workflowservice.OpenAPIGetCustomWorkflowTaskV4(workflowKey, args.ProjectKey, args.PageNum, args.PageSize, ctx.Logger)
}

func OpenAPIApproveStage(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	args := &workflowservice.OpenAPIApproveRequest{}

	data, err := c.GetRawData()
	if err != nil {
		ctx.RespErr = fmt.Errorf("failed to get raw data from request, error: %s", err)
		return
	}
	if err = json.Unmarshal(data, args); err != nil {
		ctx.RespErr = fmt.Errorf("failed to unmarshal request data, error: %s", err)
		return
	}

	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	if err := c.ShouldBindJSON(&args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	ctx.RespErr = workflowservice.ApproveStage(args.WorkflowName, args.StageName, ctx.UserName, ctx.UserID, args.Comment, args.TaskID, args.Approve, ctx.Logger)
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
