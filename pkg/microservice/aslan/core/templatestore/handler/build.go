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
	"io/ioutil"

	e "github.com/koderover/zadig/pkg/tool/errors"

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"

	"github.com/gin-gonic/gin"
	templateservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/template"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	"github.com/koderover/zadig/pkg/tool/log"
)

func GetBuildTemplate(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = templateservice.GetBuildTemplateByID(c.Param("id"))
}

func ListBuildTemplates(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := &listYamlQuery{}
	if err := c.ShouldBindQuery(args); err != nil {
		ctx.Err = err
		return
	}

	ctx.Resp, ctx.Err = templateservice.ListBuildTemplates(args.PageNum, args.PageSize)
}

func AddBuildTemplate(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(commonmodels.BuildTemplate)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("CreateBuildModule c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("CreateBuildModule json.Unmarshal err : %v", err)
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, "", "新增", "模板-构建", args.Name, string(data), ctx.Logger)
	c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(data))

	err = c.BindJSON(args)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid Build args")
		return
	}

	ctx.Err = templateservice.AddBuildTemplate(ctx.UserName, args, ctx.Logger)
}

func UpdateBuildTemplate(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(commonmodels.BuildTemplate)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("UpdateBuildTemplate c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("UpdateBuildTemplate json.Unmarshal err : %v", err)
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, "", "编辑", "模板-构建", args.Name, string(data), ctx.Logger)
	c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(data))
	err = c.BindJSON(args)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid Build args")
		return
	}

	ctx.Err = templateservice.UpdateBuildTemplate(c.Param("id"), args, ctx.Logger)
}

func RemoveBuildTemplate(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Err = templateservice.RemoveBuildTemplate(c.Param("id"), ctx.Logger)
}
