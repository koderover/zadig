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
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"k8s.io/apimachinery/pkg/util/wait"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	workflowservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/workflow/service/workflow"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/workflow/testing/service"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/types"
)

var MissingIDError = e.ErrInvalidParam.AddDesc("ID must be provided")

func GetScanningProductName(c *gin.Context) {
	args := new(commonmodels.Scanning)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("c.GetRawData() err : %s", err)
		return
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("json.Unmarshal err : %s", err)
		return
	}
	c.Set("productName", args.ProjectName)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))
	c.Next()
}

func FindScanningProjectNameFromID(c *gin.Context) {
	scanningID := c.Param("id")

	ctx := internalhandler.NewContext(c)

	if scanningID == "" {
		ctx.RespErr = MissingIDError
		return
	}

	scanning, err := service.GetScanningModuleByID(scanningID, ctx.Logger)
	if err != nil {
		ctx.RespErr = fmt.Errorf("failed to find scanning module of id: %s, error: %s", scanningID, err)
	}
	c.Set("projectKey", scanning.ProjectName)
	c.Next()
}

// @Summary 创建代码扫描
// @Description 使用模版创建时，template_id为必传
// @Tags 	testing
// @Accept 	json
// @Produce json
// @Param 	projectName 	query		string								true	"项目标识"
// @Param 	body 			body 		service.Scanning 					true 	"body"
// @Success 200
// @Router /api/aslan/testing/scanning [post]
func CreateScanningModule(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := new(service.Scanning)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("Create scanning module c.GetRawData() err : %s", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("Create scanning module json.Unmarshal err : %s", err)
	}

	internalhandler.InsertOperationLog(c, ctx.UserName, args.ProjectName, "新增", "项目管理-代码扫描", args.Name, args.Name, string(data), types.RequestBodyTypeJSON, ctx.Logger)

	// authorization check
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[args.ProjectName]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if !ctx.Resources.ProjectAuthInfo[args.ProjectName].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[args.ProjectName].Scanning.Create {
			ctx.UnAuthorized = true
			return
		}
	}

	if err = commonutil.CheckZadigProfessionalLicense(); err != nil {
		if args.CheckQualityGate == true || len(args.AdvancedSetting.NotifyCtls) != 0 {
			ctx.RespErr = err
			return
		}
	}

	ctx.RespErr = service.CreateScanningModule(ctx.UserName, args, ctx.Logger)
}

// @Summary 更新代码扫描
// @Description body参数与创建代码扫描相同
// @Tags 	testing
// @Accept 	json
// @Produce json
// @Param 	projectName 	query		string								true	"项目标识"
// @Param 	body 			body 		service.Scanning 					true 	"body"
// @Success 200
// @Router /api/aslan/testing/scanning/{id} [put]
func UpdateScanningModule(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := new(service.Scanning)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("Update scanning module c.GetRawData() err : %s", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("Update scanning module json.Unmarshal err : %s", err)
	}

	internalhandler.InsertOperationLog(c, ctx.UserName, args.ProjectName, "修改", "项目管理-代码扫描", args.Name, args.Name, string(data), types.RequestBodyTypeJSON, ctx.Logger)

	// authorization check
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[args.ProjectName]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if !ctx.Resources.ProjectAuthInfo[args.ProjectName].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[args.ProjectName].Scanning.Edit {
			ctx.UnAuthorized = true
			return
		}
	}

	id := c.Param("id")
	if id == "" {
		ctx.RespErr = MissingIDError
		return
	}

	if err = commonutil.CheckZadigProfessionalLicense(); err != nil {
		if args.CheckQualityGate == true || len(args.AdvancedSetting.NotifyCtls) != 0 {
			ctx.RespErr = err
			return
		}
	}

	ctx.RespErr = service.UpdateScanningModule(id, ctx.UserName, args, ctx.Logger)
}

