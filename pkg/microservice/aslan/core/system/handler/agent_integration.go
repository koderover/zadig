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

// @Summary Create a agent integration
// @Description Create a agent integration
// @Tags 	system
// @Accept 	json
// @Produce json
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

	request := new(AgentIntegrationRequest)
	if err := c.ShouldBindJSON(request); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid create agent integration json args")
		return
	}
	integration := convertAgentIntegrationRequest(request)
	integration.UpdatedBy = ctx.UserName
	ctx.RespErr = service.CreateAgentIntegration(context.TODO(), integration)
}

// @Summary 验证 Agent 集成
// @Description
// @Tags 	system
// @Accept 	json
// @Produce json
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

	request := new(AgentIntegrationRequest)
	if err := c.ShouldBindJSON(request); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid validate agent integration json args")
		return
	}
	ctx.RespErr = service.ValidateAgentIntegration(context.TODO(), request.ID, convertAgentIntegrationRequest(request))
}

// @Summary Get a agent integration
// @Description Get a agent integration
// @Tags 	system
// @Accept 	json
// @Produce json
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

	if c.Param("id") == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid agent integration id")
		return
	}
	// encrypted credential echo is for the admin edit dialog only; everyone else
	// always gets masked values.
	encryptedKey := c.Query("encryptedKey")
	if !ctx.Resources.IsSystemAdmin {
		encryptedKey = ""
	}
	ctx.Resp, ctx.RespErr = service.GetAgentIntegration(context.TODO(), c.Param("id"), encryptedKey)
}

// @Summary List agent integrations
// @Description List agent integrations
// @Tags 	system
// @Accept 	json
// @Produce json
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

	encryptedKey := c.Query("encryptedKey")
	if !ctx.Resources.IsSystemAdmin {
		encryptedKey = ""
	}
	ctx.Resp, ctx.RespErr = service.ListAgentIntegrations(context.TODO(), encryptedKey)
}

// @Summary Update a agent integration
// @Description Update a agent integration
// @Tags 	system
// @Accept 	json
// @Produce json
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

	if c.Param("id") == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid agent integration id")
		return
	}
	request := new(AgentIntegrationRequest)
	if err := c.ShouldBindJSON(request); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid update agent integration json args")
		return
	}
	integration := convertAgentIntegrationRequest(request)
	integration.UpdatedBy = ctx.UserName
	ctx.RespErr = service.UpdateAgentIntegration(context.TODO(), c.Param("id"), integration)
}

// @Summary Delete a agent integration
// @Description Delete a agent integration
// @Tags 	system
// @Accept 	json
// @Produce json
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

	if c.Param("id") == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid agent integration id")
		return
	}
	ctx.RespErr = service.DeleteAgentIntegration(context.TODO(), c.Param("id"))
}

func convertAgentIntegrationRequest(request *AgentIntegrationRequest) *commonmodels.AgentIntegration {
	return &commonmodels.AgentIntegration{
		Name: request.Name, Description: request.Description, BaseURL: request.BaseURL,
		Protocol: request.Protocol, Model: request.Model, AuthType: request.AuthType,
		APIKey: request.APIKey, AccessKey: request.AccessKey, SecretKey: request.SecretKey,
	}
}
