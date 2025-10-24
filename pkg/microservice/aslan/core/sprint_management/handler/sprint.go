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

	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/sprint_management/service"
	"github.com/koderover/zadig/v2/pkg/setting"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

// @Summary Get Sprint
// @Description Get Sprint
// @Tags 	SprintManagement
// @Accept 	json
// @Produce json
// @Param 	projectName		query		string							true	"project name"
// @Param 	id				path		string							true	"sprint id"
// @Success 200 			{object} 	service.Sprint
// @Router /api/aslan/sprint_management/v1/sprint/{id} [get]
func GetSprint(c *gin.Context) {
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

	ctx.Resp, ctx.RespErr = service.GetSprint(ctx, c.Param("id"))
}

// @Summary Create Sprint
// @Description Create Sprint
// @Tags 	SprintManagement
// @Accept 	json
// @Produce json
// @Param 	name			query		string							true	"sprint name"
// @Param 	projectName		query		string							true	"project name"
// @Param 	templateID		query		string							true	"sprint template ID"
// @Success 200 			{object}  	service.CreateSprintResponse
// @Router /api/aslan/sprint_management/v1/sprint [post]
func CreateSprint(c *gin.Context) {
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
			!ctx.Resources.ProjectAuthInfo[projectName].Sprint.Create {
			ctx.UnAuthorized = true
			return
		}
	}

	err = commonutil.CheckZadigEnterpriseLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	detail := fmt.Sprintf("流程ID: %s, 迭代名称: %s", c.Query("templateID"), c.Query("name"))
	detailEn := fmt.Sprintf("Sprint Template ID: %s, Sprint Name: %s", c.Query("templateID"), c.Query("name"))
	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, projectName, setting.OperationSceneSprintManagement, "创建", "迭代管理-迭代", detail, detailEn, "", types.RequestBodyTypeJSON, ctx.Logger, "")

	ctx.Resp, ctx.RespErr = service.CreateSprint(ctx, projectName, c.Query("templateID"), c.Query("name"))
}

// @Summary Update Sprint Name
// @Description Update Sprint Name
// @Tags 	SprintManagement
// @Accept 	json
// @Produce json
// @Param 	projectName		query		string							true	"project name"
// @Param 	name			query		string							true	"sprint name"
// @Param 	id				path		string							true	"sprint id"
// @Success 200
// @Router /api/aslan/sprint_management/v1/sprint/{id}/name [put]
func UpdateSprintName(c *gin.Context) {
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
			!ctx.Resources.ProjectAuthInfo[projectName].Sprint.Edit {
			ctx.UnAuthorized = true
			return
		}
	}

	err = commonutil.CheckZadigEnterpriseLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	detail := fmt.Sprintf("迭代ID: %s, 迭代名称: %s", c.Param("id"), c.Query("name"))
	detailEn := fmt.Sprintf("Sprint ID: %s, Sprint Name: %s", c.Param("id"), c.Query("name"))
	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, projectName, setting.OperationSceneSprintManagement, "更新", "迭代管理-迭代", detail, detailEn, "", types.RequestBodyTypeJSON, ctx.Logger, "")

	ctx.RespErr = service.UpdateSprintName(ctx, c.Param("id"), c.Query("name"))
}

// @Summary Delete Sprint
// @Description Delete Sprint
// @Tags 	SprintManagement
// @Accept 	json
// @Produce json
// @Param 	projectName		query		string							true	"project name"
// @Param 	id				path		string							true	"sprint id"
// @Success 200
// @Router /api/aslan/sprint_management/v1/sprint/{id} [delete]
func DeleteSprint(c *gin.Context) {
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
			!ctx.Resources.ProjectAuthInfo[projectName].Sprint.Delete {
			ctx.UnAuthorized = true
			return
		}
	}

	err = commonutil.CheckZadigEnterpriseLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	detail := fmt.Sprintf("迭代ID: %s", c.Param("id"))
	detailEn := fmt.Sprintf("Sprint ID: %s", c.Param("id"))
	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, projectName, setting.OperationSceneSprintManagement, "删除", "迭代管理-迭代", detail, detailEn, "", types.RequestBodyTypeJSON, ctx.Logger, "")

	ctx.RespErr = service.DeleteSprint(ctx, c.Param("id"))
}

// @Summary Archive Sprint
// @Description Archive Sprint
// @Tags 	SprintManagement
// @Accept 	json
// @Produce json
// @Param 	projectName		query		string							true	"project name"
// @Param 	id				path		string							true	"sprint id"
// @Success 200
// @Router /api/aslan/sprint_management/v1/sprint/{id}/archive [put]
func ArchiveSprint(c *gin.Context) {
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
			!ctx.Resources.ProjectAuthInfo[projectName].Sprint.Edit {
			ctx.UnAuthorized = true
			return
		}
	}

	err = commonutil.CheckZadigEnterpriseLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	detail := fmt.Sprintf("归档迭代ID: %s", c.Param("id"))
	detailEn := fmt.Sprintf("Archive Sprint ID: %s", c.Param("id"))
	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, projectName, setting.OperationSceneSprintManagement, "更新", "迭代管理-迭代", detail, detailEn, "", types.RequestBodyTypeJSON, ctx.Logger, "")

	ctx.RespErr = service.ArchiveSprint(ctx, c.Param("id"))
}

// @Summary Activate Archived Sprint
// @Description Activate Archived Sprint
// @Tags 	SprintManagement
// @Accept 	json
// @Produce json
// @Param 	projectName		query		string							true	"project name"
// @Param 	id				path		string							true	"sprint id"
// @Success 200
// @Router /api/aslan/sprint_management/v1/sprint/{id}/activate [put]
func ActivateArchivedSprint(c *gin.Context) {
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
			!ctx.Resources.ProjectAuthInfo[projectName].Sprint.Edit {
			ctx.UnAuthorized = true
			return
		}
	}

	err = commonutil.CheckZadigEnterpriseLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	detail := fmt.Sprintf("激活已归档迭代ID: %s", c.Param("id"))
	detailEn := fmt.Sprintf("Activate Archived Sprint ID: %s", c.Param("id"))
	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, projectName, setting.OperationSceneSprintManagement, "更新", "迭代管理-迭代", detail, detailEn, "", types.RequestBodyTypeJSON, ctx.Logger, "")

	ctx.RespErr = service.ActivateArchivedSprint(ctx, c.Param("id"))
}

// @Summary List Sprint
// @Description List Sprint
// @Tags 	SprintManagement
// @Accept 	json
// @Produce json
// @Param 	projectName		query		string							true	"project name"
// @Param 	pageNum 		query		int								true	"page num"
// @Param 	pageSize 		query		int								true	"page size"
// @Param 	isArchived 		query		bool							true	"is archived"
// @Param 	filter		 	query		string							false	"search filter"
// @Success 200 			{object} 	service.ListSprintResp
// @Router /api/aslan/sprint_management/v1/sprint [get]
func ListSprint(c *gin.Context) {
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

	opt := new(service.ListSprintOption)
	if err := c.ShouldBindQuery(&opt); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	ctx.Resp, ctx.RespErr = service.ListSprint(ctx, opt)
}
