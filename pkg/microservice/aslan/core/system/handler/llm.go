/*
Copyright 2023 The KodeRover Authors.

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

type CreateLLMIntegrationRequest struct {
	ProviderName llm.Provider `json:"provider_name"`
	Token        string       `json:"token"`
	BaseURL      string       `json:"base_url"`
	Model        string       `json:"model"`
	EnableProxy  bool         `json:"enable_proxy"`
}

// @Summary Create a llm integration
// @Description Create a llm integration
// @Tags 	system
// @Accept 	json
// @Produce json
// @Param 	body 			body 		CreateLLMIntegrationRequest 			true 	"body"
// @Success 200
// @Router /api/aslan/system/llm/integration [post]
func CreateLLMIntegration(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(CreateLLMIntegrationRequest)
	if err := c.BindJSON(args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid create llm Integration json args")
		return
	}

	llmProvider := convertLLMArgToModel(args)
	llmProvider.UpdatedBy = ctx.UserName
	ctx.RespErr = service.CreateLLMIntegration(context.TODO(), llmProvider)
}

type GetLLMIntegrationRespone struct {
	Name    string `json:"name"`
	Token   string `json:"token"`
	BaseURL string `json:"base_url"`
}

// @Summary Get a llm integration
// @Description Get a llm integration
// @Tags 	system
// @Accept 	json
// @Produce json
// @Param 	id			path		string								true	"id"
// @Success 200 		{object} 	commonmodels.LLMIntegration
// @Router /api/aslan/system/llm/integration/{id} [get]
func GetLLMIntegration(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	id := c.Param("id")
	if len(id) == 0 {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid llm id")
		return
	}

	ctx.Resp, ctx.RespErr = service.GetLLMIntegration(context.TODO(), id)
}

// @Summary List llm integrations
// @Description List llm integrations
// @Tags 	system
// @Accept 	json
// @Produce json
// @Success 200 		{array} 	commonmodels.LLMIntegration
// @Router /api/aslan/system/llm/integration [get]
func ListLLMIntegration(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.RespErr = service.ListLLMIntegration(context.TODO())
}

type checkLLMIntegrationResponse struct {
	Check bool `json:"check"`
}

// @Summary Check llm integrations
// @Description Check llm integrations
// @Tags 	system
// @Accept 	json
// @Produce json
// @Success 200 		{object} 		checkLLMIntegrationResponse
// @Router /api/aslan/system/llm/integration/check [get]
func CheckLLMIntegration(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	resp := &checkLLMIntegrationResponse{}
	resp.Check, ctx.RespErr = service.CheckLLMIntegration(context.TODO())
	ctx.Resp = resp
}

// @Summary Update a llm integration
// @Description Update a llm integration
// @Tags 	system
// @Accept 	json
// @Produce json
// @Param 	id				path		string							true	"id"
// @Param 	body 			body 		CreateLLMIntegrationRequest 	true 	"body"
// @Success 200
// @Router /api/aslan/system/llm/integration/{id} [put]
func UpdateLLMIntegration(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	id := c.Param("id")
	if len(id) == 0 {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid llm id")
		return
	}

	args := new(CreateLLMIntegrationRequest)
	if err := c.BindJSON(args); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid update llm integration json args")
		return
	}

	llmProvider := convertLLMArgToModel(args)
	llmProvider.UpdatedBy = ctx.UserName
	ctx.RespErr = service.UpdateLLMIntegration(context.TODO(), c.Param("id"), llmProvider)
}

// @Summary Delete a llm integration
// @Description Delete a llm integration
// @Tags 	system
// @Accept 	json
// @Produce json
// @Param 	id				path		string							true	"id"
// @Success 200
// @Router /api/aslan/system/llm/integration/{id} [delete]
func DeleteLLMIntegration(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	id := c.Param("id")
	if len(id) == 0 {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid llm id")
		return
	}

	ctx.RespErr = service.DeleteLLMIntegration(context.TODO(), id)
}

func convertLLMArgToModel(args *CreateLLMIntegrationRequest) *commonmodels.LLMIntegration {
	return &commonmodels.LLMIntegration{
		ProviderName: args.ProviderName,
		Token:        args.Token,
		BaseURL:      args.BaseURL,
		EnableProxy:  args.EnableProxy,
		Model:        args.Model,
		IsDefault:    true,
	}
}
