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
	"encoding/json"

	"github.com/gin-gonic/gin"

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/fs"
	templateservice "github.com/koderover/zadig/pkg/microservice/aslan/core/templatestore/service"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	"github.com/koderover/zadig/pkg/tool/errors"
)

type addChartArgs struct {
	*fs.DownloadFromSourceArgs

	Name string `json:"name"`
}

func GetChartTemplate(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = templateservice.GetChartTemplate(c.Param("name"), ctx.Logger)
}

func GetTemplateVariables(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = templateservice.GetChartTemplateVariables(c.Param("name"), ctx.Logger)
}

func ListFiles(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	// TODO: support to return multiple files in a bulk
	ctx.Resp, ctx.Err = templateservice.GetFileContentForTemplate(c.Param("name"), c.Query("filePath"), c.Query("fileName"), ctx.Logger)
}

func ListChartTemplates(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = templateservice.ListChartTemplates(ctx.Logger)
}

func AddChartTemplate(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := &addChartArgs{}
	if err := c.ShouldBindJSON(args); err != nil {
		ctx.Err = err
		return
	}

	bs, _ := json.Marshal(args)
	internalhandler.InsertOperationLog(c, ctx.UserName, "", "新建", "模板库-Chart", args.Name, string(bs), ctx.Logger)

	ctx.Err = templateservice.AddChartTemplate(args.Name, args.DownloadFromSourceArgs, ctx.Logger)
}

func UpdateChartTemplate(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := &addChartArgs{}
	if err := c.ShouldBindJSON(args); err != nil {
		ctx.Err = err
		return
	}

	bs, _ := json.Marshal(args)
	internalhandler.InsertOperationLog(c, ctx.UserName, "", "更新", "模板库-Chart", args.Name, string(bs), ctx.Logger)

	ctx.Err = templateservice.UpdateChartTemplate(c.Param("name"), args.DownloadFromSourceArgs, ctx.Logger)
}

func UpdateChartTemplateVariables(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := make([]*commonmodels.Variable, 0)
	if err := c.ShouldBindJSON(&args); err != nil {
		ctx.Err = errors.ErrInvalidParam.AddErr(err)
		return
	}

	ctx.Err = templateservice.UpdateChartTemplateVariables(c.Param("name"), args, ctx.Logger)
}

func GetChartTemplateReference(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = templateservice.GetChartTemplateReference(c.Param("name"), ctx.Logger)
}

func SyncChartTemplateReference(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	internalhandler.InsertOperationLog(c, ctx.UserName, "", "同步", "模板库-Chart", c.Param("name"), "", ctx.Logger)

	ctx.Err = templateservice.SyncHelmTemplateReference(ctx.UserName, c.Param("name"), ctx.Logger)
}

func RemoveChartTemplate(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	internalhandler.InsertOperationLog(c, ctx.UserName, "", "删除", "模板库-Chart", c.Param("name"), "", ctx.Logger)

	ctx.Err = templateservice.RemoveChartTemplate(c.Param("name"), ctx.Logger)
}
