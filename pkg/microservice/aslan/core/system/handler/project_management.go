/*
 * Copyright 2022 The KodeRover Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/system/service"
	"github.com/koderover/zadig/pkg/setting"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/jira"
	"github.com/koderover/zadig/pkg/tool/meego"
)

func ListProjectManagement(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	ctx.Resp, ctx.Err = service.ListProjectManagement(ctx.Logger)
}

func CreateProjectManagement(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	req := new(models.ProjectManagement)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.Err = err
		return
	}
	ctx.Err = service.CreateProjectManagement(req, ctx.Logger)
}

func UpdateProjectManagement(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	req := new(models.ProjectManagement)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.Err = err
		return
	}
	ctx.Err = service.UpdateProjectManagement(c.Param("id"), req, ctx.Logger)
}

func DeleteProjectManagement(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	ctx.Err = service.DeleteProjectManagement(c.Param("id"), ctx.Logger)
}

func Validate(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	req := new(models.ProjectManagement)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.Err = err
		return
	}
	switch req.Type {
	case setting.PMJira:
		ctx.Err = service.ValidateJira(req)
	case setting.PMMeego:
		ctx.Err = service.ValidateMeego(req)
	default:
		ctx.Err = e.ErrValidateProjectManagement.AddDesc("invalid type")
	}
}

func ListJiraProjects(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	ctx.Resp, ctx.Err = service.ListJiraProjects()
}

func SearchJiraIssues(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	ctx.Resp, ctx.Err = service.SearchJiraIssues(c.Query("project"), c.Query("type"), c.Query("status"), c.Query("summary"), c.Query("ne") == "true")
}

func GetJiraTypes(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	ctx.Resp, ctx.Err = service.GetJiraTypes(c.Query("project"))
}

func HandleJiraEvent(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	event := new(jira.Event)
	if err := c.ShouldBindJSON(event); err != nil {
		ctx.Err = err
		return
	}

	ctx.Err = service.HandleJiraHookEvent(c.Param("workflowName"), c.Param("hookName"), event, ctx.Logger)
}

func HandleMeegoEvent(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	event := new(meego.GeneralWebhookRequest)
	if err := c.ShouldBindJSON(event); err != nil {
		ctx.Err = err
		return
	}

	ctx.Err = service.HandleMeegoHookEvent(c.Param("workflowName"), c.Param("hookName"), event, ctx.Logger)
}
