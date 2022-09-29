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

func CreateScanningModule(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(service.Scanning)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("Create scanning module c.GetRawData() err : %s", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("Create scanning module json.Unmarshal err : %s", err)
	}

	internalhandler.InsertOperationLog(c, ctx.UserName, args.ProjectName, "新增", "项目管理-代码扫描", args.Name, string(data), ctx.Logger)

	if err != nil {
		ctx.Err = fmt.Errorf("create scanning module err : %s", err)
		return
	}

	ctx.Err = service.CreateScanningModule(ctx.UserName, args, ctx.Logger)
}

func UpdateScanningModule(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(service.Scanning)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("Update scanning module c.GetRawData() err : %s", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("Update scanning module json.Unmarshal err : %s", err)
	}

	internalhandler.InsertOperationLog(c, ctx.UserName, args.ProjectName, "修改", "项目管理-代码扫描", args.Name, string(data), ctx.Logger)

	if err != nil {
		ctx.Err = fmt.Errorf("update scanning module err : %s", err)
		return
	}

	id := c.Param("id")
	if id == "" {
		ctx.Err = MissingIDError
		return
	}

	ctx.Err = service.UpdateScanningModule(id, ctx.UserName, args, ctx.Logger)
}

func ListScanningModule(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	resp, _, err := service.ListScanningModule(c.Query("projectName"), ctx.Logger)
	ctx.Resp = resp
	ctx.Err = err
}

func GetScanningModule(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	id := c.Param("id")
	if id == "" {
		ctx.Err = MissingIDError
		return
	}

	ctx.Resp, ctx.Err = service.GetScanningModuleByID(id, ctx.Logger)
}

func DeleteScanningModule(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	internalhandler.InsertOperationLog(c, ctx.UserName, c.Query("projectName"), "删除", "项目管理-测试", c.Param("name"), "", ctx.Logger)

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
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	id := c.Param("id")
	if id == "" {
		ctx.Err = MissingIDError
		return
	}

	req := make([]*service.ScanningRepoInfo, 0)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("Create scanning task c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, &req); err != nil {
		log.Errorf("Create scanning task json.Unmarshal err : %v", err)
	}

	internalhandler.InsertOperationLog(c, ctx.UserName, "", "新增", "代码扫描任务", id, string(data), ctx.Logger)

	resp, err := service.CreateScanningTask(id, req, "", ctx.UserName, ctx.Logger)
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
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	id := c.Param("id")
	if id == "" {
		ctx.Err = MissingIDError
		return
	}

	// Query Verification
	args := &listQuery{}
	if err := c.ShouldBindQuery(args); err != nil {
		ctx.Err = err
		return
	}

	ctx.Resp, ctx.Err = service.ListScanningTask(id, args.PageNum, args.PageSize, ctx.Logger)
}

func GetScanningTask(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	id := c.Param("id")
	if id == "" {
		ctx.Err = MissingIDError
		return
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

	ctx.Resp, ctx.Err = service.GetScanningTaskInfo(id, taskID, ctx.Logger)
}

func CancelScanningTask(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	id := c.Param("id")
	if id == "" {
		ctx.Err = MissingIDError
		return
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

	internalhandler.InsertOperationLog(c, ctx.UserName, "", "取消", "代码扫描任务", id, taskIDStr, ctx.Logger)

	ctx.Err = service.CancelScanningTask(ctx.UserName, id, taskID, config.ScanningType, ctx.RequestID, ctx.Logger)
}

func GetScanningTaskSSE(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	id := c.Param("id")
	if id == "" {
		ctx.Err = MissingIDError
		return
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
			res, err := service.GetScanningTaskInfo(id, taskID, ctx.Logger)
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
