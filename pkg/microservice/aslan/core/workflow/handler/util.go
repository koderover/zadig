/*
Copyright 2024 The KodeRover Authors.

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
	"github.com/gin-gonic/gin"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/workflow/service/workflow"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	"github.com/koderover/zadig/v2/pkg/tool/errors"
)

type RenderWorkflowVariableReq struct {
	VariableType string       `json:"type"`
	WorkflowName string       `json:"workflow_name"`
	ProjectKey   string       `json:"project_key"`
	JobName      string       `json:"job_name"`
	VariableKey  string       `json:"variable_key"`
	Input        []*models.KV `json:"input"`
}

func RenderWorkflowVariables(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	req := new(RenderWorkflowVariableReq)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.Err = errors.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	ctx.Resp, ctx.Err = workflow.RenderWorkflowVariables(req.ProjectKey, req.WorkflowName, req.VariableType, req.JobName, req.VariableKey, req.Input, ctx.Logger)
}
