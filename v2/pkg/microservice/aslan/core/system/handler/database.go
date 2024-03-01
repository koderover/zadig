/*
Copyright 2023 The KodeRover Authors.

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
	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

func ListDBInstance(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	encryptedKey := c.Query("encryptedKey")
	if len(encryptedKey) == 0 {
		ctx.Err = e.ErrInvalidParam
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if !ctx.Resources.SystemActions.DBInstanceManagement.View {
			ctx.UnAuthorized = true
			return
		}
	}

	ctx.Resp, ctx.Err = commonservice.ListDBInstances(encryptedKey, ctx.Logger)
}

func ListDBInstanceInfo(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	if !ctx.Resources.IsSystemAdmin {
		if !ctx.Resources.SystemActions.DBInstanceManagement.View {
			ctx.UnAuthorized = true
			return
		}
	}

	ctx.Resp, ctx.Err = commonservice.ListDBInstancesInfo(ctx.Logger)
}

// @Summary List DB Instances Info By Project
// @Description List DB Instances Info By Project
// @Tags 	system
// @Accept 	json
// @Produce json
// @Param 	projectName	query		string										true	"project name"
// @Success 200 		{array} 	commonmodels.DBInstance
// @Router /api/aslan/system/dbinstance/project [get]
func ListDBInstancesInfoByProject(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Logger.Errorf("failed to generate authorization info for user: %s, error: %s", ctx.UserID, err)
		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Query("projectName")
	if len(projectKey) == 0 {
		ctx.Err = e.ErrInvalidParam
		return
	}

	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}
	}

	ctx.Resp, ctx.Err = commonservice.ListDBInstancesInfoByProject(projectKey, ctx.Logger)
}

func CreateDBInstance(c *gin.Context) {
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

	args := new(commonmodels.DBInstance)
	if err := c.BindJSON(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid helmRepo json args")
		return
	}

	args.UpdateBy = ctx.UserName
	ctx.Err = commonservice.CreateDBInstance(args, ctx.Logger)
}

func GetDBInstance(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if !ctx.Resources.SystemActions.DBInstanceManagement.View {
			ctx.UnAuthorized = true
			return
		}
	}

	id := c.Param("id")
	if len(id) == 0 {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid db instance id")
		return
	}

	ctx.Resp, ctx.Err = commonservice.FindDBInstance(id, "")
}

func UpdateDBInstance(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if !ctx.Resources.SystemActions.DBInstanceManagement.Edit {
			ctx.UnAuthorized = true
			return
		}
	}

	id := c.Param("id")
	if len(id) == 0 {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid db instance id")
		return
	}

	args := new(commonmodels.DBInstance)
	if err := c.BindJSON(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid db json args")
		return
	}
	args.UpdateBy = ctx.UserName

	ctx.Err = commonservice.UpdateDBInstance(id, args, ctx.Logger)
}

func DeleteDBInstance(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if !ctx.Resources.SystemActions.DBInstanceManagement.Delete {
			ctx.UnAuthorized = true
			return
		}
	}

	id := c.Param("id")
	if len(id) == 0 {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid db instance id")
		return
	}

	ctx.Err = commonservice.DeleteDBInstance(id)
}

func ValidateDBInstance(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if !ctx.Resources.SystemActions.DBInstanceManagement.View && !ctx.Resources.SystemActions.DBInstanceManagement.Edit && !ctx.Resources.SystemActions.DBInstanceManagement.Create {
			ctx.UnAuthorized = true
			return
		}
	}

	args := new(commonmodels.DBInstance)
	if err := c.BindJSON(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid helmRepo json args")
		return
	}

	ctx.Err = commonservice.ValidateDBInstance(args)
}
