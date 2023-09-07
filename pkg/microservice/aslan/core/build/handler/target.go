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

		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Query("projectName")

	// TODO: Authorization leak
	// this API is sometimes used in edit/create workflow scenario, thus giving the edit/create workflow permission
	// authorization checks
	permitted := false

	if ctx.Resources.IsSystemAdmin {
		permitted = true
	} else if projectAuthInfo, ok := ctx.Resources.ProjectAuthInfo[projectKey]; ok {
		// first check if the user is projectAdmin
		if projectAuthInfo.IsProjectAdmin {
			permitted = true
		}

		// then check if user has edit workflow permission
		if projectAuthInfo.Service.View ||
			projectAuthInfo.Env.EditConfig {
			permitted = true
		}

		// finally check if the permission is given by collaboration mode
		collaborationAuthorizedEdit, err := internalhandler.CheckPermissionGivenByCollaborationMode(ctx.UserID, projectKey, types.ResourceTypeEnvironment, types.EnvActionEditConfig)
		if err == nil && collaborationAuthorizedEdit {
			permitted = true
		}
	}

	if !permitted {
		ctx.UnAuthorized = true
		return
	}

	ctx.Resp, ctx.Err = buildservice.ListDeployTarget(projectKey, ctx.Logger)
}

func ListBuildModulesForProduct(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

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
	} else if projectedAuthInfo, ok := ctx.Resources.ProjectAuthInfo[projectKey]; ok {
		if projectedAuthInfo.IsProjectAdmin {
			permitted = true
		}

		if projectedAuthInfo.Build.View ||
			projectedAuthInfo.Workflow.Execute {
			permitted = true
		}

		collaborationAuthorizedEdit, err := internalhandler.CheckPermissionGivenByCollaborationMode(ctx.UserID, projectKey, types.ResourceTypeWorkflow, types.WorkflowActionRun)
		if err == nil && collaborationAuthorizedEdit {
			permitted = true
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
