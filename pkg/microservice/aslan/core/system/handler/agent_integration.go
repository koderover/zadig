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
	"context"
	"fmt"

	"github.com/gin-gonic/gin"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/system/service"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/llm"
)

type AgentIntegrationRequest struct {
	ID          string                     `json:"id"`
	Name        string                     `json:"name"`
	Description string                     `json:"description"`
	BaseURL     string                     `json:"base_url"`
	Protocol    llm.Protocol               `json:"protocol"`
	Model       string                     `json:"model"`
	AuthType    commonmodels.AgentAuthType `json:"auth_type"`
	APIKey      string                     `json:"api_key"`
	AccessKey   string                     `json:"access_key"`
	SecretKey   string                     `json:"secret_key"`
}

// checkAgentIntegrationPermission requires the user to be a system admin or a
// member of the project that owns the agent integrations.
func checkAgentIntegrationPermission(ctx *internalhandler.Context, projectName string) bool {
	if !requireLogin(ctx) {
		return false
	}
	if projectName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("projectName is required")
		return false
	}
	if ctx.Resources.IsSystemAdmin {
		return true
	}
	if _, ok := ctx.Resources.ProjectAuthInfo[projectName]; !ok {
		ctx.UnAuthorized = true
		return false
	}
	return true
}

// canEchoAgentCredential decides whether encrypted credential echo is allowed;
// it is for the edit dialog only, so system admins and project admins qualify.
func canEchoAgentCredential(ctx *internalhandler.Context, projectName string) bool {
	if ctx.Resources.IsSystemAdmin {
		return true
	}
	if authInfo, ok := ctx.Resources.ProjectAuthInfo[projectName]; ok {
		return authInfo.IsProjectAdmin
	}
	return false
}

// @Summary Create a agent integration
// @Description Create a agent integration
// @Tags 	system
// @Accept 	json
// @Produce json
// @Param 	projectName		query		string								true	"project name"
// @Param 	body 			body 		AgentIntegrationRequest 			true 	"body"
// @Success 200
// @Router /api/aslan/system/agent/integration [post]
func CreateAgentIntegration(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectName := c.Query("projectName")
	if !checkAgentIntegrationPermission(ctx, projectName) {
		return
	}

	request := new(AgentIntegrationRequest)
	if err := c.ShouldBindJSON(request); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid create agent integration json args")
		return
	}
	integration := convertAgentIntegrationRequest(projectName, request)
	integration.UpdatedBy = ctx.UserName
	ctx.RespErr = service.CreateAgentIntegration(context.TODO(), integration)
}

// @Summary 验证 Agent 集成
// @Description
// @Tags 	system
// @Accept 	json
// @Produce json
// @Param 	projectName		query		string								true	"project name"
// @Param 	body 			body 		AgentIntegrationRequest 			true 	"body"
// @Success 200
// @Router /api/aslan/system/agent/integration/validate [post]
func ValidateAgentIntegration(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectName := c.Query("projectName")
	if !checkAgentIntegrationPermission(ctx, projectName) {
		return
	}

	request := new(AgentIntegrationRequest)
	if err := c.ShouldBindJSON(request); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid validate agent integration json args")
		return
	}
	ctx.RespErr = service.ValidateAgentIntegration(context.TODO(), projectName, request.ID, convertAgentIntegrationRequest(projectName, request))
}

// @Summary Get a agent integration
// @Description Get a agent integration
// @Tags 	system
// @Accept 	json
// @Produce json
// @Param 	projectName		query		string								true	"project name"
// @Param 	id				path		string								true	"id"
// @Param 	encryptedKey	query		string								false	"encrypted key"
// @Success 200 		{object} 	commonmodels.AgentIntegration
// @Router /api/aslan/system/agent/integration/{id} [get]
func GetAgentIntegration(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectName := c.Query("projectName")
	if !checkAgentIntegrationPermission(ctx, projectName) {
		return
	}

	if c.Param("id") == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid agent integration id")
		return
	}
	encryptedKey := c.Query("encryptedKey")
	if !canEchoAgentCredential(ctx, projectName) {
		encryptedKey = ""
	}
	ctx.Resp, ctx.RespErr = service.GetAgentIntegration(context.TODO(), projectName, c.Param("id"), encryptedKey)
}

