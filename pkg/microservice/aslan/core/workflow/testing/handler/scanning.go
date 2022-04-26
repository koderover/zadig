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
	"encoding/json"
	"fmt"
	"io/ioutil"

	"github.com/gin-gonic/gin"

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/workflow/testing/service"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	"github.com/koderover/zadig/pkg/tool/log"
)

func GetScanningProductName(c *gin.Context) {
	args := new(commonmodels.Testing)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("c.GetRawData() err : %v", err)
		return
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("json.Unmarshal err : %v", err)
		return
	}
	c.Set("productName", args.ProductName)
	c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(data))
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
	c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(data))

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
	c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(data))

	if err != nil {
		ctx.Err = fmt.Errorf("update scanning module err : %s", err)
		return
	}

	id := c.Param("id")
	if id == "" {
		ctx.Err = fmt.Errorf("id must be provided")
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
		ctx.Err = fmt.Errorf("id must be provided")
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
		ctx.Err = fmt.Errorf("id must be provided")
		return
	}

	ctx.Err = service.DeleteScanningModuleByID(id, ctx.Logger)
}
