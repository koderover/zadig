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
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/sprint_management/service"
	"github.com/koderover/zadig/v2/pkg/setting"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/types"
)

// @Summary Get SprintWorkItem
// @Description Get SprintWorkItem
// @Tags 	SprintManagement
// @Accept 	json
// @Produce json
// @Param 	projectName		query		string							true	"project name"
// @Param 	id				path		string							true	"sprint workitem id"
// @Success 200 			{object} 	service.SprintWorkItem
// @Router /api/aslan/sprint_management/v1/sprint_workitem/{id} [get]
func GetSprintWorkItem(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	err = commonutil.CheckZadigEnterpriseLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	ctx.Resp, ctx.RespErr = service.GetSprintWorkItem(ctx, c.Param("id"))
}

// @Summary Create SprintWorkItem
// @Description Create SprintWorkItem
// @Tags 	SprintManagement
// @Accept 	json
// @Produce json
// @Param 	projectName		query		string							true	"project name"
// @Param 	body 			body 		models.SprintWorkItem 			true 	"body"
// @Success 200
// @Router /api/aslan/sprint_management/v1/sprint_workitem [post]
func CreateSprintWorkItem(c *gin.Context) {
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
			!ctx.Resources.ProjectAuthInfo[projectName].SprintWorkItem.Create {
			ctx.UnAuthorized = true
			return
		}
	}

	req := new(models.SprintWorkItem)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	err = commonutil.CheckZadigEnterpriseLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	ctx.RespErr = service.CreateSprintWorkItem(ctx, req)
}

type UpdateSprintWorkItemTitleRequest struct {
	Title string `json:"title"`
}

// @Summary Update SprintWorkItem Title
// @Description Update SprintWorkItem Title
// @Tags 	SprintManagement
// @Accept 	json
// @Produce json
// @Param 	projectName		query		string									true	"project name"
// @Param 	id				path		string									true	"sprint workitem id"
// @Param 	body 			body 		UpdateSprintWorkItemTitleRequest 		true 	"body"
// @Success 200
// @Router /api/aslan/sprint_management/v1/sprint_workitem/{id}/title [put]
func UpdateSprintWorkItemTitle(c *gin.Context) {
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
			!ctx.Resources.ProjectAuthInfo[projectName].SprintWorkItem.Edit {
			ctx.UnAuthorized = true
			return
		}
	}

	err = commonutil.CheckZadigEnterpriseLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	req := new(UpdateSprintWorkItemTitleRequest)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	ctx.RespErr = service.UpdateSprintWorkItemTitle(ctx, c.Param("id"), req.Title)
}

type UpdateSprintWorkItemDescRequest struct {
	Desc string `json:"des"`
}

// @Summary Update SprintWorkItem Description
// @Description Update SprintWorkItem Description
// @Tags 	SprintManagement
// @Accept 	json
// @Produce json
// @Param 	projectName		query		string								true	"project name"
// @Param 	id				path		string								true	"sprint workitem id"
// @Param 	body 			body 		UpdateSprintWorkItemDescRequest 	true 	"body"
// @Success 200
// @Router /api/aslan/sprint_management/v1/sprint_workitem/{id}/desc [put]
func UpdateSprintWorkItemDescription(c *gin.Context) {
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
			!ctx.Resources.ProjectAuthInfo[projectName].SprintWorkItem.Edit {
			ctx.UnAuthorized = true
			return
		}
	}

	err = commonutil.CheckZadigEnterpriseLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	req := new(UpdateSprintWorkItemDescRequest)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	ctx.RespErr = service.UpdateSprintWorkItemDescpition(ctx, c.Param("id"), req.Desc)
}

// @Summary Update SprintWorkItem Owner
// @Description Update SprintWorkItem Owner
// @Tags 	SprintManagement
// @Accept 	json
// @Produce json
// @Param 	projectName		query		string									true	"project name"
// @Param 	id				path		string									true	"sprint workitem id"
// @Param 	verb			query		service.UpdateSprintWorkItemOwnersVerb  true	"sprint workitem update verb"
// @Param 	body 			body 		[]types.UserBriefInfo 					true 	"body"
// @Success 200
// @Router /api/aslan/sprint_management/v1/sprint_workitem/{id}/owner [put]
func UpdateSprintWorkItemOwner(c *gin.Context) {
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
			!ctx.Resources.ProjectAuthInfo[projectName].SprintWorkItem.Edit {
			ctx.UnAuthorized = true
			return
		}
	}

	err = commonutil.CheckZadigEnterpriseLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	req := []types.UserBriefInfo{}
	if err := c.ShouldBindJSON(&req); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	ctx.RespErr = service.UpdateSprintWorkItemOwners(ctx, c.Param("id"), c.Query("verb"), req)
}

