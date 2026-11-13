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
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	logservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/log/service"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

// GetWorkflowV4JobContainerLogs godoc
// @Summary 获取工作流步骤日志
// @Description 默认保持原有完整日志字符串响应；format=excerpt 时校验当前用户权限及任务、步骤归属，返回从末尾向前读取的有限片段，最多 48 KiB。
// @Tags logs
// @Produce json
// @Param Authorization header string true "Bearer 用户登录令牌"
// @Param workflowName path string true "工作流标识"
// @Param taskID path int64 true "工作流任务编号" minimum(1)
// @Param jobName path string true "任务中的步骤名称"
// @Param projectName query string false "片段模式必填，须与任务实际项目一致"
// @Param format query string false "传 excerpt 使用片段模式，不传保持完整日志响应" Enums(excerpt)
// @Param offset query int false "片段模式从日志末尾跳过的行数，后续使用 next_offset" default(0) minimum(0) maximum(5000)
// @Param limit query int false "片段模式的窗口行数" default(200) minimum(1) maximum(500)
// @Success 200 {object} logservice.LogExcerpt "format=excerpt 时返回日志片段；未传时仍返回原有 JSON 字符串"
// @Failure 400 "参数无效、任务归属不匹配或日志无法读取"
// @Failure 403 "片段模式缺少当前用户的工作流查看权限"
// @Failure 404 "工作流、任务或步骤不存在"
// @Router /api/logs/log/v4/workflow/{workflowName}/tasks/{taskID}/jobs/{jobName} [get]
func GetWorkflowV4JobContainerLogs(c *gin.Context) {
	if c.Query("format") == "excerpt" {
		getWorkflowLogExcerpt(c)
		return
	}

	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	taskID, err := strconv.ParseInt(c.Param("taskID"), 10, 64)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid task id")
		return
	}
	// Use all lowercase job names to avoid subdomain errors
	ctx.Resp, ctx.RespErr = logservice.GetWorkflowV4JobContainerLogs(strings.ToLower(c.Param("workflowName")), c.Param("jobName"), taskID, ctx.Logger)
}

// GetContainerLogs godoc
// @Summary 获取容器日志
// @Description 不传 format 时保留原有字符串或流式响应。format=excerpt 时按已有项目和环境名称定位实际环境，校验环境权限及 Pod、容器归属，非流式返回最多 48 KiB 片段；不新增 production 参数。无法唯一定位环境时拒绝读取。
// @Tags logs
// @Produce json
// @Param Authorization header string true "Bearer 用户登录令牌"
// @Param name path string true "Pod 名称"
// @Param container query string true "容器名称"
// @Param projectName query string true "项目标识"
// @Param envName query string true "环境名称"
// @Param serviceName query string false "片段模式必填，用于校验 Pod 和容器属于目标服务"
// @Param format query string false "传 excerpt 使用片段模式" Enums(excerpt)
// @Param tailLines query int false "最近日志行数；片段模式默认 200，最多 500"
// @Param previous query bool false "片段模式读取最近一次退出的容器实例日志" default(false)
// @Param follow query bool false "是否持续跟随日志；片段模式只允许 false" default(false)
// @Success 200 {object} logservice.LogExcerpt "片段模式响应；未传 format 时保留原有返回格式"
// @Failure 400 "片段模式参数无效或目标无法读取"
// @Failure 403 "片段模式缺少环境查看权限或生产环境许可"
// @Failure 404 "环境、服务、Pod 或容器不存在"
// @Failure 409 "无法唯一确定目标环境"
// @Failure 502 "集群或容器日志查询失败"
// @Router /api/logs/pods/{name} [get]
func GetContainerLogs(c *gin.Context) {
	if c.Query("format") == "excerpt" {
		getContainerLogExcerpt(c)
		return
	}

	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	podName := c.Param("name")
	containerName := c.Query("container")
	envName := c.Query("envName")
	productName := c.Query("projectName")

	tailLines, err := strconv.ParseInt(c.Query("tailLines"), 10, 64)
	if err != nil {
		tailLines = int64(-1)
	}

	follow, err := strconv.ParseBool(c.Query("follow"))
	if err != nil {
		follow = false
	}

	if !follow {
		ctx.Resp, ctx.RespErr = logservice.GetCurrentContainerLogs(podName, containerName, envName, productName, tailLines, ctx.Logger)
		return
	}

	internalhandler.Stream(c, func(ctx1 context.Context, streamChan chan interface{}) {
		logservice.ContainerLogStream(ctx1, streamChan, envName, productName, podName, containerName, nil, follow, tailLines, nil, ctx.Logger)
	}, ctx.Logger)
}