// @Summary 获取代码扫描列表
// @Description
// @Tags 	testing
// @Accept 	json
// @Produce json
// @Param 	projectName 	query		string								true	"项目标识"
// @Success 200 			{array} 	service.ListScanningRespItem
// @Router /api/aslan/testing/scanning [get]
func ListScanningModule(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Query("projectName")

	// authorization check
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectKey].Scanning.View &&
			!ctx.Resources.ProjectAuthInfo[projectKey].Workflow.Edit {
			ctx.UnAuthorized = true
			return
		}
	}

	// TODO: Authorization leak
	// this API is sometimes used in edit/create workflow scenario, thus giving the edit/create workflow permission
	// authorization check
	permitted := false

	if ctx.Resources.IsSystemAdmin {
		permitted = true
	} else if projectAuthInfo, ok := ctx.Resources.ProjectAuthInfo[projectKey]; ok {
		// first check if the user is projectAdmin
		if projectAuthInfo.IsProjectAdmin {
			permitted = true
		} else if projectAuthInfo.Workflow.Edit ||
			projectAuthInfo.Workflow.Create ||
			projectAuthInfo.Scanning.View {
			// then check if user has edit workflow permission
			permitted = true
		} else {
			// finally check if the permission is given by collaboration mode
			collaborationAuthorizedEdit, err := internalhandler.CheckPermissionGivenByCollaborationMode(ctx.UserID, projectKey, types.ResourceTypeWorkflow, types.WorkflowActionEdit)
			if err == nil && collaborationAuthorizedEdit {
				permitted = true
			}
		}
	}

	if !permitted {
		ctx.UnAuthorized = true
		return
	}

	resp, _, err := service.ListScanningModule(projectKey, ctx.Logger)
	ctx.Resp = resp
	ctx.RespErr = err
}

// @Summary 获取代码扫描详情
// @Description
// @Tags 	testing
// @Accept 	json
// @Produce json
// @Param 	projectName 	query		string								true	"项目标识"
// @Param 	id 				query		string								true	"代码扫描 ID"
// @Success 200 			{object} 	service.Scanning
// @Router /api/aslan/testing/scanning/{id} [get]
func GetScanningModule(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	id := c.Param("id")
	if id == "" {
		ctx.RespErr = MissingIDError
		return
	}

	resp, err := service.GetScanningModuleByID(id, ctx.Logger)
	if err != nil {
		ctx.RespErr = err
		return
	}

	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[resp.ProjectName]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if !ctx.Resources.ProjectAuthInfo[resp.ProjectName].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[resp.ProjectName].Scanning.View {
			ctx.UnAuthorized = true
			return
		}
	}

	ctx.Resp = resp
}

func DeleteScanningModule(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Query("projectName")

	internalhandler.InsertOperationLog(c, ctx.UserName, projectKey, "删除", "项目管理-测试", c.Param("name"), c.Param("name"), "", types.RequestBodyTypeJSON, ctx.Logger)

	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectKey].Scanning.View {
			ctx.UnAuthorized = true
			return
		}
	}

	id := c.Param("id")
	if id == "" {
		ctx.RespErr = MissingIDError
		return
	}

	ctx.RespErr = service.DeleteScanningModuleByID(id, ctx.Logger)
}

type createScanningTaskResp struct {
	TaskID int64 `json:"task_id"`
}

func CreateScanningTask(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.GetString("projectKey")
	scanningID := c.Param("id")

	//req := make([]*service.ScanningRepoInfo, 0)
	req := new(service.CreateScanningTaskReq)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("Create scanning task c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, &req); err != nil {
		log.Errorf("Create scanning task json.Unmarshal err : %v", err)
	}

	internalhandler.InsertOperationLog(c, ctx.UserName, projectKey, "新增", "代码扫描任务", scanningID, scanningID, string(data), types.RequestBodyTypeJSON, ctx.Logger)

	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectKey].Scanning.Execute {
			ctx.UnAuthorized = true
			return
		}
	}

	resp, err := service.CreateScanningTaskV2(scanningID, ctx.UserName, ctx.Account, ctx.UserID, req, "", ctx.Logger)
	if err != nil {
		ctx.RespErr = err
		return
	}
	ctx.Resp = &createScanningTaskResp{TaskID: resp}
}

type listQuery struct {
	PageSize int `json:"page_size" form:"page_size,default=100"`
	PageNum  int `json:"page_num"  form:"page_num,default=1"`
}

func ListScanningTask(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.GetString("projectKey")
	scanningID := c.Param("id")

	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectKey].Scanning.View {
			ctx.UnAuthorized = true
			return
		}
	}

	// Query Verification
	args := &listQuery{}
	if err := c.ShouldBindQuery(args); err != nil {
		ctx.RespErr = err
		return
	}

	ctx.Resp, ctx.RespErr = service.ListScanningTask(scanningID, args.PageNum, args.PageSize, ctx.Logger)
}

