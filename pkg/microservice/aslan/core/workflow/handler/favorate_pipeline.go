/*
Copyright 2021 The KodeRover Authors.

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
	"io"

	"github.com/gin-gonic/gin"
	"github.com/koderover/zadig/v2/pkg/types"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/workflow/service/workflow"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

func ListFavoritePipelines(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	productName := c.Query("projectName")
	workflowType := c.Query("type")
	if workflowType == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("type can't be empty!")
		return
	}
	ctx.Resp, ctx.RespErr = workflow.ListFavoritePipelines(&commonrepo.FavoriteArgs{UserID: ctx.UserID, ProductName: productName, Type: workflowType})
}

func DeleteFavoritePipeline(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	productName := c.Param("productName")
	workflowName := c.Param("name")
	workflowType := c.Param("type")
	if workflowName == "" || workflowType == "" || productName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("productName or name or type can't be empty!")
		return
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, productName, "删除", "收藏工作流", workflowName, workflowName, "", types.RequestBodyTypeJSON, ctx.Logger)
	ctx.RespErr = workflow.DeleteFavoritePipeline(&commonrepo.FavoriteArgs{UserID: ctx.UserID, ProductName: productName, Type: workflowType, Name: workflowName})
}

func CreateFavoritePipeline(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(commonmodels.Favorite)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("CreateFavoritePipeline c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("CreateFavoritePipeline json.Unmarshal err : %v", err)
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, args.ProductName, "新增", "收藏工作流", args.Name, args.Name, string(data), types.RequestBodyTypeJSON, ctx.Logger)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	if err := c.BindJSON(args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid Favorite json args")
		return
	}
	args.UserID = ctx.UserID

	ctx.RespErr = workflow.CreateFavoritePipeline(args, ctx.Logger)
}
