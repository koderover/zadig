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
	"fmt"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/system/service"
	"github.com/koderover/zadig/v2/pkg/setting"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/jira"
	"github.com/koderover/zadig/v2/pkg/tool/meego"
)

func ListProjectManagement(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		ctx.UnAuthorized = true
		return
	}

	ctx.Resp, ctx.RespErr = service.ListProjectManagement(ctx.Logger)
}

// @Summary List Project Management For Project
// @Description List Project Management For Project
// @Tags 	system
// @Accept 	json
// @Produce json
// @Success 200 	{array} 	models.ProjectManagement
// @Router /api/aslan/system/project_management/project [get]
func ListProjectManagementForProject(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Logger.Errorf("failed to generate authorization info for user: %s, error: %s", ctx.UserID, err)
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	pms, err := service.ListProjectManagement(ctx.Logger)
	for _, pm := range pms {
		pm.JiraToken = ""
		pm.JiraUser = ""
		pm.JiraAuthType = ""
		pm.MeegoPluginID = ""
		pm.MeegoPluginSecret = ""
		pm.MeegoUserKey = ""
	}
	ctx.RespErr = err
	ctx.Resp = pms
}

func CreateProjectManagement(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		ctx.UnAuthorized = true
		return
	}

	req := new(models.ProjectManagement)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.RespErr = err
		return
	}

	err = util.CheckZadigEnterpriseLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	ctx.RespErr = service.CreateProjectManagement(req, ctx.Logger)
}

func UpdateProjectManagement(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		ctx.UnAuthorized = true
		return
	}

	req := new(models.ProjectManagement)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.RespErr = err
		return
	}

	err = util.CheckZadigEnterpriseLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	ctx.RespErr = service.UpdateProjectManagement(c.Param("id"), req, ctx.Logger)
}

func DeleteProjectManagement(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		ctx.UnAuthorized = true
		return
	}

	ctx.RespErr = service.DeleteProjectManagement(c.Param("id"), ctx.Logger)
}

func Validate(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	// authorization checks
	if !ctx.Resources.IsSystemAdmin {
		ctx.UnAuthorized = true
		return
	}

	req := new(models.ProjectManagement)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.RespErr = err
		return
	}
	switch req.Type {
	case setting.PMJira:
		ctx.RespErr = service.ValidateJira(req)
	case setting.PMMeego:
		ctx.RespErr = service.ValidateMeego(req)
	case setting.PMPingCode:
		ctx.RespErr = service.ValidatePingCode(req)
	default:
		ctx.RespErr = e.ErrValidateProjectManagement.AddDesc("invalid type")
	}
}

// @Summary List Jira Projects
// @Description List Jira Projects
// @Tags 	system
// @Accept 	json
// @Produce json
// @Param 	id 		path		string										true	"jira id"
// @Success 200 	{array} 	service.JiraProjectResp
// @Router /api/aslan/system/project_management/{id}/jira/project [get]
func ListJiraProjects(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	ctx.Resp, ctx.RespErr = service.ListJiraProjects(c.Param("id"))
}

// @Summary List Jira Boards
// @Description List Jira Boards
// @Tags 	system
// @Accept 	json
// @Produce json
// @Param 	id 			path		string										true	"jira id"
// @Param 	projectKey 	query		string										true	"jira project key"
// @Success 200 		{array} 	service.JiraBoardResp
// @Router /api/aslan/system/project_management/{id}/jira/board [get]
func ListJiraBoards(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	ctx.Resp, ctx.RespErr = service.ListJiraBoards(c.Param("id"), c.Query("projectKey"))
}

// @Summary List Jira Sprints
// @Description List Jira Sprints
// @Tags 	system
// @Accept 	json
// @Produce json
// @Param 	id 			path		string										true	"jira id"
// @Param 	boardID 	path		string										true	"jira board id"
// @Success 200 		{array} 	service.JiraSprintResp
// @Router /api/aslan/system/project_management/{id}/jira/board/:boardID/sprint [get]
func ListJiraSprints(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	boardID, err := strconv.Atoi(c.Param("boardID"))
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid board id")
		return

	}

	ctx.Resp, ctx.RespErr = service.ListJiraSprints(c.Param("id"), boardID)
}

