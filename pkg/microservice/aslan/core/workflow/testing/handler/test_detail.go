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

	"github.com/koderover/zadig/pkg/microservice/aslan/core/workflow/testing/service"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
)

func ListDetailTestModules(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.Err = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Query("projectName")

	// TODO: Authorization leak
	// this API is sometimes used in edit workflow scenario, thus giving the edit workflow permission
	// authorization check
	if !ctx.Resources.IsSystemAdmin {
		authorized := false
		if projectAuthInfo, ok := ctx.Resources.ProjectAuthInfo[projectKey]; ok {
			// first check if the user is projectAdmin
			if projectAuthInfo.IsProjectAdmin {
				authorized = true
			}

			// then check if the user has view test permission
			if projectAuthInfo.Test.View {
				authorized = true
			}

			// then check if user has edit workflow permission
			if projectAuthInfo.Workflow.Edit ||
				projectAuthInfo.Workflow.Create {
				authorized = true
			}

			// finally check if the permission is given by collaboration mode
			collaborationAuthorized, err := internalhandler.CheckPermissionGivenByCollaborationMode(ctx.UserID, projectKey, types.ResourceTypeWorkflow, types.WorkflowActionEdit)
			if err == nil {
				authorized = collaborationAuthorized
			}
		}
		if !authorized {
			ctx.UnAuthorized = true
			return
		}
	}

	ctx.Resp, ctx.Err = service.ListTestingDetails(projectKey, ctx.Logger)
}
