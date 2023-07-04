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
	"errors"

	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/template"
	templateservice "github.com/koderover/zadig/pkg/microservice/aslan/core/templatestore/service"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
)

func CreateDockerfileTemplate(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	req := &template.DockerfileTemplate{}

	if err := c.ShouldBindJSON(req); err != nil {
		ctx.Err = err
		return
	}

	// some dockerfile validation stuff
	err := templateservice.ValidateDockerfileTemplate(req.Content, ctx.Logger)
	if err != nil {
		ctx.Err = errors.New("invalid dockerfile, please check")
		return
	}

	bs, _ := json.Marshal(req)
	internalhandler.InsertOperationLog(c, ctx.UserName, "", "新建", "模板库-Dockerfile", req.Name, string(bs), ctx.Logger)

	ctx.Err = templateservice.CreateDockerfileTemplate(req, ctx.Logger)
}

func UpdateDockerfileTemplate(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	req := &template.DockerfileTemplate{}

	if err := c.ShouldBindJSON(req); err != nil {
		ctx.Err = err
		return
	}

	// some dockerfile validation stuff
	err := templateservice.ValidateDockerfileTemplate(req.Content, ctx.Logger)
	if err != nil {
		ctx.Err = errors.New("invalid dockerfile, please check")
		return
	}

	bs, _ := json.Marshal(req)
	internalhandler.InsertOperationLog(c, ctx.UserName, "", "更新", "模板库-Dockerfile", req.Name, string(bs), ctx.Logger)

	ctx.Err = templateservice.UpdateDockerfileTemplate(c.Param("id"), req, ctx.Logger)
}

type listDockerfileQuery struct {
	PageSize int `json:"page_size" form:"page_size,default=100"`
	PageNum  int `json:"page_num"  form:"page_num,default=1"`
}

type ListDockefileResp struct {
	DockerfileTemplates []*template.DockerfileListObject `json:"dockerfile_template"`
	Total               int                              `json:"total"`
}

func ListDockerfileTemplate(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	// Query Verification
	args := listDockerfileQuery{}
	if err := c.ShouldBindQuery(&args); err != nil {
		ctx.Err = err
		return
	}

	dockerfileTemplateList, total, err := templateservice.ListDockerfileTemplate(args.PageNum, args.PageSize, ctx.Logger)
	resp := ListDockefileResp{
		DockerfileTemplates: dockerfileTemplateList,
		Total:               total,
	}
	ctx.Resp = resp
	ctx.Err = err
}

func GetDockerfileTemplateDetail(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = template.GetDockerfileTemplateDetail(c.Param("id"), ctx.Logger)
}

func DeleteDockerfileTemplate(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	internalhandler.InsertOperationLog(c, ctx.UserName, "", "删除", "模板库-Dockerfile", c.Param("id"), "", ctx.Logger)

	ctx.Err = templateservice.DeleteDockerfileTemplate(c.Param("id"), ctx.Logger)
}

func GetDockerfileTemplateReference(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = templateservice.GetDockerfileTemplateReference(c.Param("id"), ctx.Logger)
}

type validateDockerfileTemplateReq struct {
	Content string `json:"content"`
}

type validateDockerfileTemplateResp struct {
	Error string `json:"error"`
}

func ValidateDockerfileTemplate(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	req := &validateDockerfileTemplateReq{}
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.Err = err
		return
	}

	err := templateservice.ValidateDockerfileTemplate(req.Content, ctx.Logger)
	ctx.Resp = &validateDockerfileTemplateResp{Error: ""}
	if err != nil {
		ctx.Resp = &validateDockerfileTemplateResp{Error: err.Error()}
	}
}
