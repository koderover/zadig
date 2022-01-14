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
	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/label/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/label/service"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

func ListLabelsBinding(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = service.ListLabelsBinding()
}

func createLBValidate(lb *models.LabelBinding) error {
	if lb.LabelID == "" || lb.ResourceType == "" || lb.ResourceID == "" {
		return e.ErrInvalidParam.AddDesc("invalid labelbinding args")
	}
	return nil
}

func CreateLabelsBinding(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	labelBinding := new(models.LabelBinding)
	if err := c.ShouldBindJSON(labelBinding); err != nil {
		ctx.Err = err
		return
	}
	if err := createLBValidate(labelBinding); err != nil {
		ctx.Err = err
		return
	}
	ctx.Err = service.CreateLabelsBinding(labelBinding, ctx.UserName, ctx.Logger)
}

func DeleteLabelsBinding(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	id := c.Param("id")
	if id == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("id must not be empty")
	}

	ctx.Err = service.DeleteLabelsBinding(id)
}
