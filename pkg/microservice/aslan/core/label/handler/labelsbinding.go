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

	"github.com/koderover/zadig/pkg/microservice/aslan/core/label/service"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

func ListLabelBindings(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = service.ListLabelsBinding()
}

func createLBValidate(cr *service.CreateLabelBindingsArgs) error {
	if cr.LabelID == "" || cr.ResourceType == "" || len(cr.ResourceIDs) == 0 {
		return e.ErrInvalidParam.AddDesc("invalid labelbinding args")
	}
	return nil
}

func CreateLabelBindings(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	createLabelBindingsArgs := new(service.CreateLabelBindingsArgs)
	if err := c.ShouldBindJSON(createLabelBindingsArgs); err != nil {
		ctx.Err = err
		return
	}
	if err := createLBValidate(createLabelBindingsArgs); err != nil {
		ctx.Err = err
		return
	}
	createLabelBindingsArgs.CreateBy = ctx.UserName
	ctx.Err = service.CreateLabelsBinding(createLabelBindingsArgs, ctx.Logger)
}

func DeleteLabelBindings(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	deleteLabelsBindingsArgs := new(service.DeleteLabelsBindingsArgs)
	if err := c.ShouldBindJSON(deleteLabelsBindingsArgs); err != nil {
		ctx.Err = err
		return
	}

	ctx.Err = service.DeleteLabelsBindings(deleteLabelsBindingsArgs.IDs)
}