// @Summary Get Jira Sprint
// @Description Get Jira Sprint
// @Tags 	system
// @Accept 	json
// @Produce json
// @Param 	id 			path		string										true	"jira id"
// @Param 	sprintID 	path		string										true	"jira sprint id"
// @Success 200 		{object} 	service.JiraSprintResp
// @Router /api/aslan/system/project_management/{id}/jira/sprint/:sprintID [get]
func GetJiraSprint(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	sprintID, err := strconv.Atoi(c.Param("sprintID"))
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid sprint id")
		return

	}

	ctx.Resp, ctx.RespErr = service.GetJiraSprint(c.Param("id"), sprintID)
}

// @Summary List Jira Sprint Issues
// @Description List Jira Sprint Issues
// @Tags 	system
// @Accept 	json
// @Produce json
// @Param 	id 			path		string										true	"jira id"
// @Param 	sprintID 	path		string										true	"jira sprint id"
// @Success 200 		{object} 	service.JiraSprintResp
// @Router /api/aslan/system/project_management/{id}/jira/sprint/:sprintID/issue [get]
func ListJiraSprintIssues(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	sprintID, err := strconv.Atoi(c.Param("sprintID"))
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid sprint id")
		return

	}

	ctx.Resp, ctx.RespErr = service.ListJiraSprintIssues(c.Param("id"), sprintID)
}

func SearchJiraIssues(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	ctx.Resp, ctx.RespErr = service.SearchJiraIssues(c.Param("id"), c.Query("project"), c.Query("type"), c.Query("status"), c.Query("summary"), c.Query("ne") == "true")
}

func SearchJiraProjectIssuesWithJQL(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	// 5.22 JQL only support {{.system.username}} variable
	// refactor if more variables are needed
	ctx.Resp, ctx.RespErr = service.SearchJiraProjectIssuesWithJQL(c.Param("id"), c.Query("project"), strings.ReplaceAll(c.Query("jql"), "{{.system.username}}", ctx.UserName), c.Query("summary"))
}

// @Summary Get Jira Types
// @Description Get Jira Types
// @Tags 	system
// @Accept 	json
// @Produce json
// @Param 	id 			path		string										true	"jira id"
// @Param 	project 	query		string										true	"jira project key"
// @Success 200 		{array} 	jira.IssueTypeWithStatus
// @Router /api/aslan/system/project_management/{id}/jira/type [get]
func GetJiraTypes(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	ctx.Resp, ctx.RespErr = service.GetJiraTypes(c.Param("id"), c.Query("project"))
}

// @Summary Get Jira Project Status
// @Description Get Jira Project Status
// @Tags 	system
// @Accept 	json
// @Produce json
// @Param 	id 			path		string										true	"jira id"
// @Param 	project 	query		string										true	"jira project id"
// @Success 200 		{array} 	string
// @Router /api/aslan/system/project_management/{id}/jira/status [get]
func GetJiraProjectStatus(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	ctx.Resp, ctx.RespErr = service.GetJiraProjectStatus(c.Param("id"), c.Query("project"))
}

// @Summary Get Jira All Status
// @Description Get Jira All Status
// @Tags 	system
// @Accept 	json
// @Produce json
// @Param 	id 			path		string										true	"jira id"
// @Success 200 		{array} 	jira.Status
// @Router /api/aslan/system/project_management/{id}/jira/allStatus [get]
func GetJiraAllStatus(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	ctx.Resp, ctx.RespErr = service.GetJiraAllStatus(c.Param("id"))
}

func HandleJiraEvent(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	event := new(jira.Event)
	if err := c.ShouldBindJSON(event); err != nil {
		ctx.RespErr = err
		return
	}

	ctx.RespErr = service.HandleJiraHookEvent(c.Param("workflowName"), c.Param("hookName"), event, ctx.Logger)
}

func HandleMeegoEvent(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	event := new(meego.GeneralWebhookRequest)
	if err := c.ShouldBindJSON(event); err != nil {
		ctx.RespErr = err
		return
	}

	ctx.RespErr = service.HandleMeegoHookEvent(c.Param("workflowName"), c.Param("hookName"), event, ctx.Logger)
}
