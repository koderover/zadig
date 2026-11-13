package handler

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"

	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	envservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/environment/service"
	logservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/log/service"
	workflowservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/workflow/service/workflow"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	"github.com/koderover/zadig/v2/pkg/tool/clientmanager"
	"github.com/koderover/zadig/v2/pkg/types"
)

func parseLogWindow(offsetValue, limitValue string) (int, int, error) {
	offset, err := strconv.Atoi(offsetValue)
	if err != nil || offset < 0 || offset > 5000 {
		return 0, 0, fmt.Errorf("offset 必须在 0 到 5000 之间")
	}
	limit, err := strconv.Atoi(limitValue)
	if err != nil || limit < 1 || limit > 500 {
		return 0, 0, fmt.Errorf("日志窗口行数必须在 1 到 500 之间")
	}
	return offset, limit, nil
}

func getWorkflowLogExcerpt(c *gin.Context) {
	offset, limit, err := parseLogWindow(c.DefaultQuery("offset", "0"), c.DefaultQuery("limit", "200"))
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	ctx, err := internalhandler.NewContextWithAuthorization(c)
	if err != nil || ctx.UserID == "" {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}
	taskID, err := strconv.ParseInt(c.Param("taskID"), 10, 64)
	if err != nil || taskID < 1 {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	w, err := workflowservice.FindWorkflowV4Raw(c.Param("workflowName"), ctx.Logger)
	if err != nil || w == nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	if w.Project != c.Query("projectName") {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	if !ctx.Resources.IsSystemAdmin {
		permissions := ctx.Resources.ProjectAuthInfo[w.Project]
		if permissions == nil {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}
		if !permissions.IsProjectAdmin && (permissions.Workflow == nil || !permissions.Workflow.View) {
			allowed, err := internalhandler.GetCollaborationModePermission(ctx.UserID, w.Project, types.ResourceTypeWorkflow, w.Name, types.WorkflowActionView)
			if err != nil || !allowed {
				c.AbortWithStatus(http.StatusForbidden)
				return
			}
		}
	}
	task, err := commonrepo.NewworkflowTaskv4Coll().Find(w.Name, taskID)
	if err != nil || task == nil || task.ProjectName != w.Project {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	found := false
	for _, stage := range task.Stages {
		for _, job := range stage.Jobs {
			if job.Name == c.Param("jobName") {
				found = true
			}
		}
	}
	if !found {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	result, err := logservice.GetWorkflowLogExcerpt(c.Request.Context(), strings.ToLower(w.Name), c.Param("jobName"), taskID, offset, limit, ctx.Logger)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": "日志片段读取失败"})
		return
	}
	c.JSON(http.StatusOK, result)
}

func containsServiceContainer(pods []*envservice.OpenAPIServicePodInfo, podName, container string) bool {
	for _, pod := range pods {
		if pod == nil || pod.PodName != podName {
			continue
		}
		for _, item := range pod.Containers {
			if item.Name == container {
				return true
			}
		}
	}
	return false
}

func getContainerLogExcerpt(c *gin.Context) {
	_, limit, err := parseLogWindow("0", c.DefaultQuery("tailLines", "200"))
	previous, previousErr := strconv.ParseBool(c.DefaultQuery("previous", "false"))
	follow, followErr := strconv.ParseBool(c.DefaultQuery("follow", "false"))
	if err != nil || previousErr != nil || followErr != nil || follow {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": "片段模式需要有效行数和布尔参数，且不能持续跟随日志"})
		return
	}
	project, envName, service := c.Query("projectName"), c.Query("envName"), c.Query("serviceName")
	if project == "" || envName == "" || service == "" || c.Param("name") == "" || c.Query("container") == "" {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	if err != nil || ctx.UserID == "" {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	// 使用实际记录的生产属性，不新增查询参数，也不在定位不唯一时猜测环境。
	envs, err := commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{Name: project, EnvName: envName})
	if err != nil {
		c.AbortWithStatus(http.StatusBadGateway)
		return
	}
	if len(envs) == 0 {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	if len(envs) != 1 {
		c.AbortWithStatus(http.StatusConflict)
		return
	}
	env := envs[0]
	allowed := ctx.Resources.IsSystemAdmin
	if permission := ctx.Resources.ProjectAuthInfo[project]; permission != nil {
		allowed = allowed || permission.IsProjectAdmin
		if env.Production {
			allowed = allowed || (permission.ProductionEnv != nil && permission.ProductionEnv.View)
		} else {
			allowed = allowed || (permission.Env != nil && permission.Env.View)
		}
	}
	if !allowed {
		action := types.EnvActionView
		if env.Production {
			action = types.ProductionEnvActionView
		}
		allowed, err = internalhandler.GetCollaborationModePermission(ctx.UserID, project, types.ResourceTypeEnvironment, envName, action)
	}
	if err != nil || !allowed {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}
	if env.Production && commonutil.CheckZadigProfessionalLicense() != nil {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}
	pods, err := envservice.OpenAPIListServicePods(project, envName, service, env.Production, ctx.Logger)
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	if !containsServiceContainer(pods.Pods, c.Param("name"), c.Query("container")) {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	client, err := clientmanager.NewKubeClientManager().GetKubernetesClientSet(env.ClusterID)
	if err != nil {
		c.AbortWithStatus(http.StatusBadGateway)
		return
	}
	readCtx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	lines, bytes := int64(limit), int64(48<<10)
	stream, err := client.CoreV1().Pods(env.Namespace).GetLogs(c.Param("name"), &corev1.PodLogOptions{
		Container: c.Query("container"), Previous: previous, TailLines: &lines, LimitBytes: &bytes,
	}).Stream(readCtx)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadGateway, gin.H{"message": "容器日志不可用"})
		return
	}
	defer stream.Close()
	result, err := logservice.ReadLogExcerpt(readCtx, stream, 0, limit)
	if err != nil {
		c.AbortWithStatus(http.StatusBadGateway)
		return
	}
	// 容器日志已在 Kubernetes 端截断，不能将局部行数冒充完整日志总量。
	result.TotalLines = -1
	result.NextOffset = 0
	result.Truncated = true
	c.JSON(http.StatusOK, result)
}