// @Summary Move SprintWorkItem
// @Description Move SprintWorkItem
// @Tags 	SprintManagement
// @Accept 	json
// @Produce json
// @Param 	projectName				query		string									true	"project name"
// @Param 	sprintID				query		string									true	"sprint id"
// @Param 	id						path		string									true	"sprint workitem id"
// @Param 	stageID					query		string									true	"sprint workitem stageID"
// @Param 	index					query		int										true	"sprint workitem index in stage"
// @Param 	sprintUpdateTime		query		int64									true	"sprint update time"
// @Param 	workitemUpdateTime		query		int64									true	"sprint workitem update time"
// @Success 200
// @Router /api/aslan/sprint_management/v1/sprint_workitem/{id}/move [put]
func MoveSprintWorkItem(c *gin.Context) {
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
			!ctx.Resources.ProjectAuthInfo[projectName].SprintWorkItem.Edit {
			ctx.UnAuthorized = true
			return
		}
	}

	index, err := strconv.Atoi(c.Query("index"))
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("index should be int")
		return
	}
	sprintUpdateTime, err := strconv.ParseInt(c.Query("sprintUpdateTime"), 10, 64)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("sprintUpdateTime should be int64")
		return
	}
	workitemUpdateTime, err := strconv.ParseInt(c.Query("workitemUpdateTime"), 10, 64)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("workitemUpdateTime should be int64")
		return
	}

	err = commonutil.CheckZadigEnterpriseLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	ctx.RespErr = service.MoveSprintWorkItem(ctx, c.Query("sprintID"), c.Param("id"), c.Query("stageID"), index, sprintUpdateTime, workitemUpdateTime)
}

// @Summary Delete SprintWorkItem
// @Description Delete SprintWorkItem
// @Tags 	SprintManagement
// @Accept 	json
// @Produce json
// @Param 	projectName		query		string							true	"project name"
// @Param 	id				path		string							true	"sprint workitem id"
// @Success 200
// @Router /api/aslan/sprint_management/v1/sprint_workitem/{id} [delete]
func DeleteSprintWorkItem(c *gin.Context) {
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
			!ctx.Resources.ProjectAuthInfo[projectName].SprintWorkItem.Delete {
			ctx.UnAuthorized = true
			return
		}
	}

	err = commonutil.CheckZadigEnterpriseLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	internalhandler.InsertDetailedOperationLog(c, ctx.UserName, projectName, setting.OperationSceneSprintManagement, "删除", "迭代管理-工作项", fmt.Sprintf("工作项ID: %s", c.Param("id")), "", types.RequestBodyTypeJSON, ctx.Logger, "")

	ctx.RespErr = service.DeleteSprintWorkItem(ctx, c.Param("id"))
}

// @Summary Exec SprintWorkItem Workflow
// @Description Exec SprintWorkItem Workflow
// @Tags 	SprintManagement
// @Accept 	json
// @Produce json
// @Param 	projectName		query		string									true	"project name"
// @Param 	id				path		string									true	"sprint workitem id"
// @Param 	ids				query		[]string								true	"sprint workitem ids"
// @Param 	workflowName	query		string									true	"workflow name"
// @Success 200
// @Router /api/aslan/sprint_management/v1/sprint_workitem/{id}/task/exec [post]
func ExecSprintWorkItemWorkflow(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectName := c.Query("projectName")
	workflowName := c.Query("workflowName")
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectName]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectName].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectName].Workflow.Execute {
			// check if the permission is given by collaboration mode
			permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, projectName, types.ResourceTypeWorkflow, workflowName, types.WorkflowActionRun)
			if err != nil || !permitted {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	err = commonutil.CheckZadigEnterpriseLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	workitemIDs, ok := c.GetQueryArray("ids")
	if !ok {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("ids should be array")
		return
	}

	args := new(commonmodels.WorkflowV4)
	err = c.BindJSON(args)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddErr(fmt.Errorf("get bind json error: %v", err))
		return
	}

	ctx.RespErr = service.ExecSprintWorkItemWorkflow(ctx, c.Param("id"), workitemIDs, workflowName, args)
}

// @Summary Clone SprintWorkItem Task
// @Description Clone SprintWorkItem Task
// @Tags 	SprintManagement
// @Accept 	json
// @Produce json
// @Param 	projectName		query		string									true	"project name"
// @Param 	id				path		string									true	"sprint workitem id"
// @Param 	workflowName	query		string									true	"workflow name"
// @Param 	taskID			query		string									true	"workitem task id"
// @Success 200
// @Router /api/aslan/sprint_management/v1/sprint_workitem/{id}/task/clone [post]
func CloneSprintWorkItemTask(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectName := c.Query("projectName")
	workflowName := c.Query("workflowName")
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectName]; !ok {
			ctx.UnAuthorized = true
			return
		}
		if !ctx.Resources.ProjectAuthInfo[projectName].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectName].Workflow.Execute {
			// check if the permission is given by collaboration mode
			permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, projectName, types.ResourceTypeWorkflow, workflowName, types.WorkflowActionRun)
			if err != nil || !permitted {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	err = commonutil.CheckZadigEnterpriseLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	ctx.Resp, ctx.RespErr = service.CloneSprintWorkItemTask(ctx, c.Query("taskID"))
}

// @Summary List Sprint WorkItem Task
// @Description List Sprint WorkItem Task
// @Tags 	SprintManagement
// @Accept 	json
// @Produce json
// @Param 	projectName		query		string							true	"project name"
// @Param 	id				path		string							true	"sprint workitem id"
// @Param 	workfloName		query		string							true	"workflow name"
// @Param 	pageNum 		query		int								true	"page num"
// @Param 	pageSize 		query		int								true	"page size"
// @Success 200 			{object} 	service.ListSprintWorkItemTaskResponse
// @Router /api/aslan/sprint_management/v1/sprint_workitem/{id}/task [get]
func ListSprintWorkItemTask(c *gin.Context) {
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

	opt := new(service.ListSprintWorkItemTaskRequset)
	if err := c.ShouldBindQuery(&opt); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	ctx.Resp, ctx.RespErr = service.ListSprintWorkItemTask(ctx, c.Param("id"), opt)
}
