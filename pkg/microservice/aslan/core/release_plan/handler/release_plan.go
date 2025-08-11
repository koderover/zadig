/*
 * Copyright 2023 The KodeRover Authors.
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

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/release_plan/service"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

func GetReleasePlan(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	if !ctx.Resources.IsSystemAdmin && !ctx.Resources.SystemActions.ReleasePlan.View {
		ctx.UnAuthorized = true
		return
	}

	err = commonutil.CheckZadigEnterpriseLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	ctx.Resp, ctx.RespErr = service.GetReleasePlan(c.Param("id"))
}

func GetReleasePlanLogs(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.Logger.Errorf("failed to generate authorization info for user: %s, error: %s", ctx.UserID, err)
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	if !ctx.Resources.IsSystemAdmin && !ctx.Resources.SystemActions.ReleasePlan.View {
		ctx.UnAuthorized = true
		return
	}

	err = commonutil.CheckZadigEnterpriseLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	ctx.Resp, ctx.RespErr = service.GetReleasePlanLogs(c.Param("id"))
}

func CreateReleasePlan(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	if !ctx.Resources.IsSystemAdmin && !ctx.Resources.SystemActions.ReleasePlan.Create {
		ctx.UnAuthorized = true
		return
	}

	req := new(models.ReleasePlan)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	err = commonutil.CheckZadigEnterpriseLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	ctx.RespErr = service.CreateReleasePlan(ctx, req)
}

func UpdateReleasePlan(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	if !ctx.Resources.IsSystemAdmin && !ctx.Resources.SystemActions.ReleasePlan.Edit {
		ctx.UnAuthorized = true
		return
	}

	err = commonutil.CheckZadigEnterpriseLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	req := new(service.UpdateReleasePlanArgs)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	ctx.RespErr = service.UpdateReleasePlan(ctx, c.Param("id"), req)
}

func GetReleasePlanJobDetail(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	if !ctx.Resources.IsSystemAdmin && !ctx.Resources.SystemActions.ReleasePlan.View {
		ctx.UnAuthorized = true
		return
	}

	err = commonutil.CheckZadigEnterpriseLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	ctx.Resp, ctx.RespErr = service.GetReleasePlanJobDetail(c.Param("id"), c.Param("jobID"))
}

func DeleteReleasePlan(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	if !ctx.Resources.IsSystemAdmin && !ctx.Resources.SystemActions.ReleasePlan.Delete {
		ctx.UnAuthorized = true
		return
	}

	err = commonutil.CheckZadigEnterpriseLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	ctx.RespErr = service.DeleteReleasePlan(c, ctx.UserName, c.Param("id"))
}

func ExecuteReleaseJob(c *gin.Context) {
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

	req := new(service.ExecuteReleaseJobArgs)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	// only release plan manager can execute release job
	// so no need to check authorization there
	ctx.RespErr = service.ExecuteReleaseJob(ctx, c.Param("id"), req, ctx.Resources.IsSystemAdmin)
}

func ScheduleExecuteReleasePlan(c *gin.Context) {
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

	ctx.RespErr = service.ScheduleExecuteReleasePlan(ctx, c.Param("id"), c.Query("jobID"))
}

func SkipReleaseJob(c *gin.Context) {
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

	req := new(service.SkipReleaseJobArgs)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	// only release plan manager can skip release job
	// so no need to check authorization there
	ctx.RespErr = service.SkipReleaseJob(ctx, c.Param("id"), req, ctx.Resources.IsSystemAdmin)
}

func UpdateReleaseJobStatus(c *gin.Context) {
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

	// only release plan manager can execute release job
	// so no need to check authorization there
	ctx.RespErr = service.UpdateReleasePlanStatus(ctx, c.Param("id"), c.Param("status"), ctx.Resources.IsSystemAdmin)
}

func ApproveReleasePlan(c *gin.Context) {
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

	req := new(service.ApproveRequest)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	ctx.RespErr = service.ApproveReleasePlan(ctx, c.Param("id"), req)
}

// @Summary List Release Plans
// @Description List Release Plans
// @Tags 	releasePlan
// @Accept 	json
// @Produce json
// @Param 	pageNum 		query		int								true	"page num"
// @Param 	pageSize 		query		int								true	"page size"
// @Param 	type 			query		service.ListReleasePlanType 	true	"search type"
// @Param 	keyword 		query		string 							true	"search keyword, 当类型为success_time时，值应为'开始时间戳-结束时间戳'的形式"
// @Success 200 			{object} 	service.ListReleasePlanResp
// @Router /api/aslan/release_plan/v1 [get]
func ListReleasePlans(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	if !ctx.Resources.IsSystemAdmin && !ctx.Resources.SystemActions.ReleasePlan.View {
		ctx.UnAuthorized = true
		return
	}

	err = commonutil.CheckZadigEnterpriseLicense()
	if err != nil {
		ctx.RespErr = err
		return
	}

	opt := new(service.ListReleasePlanOption)
	if err := c.ShouldBindQuery(&opt); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	ctx.Resp, ctx.RespErr = service.ListReleasePlans(opt)
}

// @Summary Get Release Plan Hook Setting
// @Description Get Release Plan Hook Setting
// @Tags 	releasePlan
// @Accept 	json
// @Produce json
// @Success 200 {object} models.ReleasePlanHookSettings
// @Router /api/aslan/release_plan/v1/hook/setting [get]
func GetReleasePlanHookSetting(c *gin.Context) {
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

	ctx.Resp, ctx.RespErr = service.GetReleasePlanHookSetting(ctx)
}

// @Summary Update Release Plan Hook Setting
// @Description Update Release Plan Hook Setting
// @Tags 	releasePlan
// @Accept 	json
// @Produce json
// @Param 	body 	body		models.ReleasePlanHookSettings 	        true	"release plan hook setting"
// @Success 200
// @Router /api/aslan/release_plan/v1/hook/setting [put]
func UpdateReleasePlanHookSetting(c *gin.Context) {
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

	if !ctx.Resources.IsSystemAdmin && !ctx.Resources.SystemActions.ReleasePlan.EditConfig {
		ctx.UnAuthorized = true
		return
	}

	req := new(models.ReleasePlanHookSettings)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	ctx.RespErr = service.UpdateReleasePlanHookSetting(ctx, req)
}

// @Summary Release Plan Hook Callback
// @Description Release Plan Hook Callback
// @Tags 	releasePlan
// @Accept 	json
// @Produce json
// @Param 	body 	body		service.ReleasePlanCallBackBody 	        true	"release plan hook callback"
// @Success 200
// @Router /api//callback/release_plan [post]
func ReleasePlanHookCallback(c *gin.Context) {
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

	req := new(service.ReleasePlanCallBackBody)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	ctx.RespErr = service.ReleasePlanHookCallback(ctx, req)
}

// @Summary Release Plan Swag Placeholder
// @Description Release Plan Swag Placeholder
// @Tags 	releasePlan
// @Accept 	json
// @Produce json
// @Param 	build 			body		webhooknotify.OpenAPIWorkflowBuildJobSpec 	true	"build job"
// @Param 	deploy 			body		webhooknotify.OpenAPIWorkflowDeployJobSpec 	true	"deploy job"
// @Param 	freestyle 		body		webhooknotify.OpenAPIWorkflowFreestyleJobSpec 	true	"freestyle job"
// @Param 	vm_deploy 		body		webhooknotify.OpenAPIWorkflowServiceAndVMDeploy 	true	"vm deploy job"
// @Param 	testing 		body		webhooknotify.OpenAPIWorkflowTestingJobSpec 	true	"testing job"
// @Param 	scanning 		body		webhooknotify.OpenAPIWorkflowScanningJobSpec 	true	"scanning job"
// @Param 	apollo 			body		webhooknotify.OpenAPIWorkflowApolloJobSpec 	true	"apollo job"
// @Param 	nacos 			body		webhooknotify.OpenAPIWorkflowNacosJobSpec 	true	"nacos job"
// @Param 	sql 			body		webhooknotify.OpenAPIWorkflowSQLJobSpec 	true	"sql job"
// @Param 	distribute 		body		webhooknotify.OpenAPIWorkflowDistributeImageJobSpec 	true	"distribute image job"
// @Success 200
// @Router /api/aslan/release_plan/v1/swag/placeholder [get]
func ReleasePlanSwagPlaceholder(c *gin.Context) {
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

	ctx.Resp, ctx.RespErr = service.GetReleasePlanHookSetting(ctx)
}