func GetScanningContainerLogs(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	id := c.Param("id")
	if id == "" {
		ctx.RespErr = fmt.Errorf("id must be provided")
		return
	}

	taskIDStr := c.Param("scan_id")
	if taskIDStr == "" {
		ctx.RespErr = fmt.Errorf("scan_id must be provided")
		return
	}

	taskID, err := strconv.ParseInt(taskIDStr, 10, 64)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid task id")
		return
	}

	ctx.Resp, ctx.RespErr = logservice.GetScanningContainerLogs(id, taskID, ctx.Logger)
}

func GetTestingContainerLogs(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	testName := c.Param("test_name")
	if testName == "" {
		ctx.RespErr = fmt.Errorf("testName must be provided")
		return
	}

	taskIDStr := c.Param("task_id")
	if taskIDStr == "" {
		ctx.RespErr = fmt.Errorf("task_id must be provided")
		return
	}

	taskID, err := strconv.ParseInt(taskIDStr, 10, 64)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid task id")
		return
	}

	ctx.Resp, ctx.RespErr = logservice.GetTestingContainerLogs(testName, taskID, ctx.Logger)
}

func OpenAPIGetWorkflowV4JobContainerLogs(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	taskID, err := strconv.ParseInt(c.Param("taskID"), 10, 64)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid task id")
		return
	}
	// Use all lowercase job names to avoid subdomain errors
	ctx.Resp, ctx.RespErr = logservice.GetWorkflowV4JobContainerLogs(strings.ToLower(c.Param("workflowName")), c.Param("jobName"), taskID, ctx.Logger)
}

// @Summary Get Delivery Version logs
// @Description Get Delivery Version logs
// @Tags 	logs
// @Accept 	json
// @Produce json
// @Param 	projectName 		query 		string							true	"项目标识"
// @Param 	version 			query 		string							true	"版本名称"
// @Param 	serviceName 		query 		string							true	"服务名"
// @Param 	serviceModule 		query 		string							true	"服务组件"
// @Success 200     			{string} 	string
// @Router /api/aslan/logs/log/delivery [get]
func GetDeliveryVersionLogs(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	projectName := c.Query("projectName")
	if projectName == "" {
		ctx.RespErr = fmt.Errorf("projectName must be provided")
		return
	}

	versionName := c.Query("version")
	if versionName == "" {
		ctx.RespErr = fmt.Errorf("version must be provided")
		return
	}

	serviceName := c.Query("serviceName")
	if serviceName == "" {
		ctx.RespErr = fmt.Errorf("serviceName must be provided")
		return
	}

	serviceModule := c.Query("serviceModule")
	if serviceModule == "" {
		ctx.RespErr = fmt.Errorf("serviceModule must be provided")
		return
	}

	version, err := commonrepo.NewDeliveryVersionV2Coll().Find(projectName, versionName)
	if err != nil {
		ctx.RespErr = fmt.Errorf("failed to find delivery version: %s", err)
		return
	}

	jobName, err := findDeliveryVersionJobName(ctx, version.WorkflowName, version.TaskID, serviceName, serviceModule)
	if err != nil {
		ctx.RespErr = fmt.Errorf("failed to find job name: %s", err)
		return
	}

	// Use all lowercase job names to avoid subdomain errors
	ctx.Resp, ctx.RespErr = logservice.GetWorkflowV4JobContainerLogs(strings.ToLower(version.WorkflowName), jobName, version.TaskID, ctx.Logger)
}
