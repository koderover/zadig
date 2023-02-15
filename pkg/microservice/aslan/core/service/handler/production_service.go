/*
Copyright 2023 The KodeRover Authors.

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
	"encoding/json"
	"fmt"

	"github.com/gin-gonic/gin"

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/service/service"
	svcservice "github.com/koderover/zadig/pkg/microservice/aslan/core/service/service"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

func ListProductionServices(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = service.ListProductionServices(c.Query("projectName"), ctx.Logger)
}

func GetProductionK8sService(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = service.GetProductionK8sService(c.Param("name"), c.Query("projectName"), ctx.Logger)
}

func CreateK8sProductionService(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(commonmodels.Service)
	if err := c.BindJSON(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}

	rawArg, _ := json.Marshal(args)
	internalhandler.InsertOperationLog(c, ctx.UserName, args.ProductName, "新增", "项目管理-生产服务", fmt.Sprintf("服务名称:%s", args.ServiceName), string(rawArg), ctx.Logger)
	args.CreateBy = ctx.UserName
	ctx.Resp, ctx.Err = service.CreateK8sProductionService(c.Query("projectName"), args, ctx.Logger)
}

func UpdateK8sProductionServiceVariables(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(commonservice.ServiceTmplObject)
	if err := c.ShouldBindJSON(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}

	args.ProductName = c.Query("projectName")
	args.ServiceName = c.Param("name")
	args.Username = ctx.UserName
	rawArg, _ := json.Marshal(args)
	internalhandler.InsertOperationLog(c, ctx.UserName, args.ProductName, "更新", "项目管理-生产服务变量", fmt.Sprintf("服务名称:%s", args.ServiceName), string(rawArg), ctx.Logger)

	ctx.Err = svcservice.UpdateProductionServiceVariables(args)
}

func DeleteProductionService(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	internalhandler.InsertOperationLog(c, ctx.UserName, c.Query("projectName"), "删除", "项目管理-生产服务", fmt.Sprintf("服务名称:%s", c.Param("name")), "", ctx.Logger)

	ctx.Err = svcservice.DeleteProductionServiceTemplate(c.Param("name"), c.Query("projectName"), ctx.Logger)
}