// @Summary List agent integrations
// @Description List agent integrations
// @Tags 	system
// @Accept 	json
// @Produce json
// @Param 	projectName		query		string								true	"project name"
// @Param 	encryptedKey	query		string								false	"encrypted key"
// @Success 200 		{array} 	commonmodels.AgentIntegration
// @Router /api/aslan/system/agent/integration [get]
func ListAgentIntegrations(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectName := c.Query("projectName")
	if !checkAgentIntegrationPermission(ctx, projectName) {
		return
	}

	encryptedKey := c.Query("encryptedKey")
	if !canEchoAgentCredential(ctx, projectName) {
		encryptedKey = ""
	}
	ctx.Resp, ctx.RespErr = service.ListAgentIntegrations(context.TODO(), projectName, encryptedKey)
}

// @Summary List agents of the projects visible to the caller
// @Description List agents for the workflow AI task selector
// @Tags 	system
// @Accept 	json
// @Produce json
// @Success 200 		{array} 	service.AgentIntegrationBrief
// @Router /api/aslan/system/agent/integrations [get]
func ListAllAgentIntegrations(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	if !requireLogin(ctx) {
		return
	}

	// the response carries no endpoint or credential, but the project and agent
	// names themselves are scoped: only list the projects the caller belongs to.
	var visibleProjects []string
	if !ctx.Resources.IsSystemAdmin {
		visibleProjects = make([]string, 0, len(ctx.Resources.ProjectAuthInfo))
		for projectName := range ctx.Resources.ProjectAuthInfo {
			visibleProjects = append(visibleProjects, projectName)
		}
	}
	ctx.Resp, ctx.RespErr = service.ListAgentIntegrationBriefs(context.TODO(), ctx.Resources.IsSystemAdmin, visibleProjects)
}

// @Summary Update a agent integration
// @Description Update a agent integration
// @Tags 	system
// @Accept 	json
// @Produce json
// @Param 	projectName		query		string								true	"project name"
// @Param 	id				path		string								true	"id"
// @Param 	body 			body 		AgentIntegrationRequest 			true 	"body"
// @Success 200
// @Router /api/aslan/system/agent/integration/{id} [put]
func UpdateAgentIntegration(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectName := c.Query("projectName")
	if !checkAgentIntegrationPermission(ctx, projectName) {
		return
	}

	if c.Param("id") == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid agent integration id")
		return
	}
	request := new(AgentIntegrationRequest)
	if err := c.ShouldBindJSON(request); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid update agent integration json args")
		return
	}
	integration := convertAgentIntegrationRequest(projectName, request)
	integration.UpdatedBy = ctx.UserName
	ctx.RespErr = service.UpdateAgentIntegration(context.TODO(), projectName, c.Param("id"), integration)
}

// @Summary Delete a agent integration
// @Description Delete a agent integration
// @Tags 	system
// @Accept 	json
// @Produce json
// @Param 	projectName		query		string								true	"project name"
// @Param 	id			path		string								true	"id"
// @Success 200
// @Router /api/aslan/system/agent/integration/{id} [delete]
func DeleteAgentIntegration(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectName := c.Query("projectName")
	if !checkAgentIntegrationPermission(ctx, projectName) {
		return
	}

	if c.Param("id") == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid agent integration id")
		return
	}
	ctx.RespErr = service.DeleteAgentIntegration(context.TODO(), projectName, c.Param("id"))
}

func convertAgentIntegrationRequest(projectName string, request *AgentIntegrationRequest) *commonmodels.AgentIntegration {
	return &commonmodels.AgentIntegration{
		ProjectName: projectName,
		Name:        request.Name, Description: request.Description, BaseURL: request.BaseURL,
		Protocol: request.Protocol, Model: request.Model, AuthType: request.AuthType,
		APIKey: request.APIKey, AccessKey: request.AccessKey, SecretKey: request.SecretKey,
	}
}
