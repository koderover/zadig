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
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/label/repository/dto"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/label/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/label/service"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

type ListLabelsArgs struct {
	Key       string   `json:"key" form:"key"`
	Values    []string `json:"values" form:"values"`
	LabelType string   `json:"type" form:"type"`
}

func ListLabels(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	listLabelsArgs := new(ListLabelsArgs)
	if err := c.ShouldBindQuery(listLabelsArgs); err != nil {
		ctx.Err = err
		return
	}
	ctx.Resp, ctx.Err = service.ListLabels(listLabelsArgs.Key, listLabelsArgs.Values, listLabelsArgs.LabelType)
}

func createLabelValidate(lb *models.Label) error {
	if lb.Key == "" || lb.Value == "" {
		return e.ErrInvalidParam.AddDesc("invalid label args")
	}
	return nil
}

func CreateLabel(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	label := new(models.Label)
	if err := c.ShouldBindJSON(label); err != nil {
		ctx.Err = err
		return
	}

	if err := createLabelValidate(label); err != nil {
		ctx.Err = err
		return
	}
	label.CreateBy = ctx.UserName
	ctx.Err = service.CreateLabel(label)
}

func DeleteLabel(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	id := c.Param("id")
	if id == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("id must not be empty")
		return
	}
	force := c.Query("force")
	forceBool, _ := strconv.ParseBool(force)
	ctx.Err = service.DeleteLabel(id, forceBool, ctx.Logger)
}

type ListResourceByLabelsReq struct {
	LabelFilters []dto.LabelFilter `json:"label_filters"`
}

func ListResourceByLabels(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	listResourceByLabelsReq := new(ListResourceByLabelsReq)
	if err := c.ShouldBindJSON(listResourceByLabelsReq); err != nil {
		ctx.Err = err
		return
	}
	ctx.Resp, ctx.Err = service.ListResourcesByLabels(listResourceByLabelsReq.LabelFilters, ctx.Logger)
}

type ListLabelsByResourceReq struct {
	ResourceID   string `json:"resource_id"`
	ResourceType string `json:"resource_type"`
}

func ListLabelsByResource(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	listLabelsByResourceReq := new(ListLabelsByResourceReq)
	if err := c.ShouldBindJSON(listLabelsByResourceReq); err != nil {
		ctx.Err = err
		return
	}
	ctx.Resp, ctx.Err = service.ListLabelsByResourceID(listLabelsByResourceReq.ResourceID, listLabelsByResourceReq.ResourceType, ctx.Logger)
}
