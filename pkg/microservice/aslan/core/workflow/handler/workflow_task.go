/*
Copyright 2022 The KodeRover Authors.

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
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/jinzhu/copier"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/notify"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/delivery/service"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/workflow/service/workflow"
	"github.com/koderover/zadig/v2/pkg/setting"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/types"
	"github.com/koderover/zadig/v2/pkg/types/dto"
)

// GetWorkflowArgs find workflow args
func GetWorkflowArgs(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	args := []*workflow.ServiceBuildInfo{}
	if err := c.ShouldBindWith(&args, binding.JSON); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	ctx.Resp, ctx.RespErr = workflow.GetWorkflowArgs(c.Param("productName"), c.Param("namespace"), args, ctx.Logger)
}

// PresetWorkflowArgs find a workflow task
func PresetWorkflowArgs(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.RespErr = workflow.PresetWorkflowArgs(c.Param("namespace"), c.Param("workflowName"), ctx.Logger)
}

// CreateWorkflowTask create a workflow task
func CreateWorkflowTask(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := new(commonmodels.WorkflowTaskArgs)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("CreateWorkflowTask c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("CreateWorkflowTask json.Unmarshal err : %v", err)
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, args.ProductTmplName, "新增", "工作流-task", args.WorkflowName, string(data), types.RequestBodyTypeJSON, ctx.Logger)

	// authorization check
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[args.ProductTmplName]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if !ctx.Resources.ProjectAuthInfo[args.ProductTmplName].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[args.ProductTmplName].Workflow.Execute {
			// check if the permission is given by collaboration mode
			permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, args.ProductTmplName, types.ResourceTypeWorkflow, args.WorkflowName, types.WorkflowActionRun)
			if err != nil || !permitted {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	if err := c.ShouldBindWith(&args, binding.JSON); err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
		return
	}

	if args.WorkflowTaskCreator != setting.CronTaskCreator && args.WorkflowTaskCreator != setting.WebhookTaskCreator {
		args.WorkflowTaskCreator = ctx.UserName
	}

	ctx.Resp, ctx.RespErr = workflow.CreateWorkflowTask(args, args.WorkflowTaskCreator, ctx.Logger)

	// 发送通知
	if ctx.RespErr != nil {
		notify.SendFailedTaskMessage(ctx.UserName, args.ProductTmplName, args.WorkflowName, ctx.RequestID, config.WorkflowType, ctx.RespErr, ctx.Logger)
	}
}

// CreateArtifactWorkflowTask create a artifact workflow task
func CreateArtifactWorkflowTask(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	args := new(commonmodels.WorkflowTaskArgs)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("CreateArtifactWorkflowTask c.GetRawData() err : %v", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("CreateArtifactWorkflowTask json.Unmarshal err : %v", err)
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, args.ProductTmplName, "新增", "工作流-task", args.WorkflowName, string(data), types.RequestBodyTypeJSON, ctx.Logger)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(data))

	// authorization check
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[args.ProductTmplName]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if !ctx.Resources.ProjectAuthInfo[args.ProductTmplName].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[args.ProductTmplName].Workflow.Execute {
			// check if the permission is given by collaboration mode
			permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, args.ProductTmplName, types.ResourceTypeWorkflow, args.WorkflowName, types.WorkflowActionRun)
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

	// license checks
	err = util.CheckZadigProfessionalLicense()
	if err != nil {
		if args.VersionArgs != nil && args.VersionArgs.Enabled {
			ctx.RespErr = e.ErrLicenseInvalid.AddDesc("只有专业版才能创建版本，请检查")
			return
		}
	}

	if args.WorkflowTaskCreator != setting.CronTaskCreator && args.WorkflowTaskCreator != setting.WebhookTaskCreator {
		args.WorkflowTaskCreator = ctx.UserName
	}

	ctx.Resp, ctx.RespErr = workflow.CreateArtifactWorkflowTask(args, args.WorkflowTaskCreator, ctx.Logger)
}

// ListWorkflowTasksResult workflowtask分页信息
func ListWorkflowTasksResult(c *gin.Context) {
	// TODO: add authorization info to this api, it is hard because it is used both by test & product workflows
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	workflowName := c.Param("name")

	maxResult, err := strconv.Atoi(c.Param("max"))
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid max result number")
		return
	}
	startAt, err := strconv.Atoi(c.Param("start"))
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid start at number")
		return
	}
	workflowTypeString := config.WorkflowType
	workflowType := c.Query("workflowType")
	if workflowType == string(config.TestType) {
		workflowTypeString = config.TestType
	}

	// authorization checks
	if workflowTypeString == config.WorkflowType {
		// if we are querying about a workflow, check if the user have workflow related roles or
		// given by collaboration mode
		w, err := workflow.FindWorkflowRaw(workflowName, ctx.Logger)
		if err != nil {
			ctx.Logger.Errorf("EnableDebugWorkflowTaskV4 error: %v", err)
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
	} else {
		// otherwise it is a test request, we just check if the user have test roles
		// TODO: add logics
	}

	filters := c.Query("filters")
	filtersList := strings.Split(filters, ",")
	ctx.Resp, ctx.RespErr = workflow.ListPipelineTasksV2Result(workflowName, workflowTypeString, c.Query("queryType"), filtersList, maxResult, startAt, ctx.Logger)
}

func GetFiltersPipeline(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	if c.Query("workflowType") != string(config.WorkflowType) {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid workflowType")
		return
	}
	ctx.Resp, ctx.RespErr = workflow.GetFiltersPipelineTaskV2(c.Query("projectName"), c.Param("name"), c.Query("queryType"), config.WorkflowType, ctx.Logger)
}

func GetWorkflowTask(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	workflowName := c.Param("name")

	taskID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid task id")
		return
	}
	workflowTypeString := config.WorkflowType
	workflowType := c.Query("workflowType")
	if workflowType == string(config.TestType) {
		workflowTypeString = config.TestType
	}

	// authorization checks
	if workflowTypeString == config.WorkflowType {
		// if we are querying about a workflow, check if the user have workflow related roles or
		// given by collaboration mode
		w, err := workflow.FindWorkflowRaw(workflowName, ctx.Logger)
		if err != nil {
			ctx.Logger.Errorf("EnableDebugWorkflowTaskV4 error: %v", err)
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
	} else {
		// otherwise it is a test request, we just check if the user have test roles
		// TODO: add logics
	}

	task, err := workflow.GetPipelineTaskV2(taskID, workflowName, workflowTypeString, ctx.Logger)
	if err != nil {
		ctx.RespErr = err
		return
	}
	releases, _, err := service.ListDeliveryVersion(&service.ListDeliveryVersionArgs{
		TaskId:       int(task.TaskID),
		ServiceName:  task.ServiceName,
		ProjectName:  task.ProductName,
		WorkflowName: task.PipelineName,
	}, ctx.Logger)
	if err != nil {
		ctx.RespErr = err
		return
	}

	var toReleases []dto.Release
	for _, v := range releases {
		toReleases = append(toReleases, dto.Release{
			ID:      v.VersionInfo.ID,
			Version: v.VersionInfo.Version,
		})
	}
	var toTask dto.Task
	if err := copier.Copy(&toTask, task); err != nil {
		ctx.RespErr = err
		return
	}
	toTask.Releases = toReleases
	ctx.Resp = toTask
	ctx.RespErr = err
	return
}

func RestartWorkflowTask(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Query("projectName")
	workflowName := c.Param("name")

	internalhandler.InsertOperationLog(c, ctx.UserName, projectKey, "重启", "工作流-task", workflowName, "", types.RequestBodyTypeJSON, ctx.Logger)

	// authorization check
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectKey].Workflow.Execute {
			// check if the permission is given by collaboration mode
			permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, projectKey, types.ResourceTypeWorkflow, workflowName, types.WorkflowActionRun)
			if err != nil || !permitted {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	taskID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid task id")
		return
	}

	ctx.RespErr = workflow.RestartPipelineTaskV2(ctx.UserName, taskID, workflowName, config.WorkflowType, ctx.Logger)
}

func CancelWorkflowTaskV2(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {

		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	projectKey := c.Query("projectName")
	workflowName := c.Param("name")

	internalhandler.InsertOperationLog(c, ctx.UserName, projectKey, "取消", "工作流-task", workflowName, "", types.RequestBodyTypeJSON, ctx.Logger)

	// authorization check
	if !ctx.Resources.IsSystemAdmin {
		if _, ok := ctx.Resources.ProjectAuthInfo[projectKey]; !ok {
			ctx.UnAuthorized = true
			return
		}

		if !ctx.Resources.ProjectAuthInfo[projectKey].IsProjectAdmin &&
			!ctx.Resources.ProjectAuthInfo[projectKey].Workflow.Execute {
			// check if the permission is given by collaboration mode
			permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, projectKey, types.ResourceTypeWorkflow, workflowName, types.WorkflowActionRun)
			if err != nil || !permitted {
				ctx.UnAuthorized = true
				return
			}
		}
	}

	taskID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid task id")
		return
	}
	ctx.RespErr = commonservice.CancelTaskV2(ctx.UserName, workflowName, taskID, config.WorkflowType, ctx.RequestID, ctx.Logger)
}

func GetWorkflowTaskCallback(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	taskID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid task id")
		return
	}

	ctx.Resp, ctx.RespErr = commonservice.GetWorkflowTaskCallback(taskID, c.Param("name"))
}
