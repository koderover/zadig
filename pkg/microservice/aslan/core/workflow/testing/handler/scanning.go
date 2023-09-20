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

	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/workflow/testing/service"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	"github.com/koderover/zadig/pkg/tool/log"
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
		ctx.Err = MissingIDError
		return
	}

	scanning, err := service.GetScanningModuleByID(scanningID, ctx.Logger)
	if err != nil {
		ctx.Err = fmt.Errorf("failed to find scanning module of id: %s, error: %s", scanningID, err)
	}
	c.Set("projectKey", scanning.ProjectName)
	c.Next()
}

func CreateScanningModule(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
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

	internalhandler.InsertOperationLog(c, ctx.UserName, args.ProjectName, "新增", "项目管理-代码扫描", args.Name, string(data), ctx.Logger)

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

	ctx.Err = service.CreateScanningModule(ctx.UserName, args, ctx.Logger)
}

func UpdateScanningModule(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
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

	internalhandler.InsertOperationLog(c, ctx.UserName, args.ProjectName, "修改", "项目管理-代码扫描", args.Name, string(data), ctx.Logger)

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
		ctx.Err = MissingIDError
		return
	}

	ctx.Err = service.UpdateScanningModule(id, ctx.UserName, args, ctx.Logger)
}

func ListScanningModule(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
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
		}

		// then check if user has edit workflow permission
		if projectAuthInfo.Workflow.Edit ||
			projectAuthInfo.Workflow.Create ||
			projectAuthInfo.Scanning.View {
			permitted = true
		}

		// finally check if the permission is given by collaboration mode
		collaborationAuthorizedEdit, err := internalhandler.CheckPermissionGivenByCollaborationMode(ctx.UserID, projectKey, types.ResourceTypeWorkflow, types.WorkflowActionEdit)
		if err == nil && collaborationAuthorizedEdit {
			permitted = true
		}
	}

	if !permitted {
		ctx.UnAuthorized = true
		return
	}

	resp, _, err := service.ListScanningModule(projectKey, ctx.Logger)
	ctx.Resp = resp
	ctx.Err = err
}

func GetScanningModule(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	id := c.Param("id")
	if id == "" {
		ctx.Err = MissingIDError
		return
	}

	resp, err := service.GetScanningModuleByID(id, ctx.Logger)
	if err != nil {
		ctx.Err = err
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

		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Query("projectName")

	internalhandler.InsertOperationLog(c, ctx.UserName, projectKey, "删除", "项目管理-测试", c.Param("name"), "", ctx.Logger)

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
		ctx.Err = MissingIDError
		return
	}

	ctx.Err = service.DeleteScanningModuleByID(id, ctx.Logger)
}

type createScanningTaskResp struct {
	TaskID int64 `json:"task_id"`
}

func CreateScanningTask(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.GetString("projectKey")
	scanningID := c.Param("id")

	req := make([]*service.ScanningRepoInfo, 0)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("Create scanning task c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, &req); err != nil {
		log.Errorf("Create scanning task json.Unmarshal err : %v", err)
	}

	internalhandler.InsertOperationLog(c, ctx.UserName, projectKey, "新增", "代码扫描任务", scanningID, string(data), ctx.Logger)

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

	resp, err := service.CreateScanningTask(scanningID, req, "", ctx.UserName, ctx.Logger)
	if err != nil {
		ctx.Err = err
		return
	}
	ctx.Resp = &createScanningTaskResp{TaskID: resp}
}

type listQuery struct {
	PageSize int64 `json:"page_size" form:"page_size,default=100"`
	PageNum  int64 `json:"page_num"  form:"page_num,default=1"`
}

func ListScanningTask(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
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
		ctx.Err = err
		return
	}

	ctx.Resp, ctx.Err = service.ListScanningTask(scanningID, args.PageNum, args.PageSize, ctx.Logger)
}

func GetScanningTask(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
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
		ctx.Err = fmt.Errorf("scan_id must be provided")
		return
	}

	taskID, err := strconv.ParseInt(taskIDStr, 10, 64)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(fmt.Sprintf("invalid task id: %s", err))
		return
	}

	ctx.Resp, ctx.Err = service.GetScanningTaskInfo(scanningID, taskID, ctx.Logger)
}

func CancelScanningTask(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.GetString("projectKey")
	scanningID := c.Param("id")
	taskIDStr := c.Param("scan_id")
	if taskIDStr == "" {
		ctx.Err = fmt.Errorf("scan_id must be provided")
		return
	}

	taskID, err := strconv.ParseInt(taskIDStr, 10, 64)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(fmt.Sprintf("invalid task id: %s", err))
		return
	}

	internalhandler.InsertOperationLog(c, ctx.UserName, projectKey, "取消", "代码扫描任务", scanningID, taskIDStr, ctx.Logger)

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

	ctx.Err = service.CancelScanningTask(ctx.UserName, scanningID, taskID, config.ScanningType, ctx.RequestID, ctx.Logger)
}

func GetScanningTaskSSE(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
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
		ctx.Err = fmt.Errorf("scan_id must be provided")
		return
	}

	taskID, err := strconv.ParseInt(taskIDStr, 10, 64)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc(fmt.Sprintf("invalid task id: %s", err))
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