func GetScanningTask(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.GetString("projectKey")
	scanningID := c.Param("id")

	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectKey].Scanning.View {
			ctx.UnAuthorized = true
			return
		}
	}

	taskIDStr := c.Param("scan_id")
	if taskIDStr == "" {
		ctx.RespErr = fmt.Errorf("scan_id must be provided")
		return
	}

	taskID, err := strconv.ParseInt(taskIDStr, 10, 64)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(fmt.Sprintf("invalid task id: %s", err))
		return
	}

	ctx.Resp, ctx.RespErr = service.GetScanningTaskInfo(scanningID, taskID, ctx.Logger)
}

func CancelScanningTask(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.GetString("projectKey")
	scanningID := c.Param("id")
	taskIDStr := c.Param("scan_id")
	if taskIDStr == "" {
		ctx.RespErr = fmt.Errorf("scan_id must be provided")
		return
	}

	taskID, err := strconv.ParseInt(taskIDStr, 10, 64)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(fmt.Sprintf("invalid task id: %s", err))
		return
	}

	internalhandler.InsertOperationLog(c, ctx.UserName, projectKey, "取消", "代码扫描任务", scanningID, scanningID, taskIDStr, types.RequestBodyTypeJSON, ctx.Logger)

	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectKey].Scanning.Execute {
			ctx.UnAuthorized = true
			return
		}
	}

	workflowName := commonutil.GenScanningWorkflowName(scanningID)

	ctx.RespErr = workflowservice.CancelWorkflowTaskV4(ctx.UserName, workflowName, taskID, ctx.Logger)
}

func GetScanningArtifactInfo(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	taskIDStr := c.Param("scan_id")
	if taskIDStr == "" {
		ctx.RespErr = fmt.Errorf("scan_id must be provided")
		return
	}

	taskID, err := strconv.ParseInt(taskIDStr, 10, 64)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(fmt.Sprintf("invalid task id: %s", err))
		return
	}

	scanningID := c.Param("id")
	ctx.Resp, ctx.RespErr = service.GetWorkflowV4ScanningArtifactInfo(scanningID, c.Query("jobName"), taskID, ctx.Logger)
}

func GetScanningTaskArtifact(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Query("projectName")
	scanningID := c.Query("id")
	jobName := c.Query("jobName")
	taskIDStr := c.Query("taskID")

	// authorization check
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectKey].Scanning.View {
			ctx.UnAuthorized = true
			return
		}
	}

	taskID, err := strconv.ParseInt(taskIDStr, 10, 64)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid task id")
		return
	}

	scanningInfo, err := commonrepo.NewScanningColl().GetByID(scanningID)
	if err != nil {
		ctx.RespErr = fmt.Errorf("failed to get scanning from mongodb, the error is: %s", err)
		return
	}
	workflowName := commonutil.GenScanningWorkflowName(scanningInfo.ID.Hex())

	resp, err := workflowservice.GetWorkflowV4ArtifactFileContent(workflowName, jobName, taskID, ctx.Logger)
	if err != nil {
		ctx.RespErr = err
		return
	}
	c.Writer.Header().Set("Content-Disposition", `attachment; filename="artifact.tar.gz"`)

	c.Data(200, "application/octet-stream", resp)
}

func GetScanningTaskSSE(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.GetString("projectKey")
	scanningID := c.Param("id")

	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectKey].Scanning.View {
			ctx.UnAuthorized = true
			return
		}
	}

	taskIDStr := c.Param("scan_id")
	if taskIDStr == "" {
		ctx.RespErr = fmt.Errorf("scan_id must be provided")
		return
	}

	taskID, err := strconv.ParseInt(taskIDStr, 10, 64)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(fmt.Sprintf("invalid task id: %s", err))
		return
	}

	internalhandler.Stream(c, func(ctx1 context.Context, msgChan chan interface{}) {
		err := wait.PollImmediateUntil(time.Second, func() (bool, error) {
			res, err := service.GetScanningTaskInfo(scanningID, taskID, ctx.Logger)
			if err != nil {
				ctx.Logger.Errorf("[%s] Get scanning task info error: %s", ctx.UserName, err)
				return false, err
			}

			msgChan <- res

			return false, nil
		}, ctx1.Done())

		if err != nil && err != wait.ErrWaitTimeout {
			ctx.Logger.Error(err)
		}
	}, ctx.Logger)
}
