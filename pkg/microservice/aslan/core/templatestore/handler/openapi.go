/*
Copyright 2026 The KodeRover Authors.

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

	templateservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/templatestore/service"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
)

// @Summary List chart templates
// @Description List chart templates and their supported system variables
// @Tags OpenAPI
// @Accept json
// @Produce json
// @Success 200 {object} templateservice.OpenAPIListChartTemplatesResponse
// @Router /openapi/templates/charts [get]
func OpenAPIListChartTemplates(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	// authorization check
	if !ctx.Resources.IsSystemAdmin {
		if !ctx.Resources.SystemActions.Template.View {
			ctx.UnAuthorized = true
			return
		}
	}

	ctx.Resp, ctx.RespErr = templateservice.OpenAPIListChartTemplates(ctx.Logger)
}

// @Summary List workflow templates
// @Description List workflow template previews and the latest bindable version of each template. latest_version is 0 when the template has no version snapshot.
// @Tags OpenAPI
// @Accept json
// @Produce json
// @Param projectKey query string false "Target project key for authorization; required unless the caller is a system admin or has system template view permission"
// @Param category query string false "Workflow category filter, e.g. release; empty means no filter"
// @Param excludeBuildIn query string false "Only the string true excludes built-in templates"
// @Success 200 {object} templateservice.OpenAPIListWorkflowTemplatesResponse
// @Failure 400 {object} map[string]interface{}
// @Failure 403 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /openapi/templates/workflows [get]
func OpenAPIListWorkflowTemplates(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	// authorization check, same as the internal workflow template list
	if !ctx.Resources.IsSystemAdmin {
		if !ctx.Resources.SystemActions.Template.View {
			projectKey := c.Query("projectKey")
			if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
				ctx.UnAuthorized = true
				return
			}
			if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
				!ctx.Resources.ProjectAuthInfo[projectKey].Workflow.Create {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	excludeBuildIn := c.Query("excludeBuildIn") == "true"

	ctx.Resp, ctx.RespErr = templateservice.OpenAPIListWorkflowTemplates(c.Query("category"), excludeBuildIn, ctx.Logger)
}
