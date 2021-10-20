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
	"io/ioutil"

	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/template"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	projectservice "github.com/koderover/zadig/pkg/microservice/aslan/core/project/service"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/log"
)

func GetProductTemplate(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	productTemplatName := c.Param("name")
	ctx.Resp, ctx.Err = commonservice.GetProductTemplate(productTemplatName, ctx.Logger)
}

func GetProductTemplateServices(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	productTemplatName := c.Param("name")
	ctx.Resp, ctx.Err = projectservice.GetProductTemplateServices(productTemplatName, ctx.Logger)
}

//ListProductTemplate 产品分页信息
func ListProductTemplate(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	productType := c.DefaultQuery("productType", "normal")
	if productType != "openSource" {
		ctx.Resp, ctx.Err = projectservice.ListProductTemplate(ctx.User.ID, ctx.User.IsSuperUser, ctx.Logger)
		return
	}
	ctx.Resp, ctx.Err = projectservice.ListOpenSourceProduct(ctx.Logger)
}

func CreateProductTemplate(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(template.Product)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("CreateProductTemplate c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("CreateProductTemplate json.Unmarshal err : %v", err)
	}
	internalhandler.InsertOperationLog(c, ctx.Username, args.ProductName, "新增", "项目管理-项目", args.ProductName, string(data), ctx.Logger)
	c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(data))

	if err := c.BindJSON(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid ProductTmpl json args")
		return
	}
	args.UpdateBy = ctx.Username
	ctx.Err = projectservice.CreateProductTemplate(args, ctx.Logger)
}

// UpdateProductTemplate ...
func UpdateProductTemplate(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(template.Product)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("UpdateProductTemplate c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("UpdateProductTemplate json.Unmarshal err : %v", err)
	}
	internalhandler.InsertOperationLog(c, ctx.Username, args.ProductName, "更新", "项目管理-项目环境模板或变量", args.ProductName, string(data), ctx.Logger)
	c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(data))

	if err := c.BindJSON(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid ProductTmpl json args")
		return
	}

	args.UpdateBy = ctx.Username
	ctx.Err = projectservice.UpdateProductTemplate(c.Param("name"), args, ctx.Logger)
}

func UpdateProductTmplStatus(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	productName := c.Param("name")
	onboardingStatus := c.Param("status")

	ctx.Err = projectservice.UpdateProductTmplStatus(productName, onboardingStatus, ctx.Logger)
}

func UpdateProject(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(template.Product)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("UpdateProject c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("UpdateProject json.Unmarshal err : %v", err)
	}
	internalhandler.InsertOperationLog(c, ctx.Username, args.ProductName, "更新", "项目管理-项目", args.ProductName, string(data), ctx.Logger)
	c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(data))

	if err := c.BindJSON(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid ProductTmpl json args")
		return
	}
	args.UpdateBy = ctx.Username
	productName := c.Query("productName")
	if productName == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("productName can't be empty")
		return
	}
	ctx.Err = projectservice.UpdateProject(productName, args, ctx.Logger)
}

func DeleteProductTemplate(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	internalhandler.InsertOperationLog(c, ctx.Username, c.Param("name"), "删除", "项目管理-项目", c.Param("name"), "", ctx.Logger)
	ctx.Err = projectservice.DeleteProductTemplate(ctx.Username, c.Param("name"), ctx.RequestID, ctx.Logger)
}

func ListTemplatesHierachy(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = projectservice.ListTemplatesHierachy(ctx.User.Name, ctx.User.ID, ctx.User.IsSuperUser, ctx.Logger)
}

func ForkProduct(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(template.ForkProject)
	if err := c.BindJSON(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid fork project json args")
		return
	}
	args.ProductName = c.Param("productName")
	ctx.Err = projectservice.ForkProduct(ctx.User.ID, ctx.Username, ctx.RequestID, args, ctx.Logger)
}

func UnForkProduct(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Err = projectservice.UnForkProduct(ctx.User.ID, ctx.Username, c.Param("productName"), c.Query("workflowName"), c.Query("envName"), ctx.RequestID, ctx.Logger)
}

func GetCustomMatchRules(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if c.Param("name") == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("productName can not be null!")
		return
	}

	ctx.Resp, ctx.Err = projectservice.GetCustomMatchRules(c.Param("name"), ctx.Logger)
}

func CreateOrUpdateMatchRules(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if c.Param("name") == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("productName can not be null!")
		return
	}

	internalhandler.InsertOperationLog(c, ctx.Username, c.Param("name"), "更新", "工程管理-项目", c.Param("name"), "", ctx.Logger)

	args := new(projectservice.CustomParseDataArgs)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("CreateOrUpdateMatchRules c.GetRawData() err : %v", err)
		ctx.Err = e.ErrInvalidParam
		return
	}
	if err = json.Unmarshal(data, &args); err != nil {
		log.Errorf("CreateOrUpdateMatchRules json.Unmarshal err : %v", err)
		ctx.Err = e.ErrInvalidParam
		return
	}

	ctx.Err = projectservice.UpdateCustomMatchRules(c.Param("name"), ctx.Username, args.Rules)
}
