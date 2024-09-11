/*
Copyright 2024 The KodeRover Authors.

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
	"fmt"

	"github.com/gin-gonic/gin"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/system/service"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

func CreateServiceLabel(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if !ctx.Resources.SystemActions.DBInstanceManagement.Create {
			ctx.UnAuthorized = true
			return
		}
	}

	args := new(commonmodels.Label)
	if err := c.BindJSON(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid label json args")
		return
	}

	ctx.Err = service.CreateServiceLabel(args, ctx.Logger)
}

func ListServiceLabels(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	// TODO: List service label have no authorization right now because it does not need one.

	ctx.Resp, ctx.Err = service.ListServiceLabels(ctx.Logger)
}

func UpdateServiceLabel(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if !ctx.Resources.SystemActions.DBInstanceManagement.Create {
			ctx.UnAuthorized = true
			return
		}
	}

	args := new(commonmodels.Label)
	if err := c.BindJSON(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid label json args")
		return
	}

	id := c.Param("id")
	if len(id) == 0 {
		ctx.Err = e.ErrInvalidParam.AddDesc("id cannot be empty")
		return
	}

	ctx.Err = service.UpdateServiceLabel(id, args, ctx.Logger)
}

func DeleteServiceLabel(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if !ctx.Resources.SystemActions.DBInstanceManagement.Create {
			ctx.UnAuthorized = true
			return
		}
	}

	id := c.Param("id")
	if len(id) == 0 {
		ctx.Err = e.ErrInvalidParam.AddDesc("id cannot be empty")
		return
	}

	ctx.Err = service.DeleteServiceLabel(id, ctx.Logger)
}
