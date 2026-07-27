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

	"github.com/gin-gonic/gin"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/system/service"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/llm"
)

type AgentIntegrationRequest struct {
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

func CreateAgentIntegration(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	request := new(AgentIntegrationRequest)
	if err := c.ShouldBindJSON(request); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid create agent integration json args")
		return
	}
	integration := convertAgentIntegrationRequest(request)
	integration.UpdatedBy = ctx.UserName
	ctx.RespErr = service.CreateAgentIntegration(context.TODO(), integration)
}

func ValidateAgentIntegration(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	request := new(AgentIntegrationRequest)
	if err := c.ShouldBindJSON(request); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid validate agent integration json args")
		return
	}
	ctx.RespErr = service.ValidateAgentIntegration(context.TODO(), convertAgentIntegrationRequest(request))
}

func GetAgentIntegration(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	if c.Param("id") == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid agent integration id")
		return
	}
	ctx.Resp, ctx.RespErr = service.GetAgentIntegration(context.TODO(), c.Param("id"))
}

func ListAgentIntegrations(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	ctx.Resp, ctx.RespErr = service.ListAgentIntegrations(context.TODO())
}

func UpdateAgentIntegration(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
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

func DeleteAgentIntegration(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
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
