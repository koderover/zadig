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
	"log"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	runtimeJobController "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/workflowcontroller/jobcontroller"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	logservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/log/service"
	jobcontroller "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/workflow/service/workflow/controller/job"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/workflow/testing/service"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/types"
	stepspec "github.com/koderover/zadig/v2/pkg/types/step"
	"github.com/koderover/zadig/v2/pkg/util/ginzap"
)

// parseSinceSeconds parses query param "since" as a duration (e.g. 5m, 1h). Returns nil when empty; error when invalid.
func parseSinceSeconds(since string) (*int64, error) {
	if since == "" {
		return nil, nil
	}
	d, err := time.ParseDuration(since)
	if err != nil {
		return nil, fmt.Errorf("invalid since: %w", err)
	}
	sec := int64(d / time.Second)
	if sec < 1 {
		sec = 1
	}
	return &sec, nil
}

// GetContainerLogsSSE streams container logs via SSE.
// Query params: envName, projectName, tails (default 10), since (optional, e.g. 5m, 1h; Go duration).
// @Param since query string false "Only logs from the last duration, e.g. 5m, 1h (time.ParseDuration format)"
func GetContainerLogsSSE(c *gin.Context) {
	logger := ginzap.WithContext(c).Sugar()

	ctx, err := internalhandler.NewContextWithAuthorization(c)
	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		internalhandler.JSONResponse(c, ctx)
		return
	}

	tails, err := strconv.ParseInt(c.Query("tails"), 10, 64)
	if err != nil {
		tails = int64(10)
	}
	sinceSeconds, err := parseSinceSeconds(c.Query("since"))
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
		internalhandler.JSONResponse(c, ctx)
		return
	}

	envName := c.Query("envName")
	productName := c.Query("projectName")
	isProduction := c.Query("production") == "true"

	if !isProduction {
		// authorization checks
		if !ctx.Resources.IsSystemAdmin {
			if _, ok := ctx.Resources.ProjectAuthInfo[productName]; !ok {
				ctx.UnAuthorized = true
				internalhandler.JSONResponse(c, ctx)
				return
			}
			if !ctx.Resources.ProjectAuthInfo[productName].Env.View &&
				!ctx.Resources.ProjectAuthInfo[productName].IsProjectAdmin {
				permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, productName, types.ResourceTypeEnvironment, envName, types.EnvActionView)
				if err != nil || !permitted {
					ctx.UnAuthorized = true
					internalhandler.JSONResponse(c, ctx)
					return
				}
			}
		}

		internalhandler.Stream(c, func(ctx context.Context, streamChan chan interface{}) {
			logservice.ContainerLogStream(ctx, streamChan, envName, productName, c.Param("podName"), c.Param("containerName"), true, tails, sinceSeconds, logger)
		}, logger)
	} else {
		// authorization checks
		if !ctx.Resources.IsSystemAdmin {
			if _, ok := ctx.Resources.ProjectAuthInfo[productName]; !ok {
				ctx.UnAuthorized = true
				internalhandler.JSONResponse(c, ctx)
				return
			}
			if !ctx.Resources.ProjectAuthInfo[productName].ProductionEnv.View &&
				!ctx.Resources.ProjectAuthInfo[productName].IsProjectAdmin {
				permitted, err := internalhandler.GetCollaborationModePermission(ctx.UserID, productName, types.ResourceTypeEnvironment, envName, types.ProductionEnvActionView)
				if err != nil || !permitted {
					ctx.UnAuthorized = true
					internalhandler.JSONResponse(c, ctx)
					return
				}
			}
		}

		internalhandler.Stream(c, func(ctx context.Context, streamChan chan interface{}) {
			logservice.ContainerLogStream(ctx, streamChan, envName, productName, c.Param("podName"), c.Param("containerName"), true, tails, sinceSeconds, logger)
		}, logger)
	}

}

