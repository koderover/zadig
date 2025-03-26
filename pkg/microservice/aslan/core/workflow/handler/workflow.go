/*
Copyright 2021 The KodeRover Authors.

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
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/workflow/service/workflow"
	"github.com/koderover/zadig/v2/pkg/setting"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/types"
)

func GetWorkflowProductName(c *gin.Context) {
	args := new(commonmodels.Workflow)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("c.GetRawData() err : %v", err)
		return
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("json.Unmarshal err : %v", err)
		return
	}
	c.Set("productName", args.ProductTmplName)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))
	c.Next()
}

func GetProductNameByWorkflow(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	workflow, err := workflow.FindWorkflow(c.Param("name"), ctx.Logger)
	if err != nil {
		log.Errorf("FindWorkflow err : %v", err)
		return
	}
	c.Set("productName", workflow.ProductTmplName)
	c.Next()
}

func AutoCreateWorkflow(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	// TODO: Authorization leak
	// this API is sometimes used in edit/create workflow scenario, thus giving the edit/create workflow permission
	// authorization check
	permitted := false
	projectKey := c.Query("projectName")

	if ctx.Resources.IsSystemAdmin {
		permitted = true
	} else if projectAuthInfo, ok := ctx.Resources.ProjectAuthInfo[projectKey]; ok {
		// first check if the user is projectAdmin
		if projectAuthInfo.IsProjectAdmin {
			permitted = true
		} else if projectAuthInfo.Workflow.Create ||
			projectAuthInfo.Env.EditConfig {
			// then check if user has edit workflow permission
			permitted = true
		} else {
			// finally check if the permission is given by collaboration mode
			collaborationAuthorizedEdit, err := internalhandler.CheckPermissionGivenByCollaborationMode(ctx.UserID, projectKey, types.ResourceTypeEnvironment, types.EnvActionEditConfig)
			if err == nil && collaborationAuthorizedEdit {
				permitted = true
			}
		}
	}

	if !permitted {
		ctx.UnAuthorized = true
		return
	}

	ctx.Resp = workflow.AutoCreateWorkflow(projectKey, ctx.Logger)
}

func CreateWorkflow(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := new(commonmodels.Workflow)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("CreateWorkflow c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("CreateWorkflow json.Unmarshal err : %v", err)
	}

	projectKey := args.ProductTmplName
	workflowName := args.Name

	internalhandler.InsertOperationLog(c, ctx.UserName, projectKey, "新增", "工作流", workflowName, string(data), types.RequestBodyTypeJSON, ctx.Logger)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	// authorization check
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectKey].Workflow.Create {
			ctx.UnAuthorized = true
			return
		}
	}

	if err := c.ShouldBindWith(&args, binding.JSON); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	args.UpdateBy = ctx.UserName
	args.CreateBy = ctx.UserName
	if err := workflow.CreateWorkflow(args, ctx.Logger); err != nil {
		ctx.RespErr = err
		return
	}

	if view := c.Query("viewName"); view != "" {
		workflow.AddWorkflowToView(args.ProductTmplName, view, []*commonmodels.WorkflowViewDetail{
			{
				WorkflowName:        args.Name,
				WorkflowDisplayName: args.DisplayName,
				WorkflowType:        setting.ProductWorkflowType,
			},
		}, ctx.Logger)
	}
}

// UpdateWorkflow  update a workflow
func UpdateWorkflow(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := new(commonmodels.Workflow)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("CreateWorkflow c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("UpdateWorkflow json.Unmarshal err : %v", err)
	}

	projectKey := args.ProductTmplName
	workflowName := args.Name

	internalhandler.InsertOperationLog(c, ctx.UserName, projectKey, "更新", "工作流", workflowName, string(data), types.RequestBodyTypeYAML, ctx.Logger)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	// authorization check
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectKey].Workflow.Edit {
			// check if the permission is given by collaboration mode
			permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, projectKey, types.ResourceTypeWorkflow, args.Name, types.WorkflowActionEdit)
			if err != nil || !permitted {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	if err := c.ShouldBindWith(&args, binding.JSON); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}
	args.UpdateBy = ctx.UserName
	ctx.RespErr = workflow.UpdateWorkflow(args, ctx.Logger)
}

func ListWorkflows(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	projects := c.QueryArray("projects")
	projectName := c.Query("projectName")
	if projectName != "" && len(projects) > 0 {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("projects and projectName can not be set together")
		return
	}
	if projectName != "" {
		projects = []string{c.Query("projectName")}
	}

	workflowNames, found := internalhandler.GetResourcesInHeader(c)
	if found && len(workflowNames) == 0 {
		ctx.Resp = []*workflow.Workflow{}
		return
	}

	ctx.Resp, ctx.RespErr = workflow.ListWorkflows(projects, ctx.UserID, workflowNames, ctx.Logger)
}

// TODO: this API is used only by picket, should be removed later
func ListTestWorkflows(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.RespErr = workflow.ListTestWorkflows(c.Param("testName"), c.QueryArray("projects"), ctx.Logger)
}

// FindWorkflow find a workflow
func FindWorkflow(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	workflowName := c.Param("name")

	w, err := workflow.FindWorkflow(workflowName, ctx.Logger)
	if err != nil {
		ctx.Logger.Errorf("FindWorkflow error: %v", err)
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}

	// authorization check
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[w.ProductTmplName]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if !ctx.Resources.ProjectAuthInfo[w.ProductTmplName].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[w.ProductTmplName].Workflow.View {
			// check if the permission is given by collaboration mode
			permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, w.ProductTmplName, types.ResourceTypeWorkflow, workflowName, types.WorkflowActionView)
			if err != nil || !permitted {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	ctx.Resp = w
}

func DeleteWorkflow(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.GetString("productName")
	workflowName := c.Param("name")

	// authorization check
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectKey].Workflow.Delete {
			ctx.UnAuthorized = true
			return
		}
	}

	internalhandler.InsertOperationLog(c, ctx.UserName, projectKey, "删除", "工作流", workflowName, "", types.RequestBodyTypeJSON, ctx.Logger)
	ctx.RespErr = commonservice.DeleteWorkflow(workflowName, ctx.RequestID, false, ctx.Logger)
}

func PreSetWorkflow(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.RespErr = workflow.PreSetWorkflow(c.Param("productName"), ctx.Logger)
}

func CopyWorkflow(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	workflowName := c.Param("old")

	w, err := workflow.FindWorkflow(workflowName, ctx.Logger)
	if err != nil {
		ctx.Logger.Errorf("FindWorkflow error: %v", err)
		ctx.RespErr = e.ErrInvalidParam.AddErr(err)
		return
	}

	// authorization check
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[w.ProductTmplName]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if !ctx.Resources.ProjectAuthInfo[w.ProductTmplName].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[w.ProductTmplName].Workflow.Create {
			ctx.UnAuthorized = true
			return
		}
	}

	ctx.RespErr = workflow.CopyWorkflow(workflowName, c.Param("new"), c.Param("newDisplay"), ctx.UserName, ctx.Logger)
}

// @Summary [DONT USE]  ZadigDeployJobSpec
// @Description [DONT USE] ZadigDeployJobSpec
// @Tags 	placeholder
// @Accept 	json
// @Produce json
// @Param 	deploy_job_spec 			body 		 commonmodels.ZadigDeployJobSpec	true 	"body"
// @Success 200
// @Router /api/aslan/placeholder/deploy_job_spec [post]
func PlaceholderDeplyJobSpec(c *gin.Context) {

}

// @Summary [DONT USE]  UpdateEnvIstioConfigJobSpec
// @Description [DONT USE] UpdateEnvIstioConfigJobSpec
// @Tags 	placeholder
// @Accept 	json
// @Produce json
// @Param 	update_env_istio_config_job_spec 			body 		 commonmodels.UpdateEnvIstioConfigJobSpec	true 	"body"
// @Success 200
// @Router /api/aslan/placeholder/update_env_istio_config_job_spec [post]
func PlaceholderUpdateEnvIstioConfigJobSpec(c *gin.Context) {

}

// @Summary [DONT USE]  CustomDeployJobSpec
// @Description [DONT USE] CustomDeployJobSpec
// @Tags 	placeholder
// @Accept 	json
// @Produce json
// @Param 	deploy_job_spec 			body 		 commonmodels.CustomDeployJobSpec	true 	"body"
// @Success 200
// @Router /api/aslan/placeholder/custom_deploy_job_spec [post]
func Placeholder2(c *gin.Context) {

}

// @Summary [DONT USE]  ZadigBuildJobSpec
// @Description [DONT USE] ZadigBuildJobSpec
// @Tags 	placeholder
// @Accept 	json
// @Produce json
// @Param 	deploy_job_spec 			body 		 commonmodels.ZadigBuildJobSpec	true 	"body"
// @Success 200
// @Router /api/aslan/placeholder/zadig_build_job_spec [post]
func Placeholder3(c *gin.Context) {

}

// @Summary [DONT USE]  ZadigHelmChartDeployJobSpec
// @Description [DONT USE] ZadigHelmChartDeployJobSpec
// @Tags 	placeholder
// @Accept 	json
// @Produce json
// @Param 	deploy_job_spec 			body 		 commonmodels.ZadigHelmChartDeployJobSpec	true 	"body"
// @Success 200
// @Router /api/aslan/placeholder/zadig_helm_chart_deploy_job_spec [post]
func Placeholder4(c *gin.Context) {

}

// @Summary [DONT USE]  K8sPatchJobSpec
// @Description [DONT USE] K8sPatchJobSpec
// @Tags 	placeholder
// @Accept 	json
// @Produce json
// @Param 	deploy_job_spec 			body 		 commonmodels.K8sPatchJobSpec	true 	"body"
// @Success 200
// @Router /api/aslan/placeholder/k8s_patch_job_spec [post]
func Placeholder5(c *gin.Context) {

}

// @Summary [DONT USE]  BlueGreenDeployV2JobSpec
// @Description [DONT USE] BlueGreenDeployV2JobSpec
// @Tags 	placeholder
// @Accept 	json
// @Produce json
// @Param 	blue_green_deploy_v2_jobSpec 			body 		 commonmodels.BlueGreenDeployV2JobSpec	true 	"body"
// @Success 200
// @Router /api/aslan/placeholder/blue_green_deploy_v2_jobSpec [post]
func Placeholder6(c *gin.Context) {

}

// @Summary [DONT USE]  CanaryDeployJobSpec
// @Description [DONT USE] CanaryDeployJobSpec
// @Tags 	placeholder
// @Accept 	json
// @Produce json
// @Param 	canary_deploy_job_spec 			body 		 commonmodels.CanaryDeployJobSpec	true 	"body"
// @Success 200
// @Router /api/aslan/placeholder/canary_deploy_job_spec [post]
func Placeholder7(c *gin.Context) {

}

// @Summary [DONT USE]  ZadigDistributeImageJobSpec
// @Description [DONT USE] ZadigDistributeImageJobSpec
// @Tags 	placeholder
// @Accept 	json
// @Produce json
// @Param 	zadig_distribute_image_job_spec 			body 		 commonmodels.ZadigDistributeImageJobSpec	true 	"body"
// @Success 200
// @Router /api/aslan/placeholder/zadig_distribute_image_job_spec [post]
func Placeholder8(c *gin.Context) {

}

// @Summary [DONT USE]  GrayReleaseJobSpec
// @Description [DONT USE] GrayReleaseJobSpec
// @Tags 	placeholder
// @Accept 	json
// @Produce json
// @Param 	gray_release_job_spec 			body 		 commonmodels.GrayReleaseJobSpec	true 	"body"
// @Success 200
// @Router /api/aslan/placeholder/gray_release_job_spec [post]
func Placeholder9(c *gin.Context) {

}

// @Summary [DONT USE]  GrayRollbackJobSpec
// @Description [DONT USE] GrayRollbackJobSpec
// @Tags 	placeholder
// @Accept 	json
// @Produce json
// @Param 	gray_rollback_job_spec 			body 		 commonmodels.GrayRollbackJobSpec	true 	"body"
// @Success 200
// @Router /api/aslan/placeholder/gray_rollback_job_spec [post]
func Placeholder10(c *gin.Context) {

}

// @Summary [DONT USE]  ZadigScanningJobSpec
// @Description [DONT USE] ZadigScanningJobSpec
// @Tags 	placeholder
// @Accept 	json
// @Produce json
// @Param 	zadig_scanning_job_spec 			body 		 commonmodels.ZadigScanningJobSpec	true 	"body"
// @Success 200
// @Router /api/aslan/placeholder/zadig_scanning_job_spec [post]
func Placeholder11(c *gin.Context) {

}

// @Summary [DONT USE]  ZadigVMDeployJobSpec
// @Description [DONT USE] ZadigVMDeployJobSpec
// @Tags 	placeholder
// @Accept 	json
// @Produce json
// @Param 	zadig_vm_deploy_job_spec 			body 		 commonmodels.ZadigVMDeployJobSpec	true 	"body"
// @Success 200
// @Router /api/aslan/placeholder/zadig_vm_deploy_job_spec [post]
func Placeholder12(c *gin.Context) {

}

// @Summary [DONT USE]  ApolloJobSpec
// @Description [DONT USE] ApolloJobSpec
// @Tags 	placeholder
// @Accept 	json
// @Produce json
// @Param 	apollo_job_spec 			body 		 commonmodels.ApolloJobSpec	true 	"body"
// @Success 200
// @Router /api/aslan/placeholder/apollo_job_spec [post]
func Placeholder13(c *gin.Context) {

}

// @Summary [DONT USE] JobTaskSQLSpec
// @Description [DONT USE] JobTaskSQLSpec
// @Tags 	placeholder
// @Accept 	json
// @Produce json
// @Param 	sql_task_spec 			body 		 commonmodels.JobTaskSQLSpec true 	"body"
// @Success 200
// @Router /api/aslan/placeholder/sql_job_task_spec [post]
func Placeholder14(c *gin.Context) {

}
