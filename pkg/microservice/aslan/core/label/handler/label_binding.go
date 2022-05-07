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

	"github.com/koderover/zadig/pkg/microservice/aslan/core/label/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/label/service"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

func lBValidate(lb []*mongodb.LabelBinding) error {
	for _, v := range lb {
		if v.LabelID == "" || v.Resource.Type == "" {
			return e.ErrInvalidParam.AddDesc("invalid labelbinding args")
		}
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
	if err := lBValidate(createLabelBindingsArgs.LabelBindings); err != nil {
		ctx.Err = err
		return
	}
	ctx.Err = service.CreateLabelBindings(createLabelBindingsArgs, ctx.UserName, ctx.Logger)
}

func DeleteLabelBindings(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	deleteLabelBindingsArgs := new(service.DeleteLabelBindingsArgs)
	if err := c.ShouldBindJSON(deleteLabelBindingsArgs); err != nil {
		ctx.Err = err
		return
	}
	if err := lBValidate(deleteLabelBindingsArgs.LabelBindings); err != nil {
		ctx.Err = err
		return
	}
	ctx.Logger.Infof("DeleteLabelBindings user:%s", ctx.UserName)
	ctx.Err = service.DeleteLabelBindings(deleteLabelBindingsArgs, ctx.UserName, ctx.Logger)
}