func GetWorkflowJobContainerLogsSSE(c *gin.Context) {
	ctx := internalhandler.NewContext(c)

	taskID, err := strconv.ParseInt(c.Param("taskID"), 10, 64)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid task id")
		internalhandler.JSONResponse(c, ctx)
		return
	}

	tails, err := strconv.ParseInt(c.Param("lines"), 10, 64)
	if err != nil {
		tails = int64(10)
	}

	jobName := c.Param("jobName")

	internalhandler.Stream(c, func(ctx1 context.Context, streamChan chan interface{}) {
		logservice.WorkflowTaskV4ContainerLogStream(
			ctx1, streamChan,
			&logservice.GetContainerOptions{
				Namespace:    config.Namespace(),
				PipelineName: c.Param("workflowName"),
				SubTask:      runtimeJobController.GetJobContainerName(jobName),
				TaskID:       taskID,
				TailLines:    tails,
			},
			ctx.Logger)
	}, ctx.Logger)
}

func GetScanningContainerLogsSSE(c *gin.Context) {
	ctx := internalhandler.NewContext(c)

	id := c.Param("id")
	if id == "" {
		ctx.RespErr = fmt.Errorf("id must be provided")
		internalhandler.JSONResponse(c, ctx)
		return
	}

	taskIDStr := c.Param("scan_id")
	if taskIDStr == "" {
		ctx.RespErr = fmt.Errorf("scan_id must be provided")
		internalhandler.JSONResponse(c, ctx)
		return
	}

	taskID, err := strconv.ParseInt(taskIDStr, 10, 64)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid task id")
		internalhandler.JSONResponse(c, ctx)
		return
	}

	tails, err := strconv.ParseInt(c.Query("lines"), 10, 64)
	if err != nil {
		tails = int64(10)
	}

	resp, err := service.GetScanningModuleByID(id, ctx.Logger)
	if err != nil {
		ctx.RespErr = fmt.Errorf("failed to get scanning module by id: %s, err: %v", id, err)
		internalhandler.JSONResponse(c, ctx)
		return
	}

	clusterId := ""
	namespace := config.Namespace()
	if resp.AdvancedSetting != nil {
		clusterId = resp.AdvancedSetting.ClusterID
	}

	workflowName := commonutil.GenScanningWorkflowName(id)
	workflowTask, err := commonrepo.NewworkflowTaskv4Coll().Find(workflowName, taskID)
	if err != nil {
		ctx.Logger.Errorf("failed to find workflow task for scanning: %s, err: %s", workflowName, err)
		ctx.RespErr = err
		internalhandler.JSONResponse(c, ctx)
		return
	}
	if len(workflowTask.Stages) != 1 {
		log.Printf("Invalid stage length: stage length for scanning should be 1")
		ctx.RespErr = fmt.Errorf("invalid stage length")
		internalhandler.JSONResponse(c, ctx)
		return
	}

	if len(workflowTask.Stages[0].Jobs) != 1 {
		log.Printf("Invalid Job length: job length for scanning should be 1")
		ctx.RespErr = fmt.Errorf("invalid job length")
		internalhandler.JSONResponse(c, ctx)
		return
	}

	job := workflowTask.Stages[0].Jobs[0]
	jobName := jobcontroller.GenJobName(workflowTask.WorkflowArgs, job.OriginName, 0)

	internalhandler.Stream(c, func(ctx1 context.Context, streamChan chan interface{}) {
		logservice.WorkflowTaskV4ContainerLogStream(
			ctx1, streamChan,
			&logservice.GetContainerOptions{
				Namespace:    namespace,
				PipelineName: commonutil.GenScanningWorkflowName(id),
				SubTask:      runtimeJobController.GetJobContainerName(jobName),
				TaskID:       taskID,
				TailLines:    tails,
				ClusterID:    clusterId,
			},
			ctx.Logger)
	}, ctx.Logger)
}

