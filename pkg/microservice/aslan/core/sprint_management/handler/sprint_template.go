/*
 * Copyright 2024 The KodeRover Authors.
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

	"github.com/gin-gonic/gin"
	"github.com/koderover/zadig/v2/pkg/types"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/sprint_management/service"
	"github.com/koderover/zadig/v2/pkg/setting"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

// @Summary Get Sprint Template
// @Description Get Sprint Template
// @Tags 	SprintManagement
// @Accept 	json
// @Produce json
// @Param 	projectName		query		string							true	"project name"
// @Param 	id				path		string							true	"sprint template id"
// @Success 200 			{object} 	models.SprintTemplate
// @Router /api/aslan/sprint_management/v1/sprint_template/{id} [get]
func GetSprintTemplate(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectName := c.Query("projectName")
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectName]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectName].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectName].Sprint.View {
			ctx.UnAuthorized = true
			return
		}
	}

	err = commonutil.CheckZadigEnterpriseLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	ctx.Resp, ctx.RespErr = service.GetSprintTemplate(ctx, c.Param("id"))
}

// @Summary Create Sprint Template
// @Description Create Sprint Template
// @Tags 	SprintManagement
// @Accept 	json
// @Produce json
// @Param 	projectName		query		string							true	"project name"
// @Param 	body 			body 		models.SprintTemplate 			true 	"body"
// @Success 200
// @Router /api/aslan/sprint_management/v1/sprint_template [post]
func CreateSprintTemplate(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectName := c.Query("projectName")
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectName]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectName].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectName].SprintTemplate.Edit {
			ctx.UnAuthorized = true
			return
		}
	}

	data, err := internalhandler.GetRawData(c)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}

	req := new(models.SprintTemplate)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, projectName, setting.OperationSceneSprintManagement, "创建", "迭代管理-流程", req.Name, string(data), types.RequestBodyTypeJSON, ctx.Logger, "")

	err = commonutil.CheckZadigEnterpriseLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	ctx.RespErr = service.CreateSprintTemplate(ctx, req)
}

// @Summary Delete Sprint Template
// @Description Delete Sprint Template
// @Tags 	SprintManagement
// @Accept 	json
// @Produce json
// @Param 	projectName		query		string							true	"project name"
// @Param 	id				path		string							true	"sprint template id"
// @Success 200
// @Router /api/aslan/sprint_management/v1/sprint_template/{id} [delete]
func DeleteSprintTemplate(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectName := c.Query("projectName")
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectName]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectName].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectName].SprintTemplate.Edit {
			ctx.UnAuthorized = true
			return
		}
	}

	err = commonutil.CheckZadigEnterpriseLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, projectName, setting.OperationSceneSprintManagement, "删除", "迭代管理-流程", "流程ID: "+c.Param("id"), "", types.RequestBodyTypeJSON, ctx.Logger, "")

	ctx.RespErr = service.DeleteSprintTemplate(ctx, ctx.UserName, c.Param("id"))
}

// @Summary List Sprint Templates
// @Description List Sprint Templates
// @Tags 	SprintManagement
// @Accept 	json
// @Produce json
// @Param 	projectName		query		string							true	"project name"
// @Param 	pageNum 		query		int								true	"page num"
// @Param 	pageSize 		query		int								true	"page size"
// @Success 200 			{object} 	service.ListSprintTemplateResp
// @Router /api/aslan/sprint_management/v1/sprint_template [get]
func ListSprintTemplates(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectName := c.Query("projectName")
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectName]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectName].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectName].Sprint.View {
			ctx.UnAuthorized = true
			return
		}
	}

	err = commonutil.CheckZadigEnterpriseLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	opt := new(service.ListSprintTemplateOption)
	if err := c.ShouldBindQuery(&opt); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	ctx.Resp, ctx.RespErr = service.ListSprintTemplate(ctx, opt)
}

// @Summary Get Default Sprint Templates
// @Description Get Default Sprint Templates
// @Tags 	SprintManagement
// @Accept 	json
// @Produce json
// @Param 	projectName		query		string							true	"project name"
// @Success 200 			{object} 	models.SprintTemplate
// @Router /api/aslan/sprint_management/v1/sprint_template/default [get]
func GetDefaultSprintTemplates(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectName := c.Query("projectName")
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectName]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectName].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectName].Sprint.View {
			ctx.UnAuthorized = true
			return
		}
	}

	err = commonutil.CheckZadigEnterpriseLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	ctx.Resp, ctx.RespErr = service.GetDefaultSprintTemplate(ctx, projectName)
}

// @Summary Add Sprint Template Stage
// @Description Add Sprint Template Stage
// @Tags 	SprintManagement
// @Accept 	json
// @Produce json
// @Param 	projectName		query		string							true	"project name"
// @Param 	id				path		string							true	"sprint template id"
// @Param 	stageName		query		string							true	"sprint template stage name"
// @Success 200
// @Router /api/aslan/sprint_management/v1/sprint_template/{id}/stage [post]
func AddSprintTemplateStage(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectName := c.Query("projectName")
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectName]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectName].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectName].SprintTemplate.Edit {
			ctx.UnAuthorized = true
			return
		}
	}

	err = commonutil.CheckZadigEnterpriseLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, projectName, setting.OperationSceneSprintManagement, "更新", "迭代管理-流程添加阶段", "阶段: "+c.Query("stageName"), "", types.RequestBodyTypeJSON, ctx.Logger, "")

	ctx.RespErr = service.AddSprintTemplateStage(ctx, c.Param("id"), c.Query("stageName"))
}

// @Summary Delete Sprint Template Stage
// @Description Delete Sprint Template Stage
// @Tags 	SprintManagement
// @Accept 	json
// @Produce json
// @Param 	projectName		query		string							true	"project name"
// @Param 	id				path		string							true	"sprint template id"
// @Param 	stageID			path		string							true	"sprint template stage id"
// @Success 200
// @Router /api/aslan/sprint_management/v1/sprint_template/{id}/stage/{stageID} [delete]
func DeleteSprintTemplateStage(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectName := c.Query("projectName")
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectName]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectName].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectName].SprintTemplate.Edit {
			ctx.UnAuthorized = true
			return
		}
	}

	err = commonutil.CheckZadigEnterpriseLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, projectName, setting.OperationSceneSprintManagement, "更新", "迭代管理-流程删除阶段", "阶段ID: "+c.Param("stageID"), "", types.RequestBodyTypeJSON, ctx.Logger, "")

	ctx.RespErr = service.DeleteSprintTemplateStage(ctx, c.Param("id"), c.Param("stageID"))
}

// @Summary Update Sprint Template Stage Name
// @Description Update Sprint Template Stage Name
// @Tags 	SprintManagement
// @Accept 	json
// @Produce json
// @Param 	projectName		query		string							true	"project name"
// @Param 	id				path		string							true	"sprint template id"
// @Param 	stageID			path		string							true	"sprint template stage id"
// @Param 	stageName		query		string							true	"sprint template stage name"
// @Param 	body 			body 		models.SprintTemplate 			true 	"body"
// @Success 200
// @Router /api/aslan/sprint_management/v1/sprint_template/{id}/stage/{stageID}/name [put]
func UpdateSprintTemplateStageName(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectName := c.Query("projectName")
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectName]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectName].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectName].SprintTemplate.Edit {
			ctx.UnAuthorized = true
			return
		}
	}

	err = commonutil.CheckZadigEnterpriseLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, projectName, setting.OperationSceneSprintManagement, "更新", "迭代管理-流程阶段名称", "阶段: "+c.Query("stageName"), "", types.RequestBodyTypeJSON, ctx.Logger, "")

	ctx.RespErr = service.UpdateSprintTemplateStageName(ctx, c.Param("id"), c.Param("stageID"), c.Query("stageName"))
}

type UpdateSprintTemplateStageWorkflowsRequest struct {
	Workflows  []*models.SprintWorkflow `json:"workflows"`
	UpdateTime int64                    `json:"update_time"`
}

// @Summary Update Sprint Template Stage Workflows
// @Description Update Sprint Template Stage Workflows
// @Tags 	SprintManagement
// @Accept 	json
// @Produce json
// @Param 	projectName		query		string										true	"project name"
// @Param 	id				path		string										true	"sprint template id"
// @Param 	stageID			path		string										true	"sprint template stage id"
// @Param 	body 			body 		UpdateSprintTemplateStageWorkflowsRequest 	true 	"body"
// @Success 200
// @Router /api/aslan/sprint_management/v1/sprint_template/{id}/stage/{stageID}/workflows [put]
func UpdateSprintTemplateStageWorkflows(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectName := c.Query("projectName")
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectName]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectName].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectName].SprintTemplate.Edit {
			ctx.UnAuthorized = true
			return
		}
	}

	err = commonutil.CheckZadigEnterpriseLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	data, err := internalhandler.GetRawData(c)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}

	req := new(UpdateSprintTemplateStageWorkflowsRequest)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, projectName, setting.OperationSceneSprintManagement, "更新", "迭代管理-流程阶段工作流", "阶段ID: "+c.Param("stageID"), string(data), types.RequestBodyTypeJSON, ctx.Logger, "")

	ctx.RespErr = service.UpdateSprintTemplateStageWorkflows(ctx, c.Param("id"), c.Param("stageID"), req.Workflows, req.UpdateTime)
}
