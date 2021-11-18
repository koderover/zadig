package handler

import (
	"bytes"
	"encoding/json"
	"io/ioutil"

	"github.com/gin-gonic/gin"

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	workflowservice "github.com/koderover/zadig/pkg/microservice/aslan/core/workflow/service/workflow"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	"github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/log"
)

func CreateWorkflowV3(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	req := &commonmodels.WorkflowV3{}
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("CreateWorkflowV3 err: %s", err)
	}
	if err = json.Unmarshal(data, req); err != nil {
		log.Errorf("CreateWorkflow unmarshal json err: %s", err)
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, req.ProjectName, "新增", "工作流V3", req.Name, string(data), ctx.Logger)
	c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(data))

	if err := c.ShouldBindJSON(&req); err != nil {
		ctx.Err = errors.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	ctx.Err = workflowservice.CreateWorkflowV3(ctx.UserName, req, ctx.Logger)
}

type listWorkflowV3Query struct {
	PageSize    int64  `json:"page_size"    form:"page_size,default=100"`
	PageNum     int64  `json:"page_num"     form:"page_num,default=1"`
	ProjectName string `json:"project_name" form:"project_name"`
}

type listWorkflowV3Resp struct {
	WorkflowList []*workflowservice.WorkflowV3Brief `json:"workflow_list"`
	Total        int64                              `json:"total"`
}

func ListWorkflowsV3(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	// Query Verification
	args := &listWorkflowV3Query{}
	if err := c.ShouldBindQuery(args); err != nil {
		ctx.Err = err
		return
	}

	workflowList, total, err := workflowservice.ListWorkflowsV3(args.ProjectName, args.PageNum, args.PageSize, ctx.Logger)
	resp := listWorkflowV3Resp{
		WorkflowList: workflowList,
		Total:        total,
	}
	ctx.Resp = resp
	ctx.Err = err
}

func GetWorkflowV3Detail(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = workflowservice.GetWorkflowV3Detail(c.Param("id"), ctx.Logger)
}

func UpdateWorkflowV3(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	req := &workflowservice.WorkflowV3{}

	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("UpdateWorkflowV3 err: %s", err)
	}
	if err = json.Unmarshal(data, req); err != nil {
		log.Errorf("UpdateWorkflowV3 unmarshal json err: %s", err)
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, req.ProjectName, "修改", "工作流V3", req.Name, string(data), ctx.Logger)

	if err := c.ShouldBindJSON(&req); err != nil {
		ctx.Err = errors.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	ctx.Err = workflowservice.UpdateWorkflowV3(c.Param("id"), ctx.UserName, req, ctx.Logger)
}

func DeleteWorkflowV3(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Err = workflowservice.DeleteWorkflowV3(c.Param("id"), ctx.Logger)
}