func GetTestingContainerLogsSSE(c *gin.Context) {
	ctx := internalhandler.NewContext(c)

	testName := c.Param("test_name")
	if testName == "" {
		ctx.RespErr = fmt.Errorf("testName must be provided")
		internalhandler.JSONResponse(c, ctx)
		return
	}

	taskIDStr := c.Param("task_id")
	if taskIDStr == "" {
		ctx.RespErr = fmt.Errorf("task_id must be provided")
		internalhandler.JSONResponse(c, ctx)
		return
	}

	taskID, err := strconv.ParseInt(taskIDStr, 10, 64)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid task id")
		internalhandler.JSONResponse(c, ctx)
		return
	}

	tails, err := strconv.ParseInt(c.Query("lines"), 10, 64)
	if err != nil {
		tails = int64(9999999)
	}

	workflowName := commonutil.GenTestingWorkflowName(testName)
	workflowTask, err := commonrepo.NewworkflowTaskv4Coll().Find(workflowName, taskID)
	if err != nil {
		ctx.Logger.Errorf("failed to find workflow task for testing: %s, err: %s", testName, err)
		ctx.RespErr = err
		internalhandler.JSONResponse(c, ctx)
		return
	}

	if len(workflowTask.Stages) != 1 {
		ctx.Logger.Errorf("Invalid stage length: stage length for testing should be 1")
		ctx.RespErr = fmt.Errorf("invalid stage length")
		internalhandler.JSONResponse(c, ctx)
		return
	}

	if len(workflowTask.Stages[0].Jobs) != 1 {
		ctx.Logger.Errorf("Invalid Job length: job length for testing should be 1")
		ctx.RespErr = fmt.Errorf("invalid job length")
		internalhandler.JSONResponse(c, ctx)
		return
	}

	job := workflowTask.Stages[0].Jobs[0]
	jobName := jobcontroller.GenJobName(workflowTask.WorkflowArgs, job.OriginName, 0)

	internalhandler.Stream(c, func(ctx1 context.Context, streamChan chan interface{}) {
		logservice.WorkflowTaskV4ContainerLogStream(
			ctx1, streamChan,
			&logservice.GetContainerOptions{
				Namespace:    config.Namespace(),
				PipelineName: commonutil.GenTestingWorkflowName(testName),
				SubTask:      runtimeJobController.GetJobContainerName(jobName),
				TaskID:       taskID,
				TailLines:    tails,
			},
			ctx.Logger)
	}, ctx.Logger)
}

func GetJenkinsJobContainerLogsSSE(c *gin.Context) {
	ctx := internalhandler.NewContext(c)

	jobID, err := strconv.ParseInt(c.Param("jobID"), 10, 64)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid task id")
		internalhandler.JSONResponse(c, ctx)
		return
	}

	internalhandler.Stream(c, func(ctx1 context.Context, streamChan chan interface{}) {
		logservice.JenkinsJobLogStream(ctx1, c.Param("id"), c.Param("jobName"), jobID, streamChan)
	}, ctx.Logger)
}

// OpenAPIGetContainerLogsSSE streams container logs via SSE (OpenAPI).
// Query params: envName, projectKey, tails (default 10), since (optional, e.g. 5m, 1h; Go duration).
// @Param since query string false "Only logs from the last duration, e.g. 5m, 1h (time.ParseDuration format)"
func OpenAPIGetContainerLogsSSE(c *gin.Context) {
	logger := ginzap.WithContext(c).Sugar()

	tails, err := strconv.ParseInt(c.Query("tails"), 10, 64)
	if err != nil {
		tails = int64(10)
	}
	sinceSeconds, err := parseSinceSeconds(c.Query("since"))
	if err != nil {
		ctx := internalhandler.NewContext(c)
		ctx.RespErr = e.ErrInvalidParam.AddDesc(err.Error())
		internalhandler.JSONResponse(c, ctx)
		return
	}

	envName := c.Query("envName")
	productName := c.Query("projectKey")

	internalhandler.Stream(c, func(ctx context.Context, streamChan chan interface{}) {
		logservice.ContainerLogStream(ctx, streamChan, envName, productName, c.Param("podName"), c.Param("containerName"), true, tails, sinceSeconds, logger)
	}, logger)
}

