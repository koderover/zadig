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
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/koderover/zadig/pkg/types"

	buildservice "github.com/koderover/zadig/pkg/microservice/aslan/core/build/service"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
)

func ListDeployTarget(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Logger.Errorf("failed to generate authorization info for user: %s, error: %s", ctx.UserID, err)
		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Query("projectName")

	// authorization checks
	// FIX ME LATER: return empty when not authorized
	// FIX ME LATER 2: the authorization requirement for this api has been changed from
	// View Build -> View Service. If this is ok remove the TODO, if not just do a rollback
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectKey].Service.View {
			ctx.UnAuthorized = true
			return
		}
	}

	ctx.Resp, ctx.Err = buildservice.ListDeployTarget(projectKey, ctx.Logger)
}

func ListBuildModulesForProduct(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Logger.Errorf("failed to generate authorization info for user: %s, error: %s", ctx.UserID, err)
		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Param("productName")

	// TODO: Authorization leak
	// authorization checks
	permitted := false

	if ctx.Resources.IsSystemAdmin {
		permitted = true
	}

	if projectedAuthInfo, ok := ctx.Resources.ProjectAuthInfo[projectKey]; ok {
		if projectedAuthInfo.IsProjectAdmin {
			permitted = true
		}

		if projectedAuthInfo.Build.View ||
			projectedAuthInfo.Workflow.Execute {
			permitted = true
		}

		collaborationAuthorizedEdit, err := internalhandler.CheckPermissionGivenByCollaborationMode(ctx.UserID, projectKey, types.ResourceTypeWorkflow, types.WorkflowActionRun)
		if err == nil {
			permitted = collaborationAuthorizedEdit
		}
	}

	if !permitted {
		ctx.UnAuthorized = true
		return
	}

	containerList, err := buildservice.ListContainers(projectKey, ctx.Logger)
	if err != nil {
		ctx.Err = err
		return
	}

	ctx.Resp, ctx.Err = buildservice.ListBuildForProduct(projectKey, containerList, ctx.Logger)
}
