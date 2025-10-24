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
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/system/service"
	"github.com/koderover/zadig/v2/pkg/setting"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/types"
)

func GetOperationLogs(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	envName := c.Query("envName")
	projectKey := c.Query("projectName")
	production := c.Query("production") == "true"
	if len(projectKey) == 0 {
		ctx.RespErr = e.ErrFindOperationLog.AddDesc("projectName can't be nil")
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if production {
			if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
				!ctx.Resources.ProjectAuthInfo[projectKey].ProductionEnv.EditConfig {
				permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, projectKey, types.ResourceTypeEnvironment, envName, types.ProductionEnvActionEditConfig)
				if err != nil || !permitted {
					ctx.UnAuthorized = true
					return
				}
			}

			err = commonutil.CheckZadigProfessionalLicense()
			if err != nil {
				ctx.RespErr = err
				return
			}
		} else {
			if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
				!ctx.Resources.ProjectAuthInfo[projectKey].Env.EditConfig {
				permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, projectKey, types.ResourceTypeEnvironment, envName, types.EnvActionEditConfig)
				if err != nil || !permitted {
					ctx.UnAuthorized = true
					return
				}
			}
		}
	}

	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil {
		ctx.RespErr = e.ErrFindOperationLog.AddErr(err)
		return
	}

	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "50"))
	if err != nil {
		ctx.RespErr = e.ErrFindOperationLog.AddErr(err)
		return
	}

	status, _ := strconv.Atoi(c.DefaultQuery("status", "0"))
	if err != nil {
		ctx.RespErr = e.ErrFindOperationLog.AddErr(err)
		return
	}

	args := &service.OperationLogArgs{
		ExactProduct: projectKey,
		Username:     c.Query("username"),
		Function:     c.Query("function"),
		Scene:        setting.OperationSceneEnv,
		TargetID:     envName,
		Status:       status,
		PerPage:      pageSize,
		Page:         page,
		Detail:       c.Query("detail"),
	}

	resp, count, err := service.FindOperation(args, ctx.Logger)
	ctx.Resp = resp
	ctx.RespErr = err
	c.Writer.Header().Set("X-Total", strconv.Itoa(count))
}