func OpenAPIGetWorkflowJobContainerLogsSSE(c *gin.Context) {
	ctx := internalhandler.NewContext(c)

	taskID, err := strconv.ParseInt(c.Param("taskID"), 10, 64)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("invalid task id")
		internalhandler.JSONResponse(c, ctx)
		return
	}

	tails, err := strconv.ParseInt(c.Param("lines"), 10, 64)
	if err != nil {
		tails = int64(10)
	}

	jobName := c.Param("jobName")

	internalhandler.Stream(c, func(ctx1 context.Context, streamChan chan interface{}) {
		logservice.WorkflowTaskV4ContainerLogStream(
			ctx1, streamChan,
			&logservice.GetContainerOptions{
				Namespace:    config.Namespace(),
				PipelineName: c.Param("workflowName"),
				SubTask:      runtimeJobController.GetJobContainerName(jobName),
				TaskID:       taskID,
				TailLines:    tails,
			},
			ctx.Logger)
	}, ctx.Logger)
}

// @Summary Get Delivery Version sse logs
// @Description Get Delivery Version sse logs
// @Tags 	logs
// @Accept 	json
// @Produce json
// @Param 	projectName 		query 		string							true	"项目标识"
// @Param 	version 			query 		string							true	"版本名称"
// @Param 	serviceName 		query 		string							true	"服务名"
// @Param 	serviceModule 		query 		string							true	"服务组件"
// @Param 	lines 				path 		string							true	"行数"
// @Success 200     			{string} 	string
// @Router /api/aslan/logs/sse/delivery/{lines} [get]
func GetDeliveryVersionLogsSSE(c *gin.Context) {
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

	tails, err := strconv.ParseInt(c.Param("lines"), 10, 64)
	if err != nil {
		tails = int64(10)
	}

	internalhandler.Stream(c, func(ctx1 context.Context, streamChan chan interface{}) {
		logservice.WorkflowTaskV4ContainerLogStream(
			ctx1, streamChan,
			&logservice.GetContainerOptions{
				Namespace:    config.Namespace(),
				PipelineName: version.WorkflowName,
				SubTask:      runtimeJobController.GetJobContainerName(jobName),
				TaskID:       version.TaskID,
				TailLines:    tails,
			},
			ctx.Logger)
	}, ctx.Logger)
}

func findDeliveryVersionJobName(ctx *internalhandler.Context, workflowName string, taskID int64, serviceName string, serviceModule string) (string, error) {
	task, err := commonrepo.NewworkflowTaskv4Coll().Find(workflowName, taskID)
	if err != nil {
		return "", fmt.Errorf("failed to find workflow: %s", err)
	}

	jobName := ""
	for _, stage := range task.Stages {
		for _, job := range stage.Jobs {
			if job.JobType == string(config.JobZadigDistributeImage) {
				jobSpec := new(commonmodels.JobTaskFreestyleSpec)
				if err := commonmodels.IToi(job.Spec, jobSpec); err != nil {
					return "", fmt.Errorf("failed to decode job spec: %s", err)
				}

				for _, step := range jobSpec.Steps {
					if string(step.StepType) == string(config.StepDistributeImage) {
						distributeImageStep := new(stepspec.StepImageDistributeSpec)
						if err := commonmodels.IToi(step.Spec, distributeImageStep); err != nil {
							return "", fmt.Errorf("failed to decode distribute image step: %s", err)
						}

						for _, image := range distributeImageStep.DistributeTarget {
							if image.ServiceName == serviceName && image.ServiceModule == serviceModule {
								jobName = job.Name
								goto FoundJobName
							}
						}
					}
				}
			}
		}
	}

FoundJobName:
	if jobName == "" {
		return "", fmt.Errorf("failed to find job name")
	}
	return jobName, nil
}
