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

package service

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/koderover/zadig/v2/pkg/tool/clientmanager"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/kube/getter"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

func ServeWs(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	// 提取用户身份信息
	userInfo := &UserInfo{
		UserID:   ctx.UserID,
		UserName: ctx.UserName,
	}

	// 检查是否为重连请求
	sessionID := c.Query("session_id")
	sessionMgr := GetSessionManager()

	if sessionID != "" {
		// 🔒 重连场景：验证用户身份
		if reconnectErr := sessionMgr.ReconnectSession(sessionID, c.Writer, c.Request, userInfo); reconnectErr == nil {
			log.Infof("session %s reconnected for user %s from %s", sessionID, userInfo.UserName, c.ClientIP())
			ctx.RespErr = nil
			return
		} else {
			log.Warnf("failed to reconnect session %s for user %s: %v, creating new session", sessionID, userInfo.UserName, reconnectErr)
		}
	}

	// 新建会话场景
	podName := c.Param("podName")
	containerName := c.Param("containerName")

	if podName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("podName can't be empty,please check!")
		return
	}
	log.Infof("exec containerName: %s, pod: %s", containerName, podName)

	productName := c.Query("projectName")
	envName := c.Param("envName")
	productInfo, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{Name: productName, EnvName: envName})
	if err != nil {
		ctx.RespErr = e.ErrInternalError.AddDesc(fmt.Sprintf("failed to find product %s/%s, err: %s", productName, envName, err))
		return
	}
	namespace, clusterID := productInfo.Namespace, productInfo.ClusterID

	// 验证 Pod 是否存在
	kubeCli, err := clientmanager.NewKubeClientManager().GetKubernetesClientSet(clusterID)
	if err != nil {
		ctx.RespErr = e.ErrInternalError.AddDesc(fmt.Sprintf("get kubecli err :%v", err))
		return
	}

	ok, err := ValidatePod(kubeCli, namespace, podName, containerName)
	if !ok {
		ctx.RespErr = e.ErrInternalError.AddDesc(fmt.Sprintf("Validate pod error! err: %v", err))
		return
	}

	// 创建会话上下文
	execCtx := &ExecContext{
		ClusterID:     clusterID,
		Namespace:     namespace,
		PodName:       podName,
		ContainerName: containerName,
		Command:       []string{"sh"}, // 使用标准 shell
	}

	// 创建新会话（传入用户信息）
	newSessionID, pty, err := sessionMgr.CreateSession(c.Writer, c.Request, execCtx, nil, userInfo)
	if err != nil {
		log.Errorf("failed to create session: %v", err)
		ctx.RespErr = e.ErrInternalError.AddDesc(fmt.Sprintf("failed to create session: %v", err))
		return
	}

	log.Infof("session %s created for user %s, pod %s/%s", newSessionID, userInfo.UserName, namespace, podName)

	// 在后台 goroutine 中执行 pod exec
	go func() {
		defer func() {
			log.Infof("session %s exec completed, removing session", newSessionID)
			sessionMgr.RemoveSession(newSessionID)
		}()

		sessionMgr.MarkExecStarted(newSessionID)

		// 执行 pod exec
		err := ExecPod(execCtx.ClusterID, execCtx.Command, pty, execCtx.Namespace, execCtx.PodName, execCtx.ContainerName)

		// 🆕 标记 exec 已完成
		sessionMgr.MarkExecCompleted(newSessionID)

		if err != nil {
			msg := fmt.Sprintf("Exec to pod error! err: %v", err)
			log.Errorf("session %s: %s", newSessionID, msg)
			_, _ = pty.Write([]byte(msg))
			pty.Done()
		}
	}()

	// 主 goroutine 不再阻塞，直接返回
	ctx.RespErr = nil
}

func DebugWorkflow(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	logger := ctx.Logger
	taskID, err := strconv.ParseInt(c.Param("taskID"), 10, 64)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("无效 task ID")
		return
	}

	// 传递用户信息到下层函数
	c.Set("userId", ctx.UserID)
	c.Set("userName", ctx.UserName)

	ctx.RespErr = debugWorkflow(c, c.Param("workflowName"), c.Param("jobName"), taskID, logger)
}

func debugWorkflow(c *gin.Context, workflowName, jobName string, taskID int64, logger *zap.SugaredLogger) error {
	// 提取用户身份信息（从上层 DebugWorkflow 传递）
	userID := c.GetString("userId")
	userName := c.GetString("userName")

	userInfo := &UserInfo{
		UserID:   userID,
		UserName: userName,
	}

	// 检查是否为重连请求
	sessionID := c.Query("session_id")
	sessionMgr := GetSessionManager()

	if sessionID != "" {
		// 🔒 重连场景：验证用户身份
		if reconnectErr := sessionMgr.ReconnectSession(sessionID, c.Writer, c.Request, userInfo); reconnectErr == nil {
			log.Infof("debug session %s reconnected for user %s from %s", sessionID, userInfo.UserName, c.ClientIP())
			return nil
		} else {
			log.Warnf("failed to reconnect debug session %s for user %s: %v, creating new session", sessionID, userInfo.UserName, reconnectErr)
		}
	}

	// 新建会话场景
	workflowTask, err := commonrepo.NewworkflowTaskv4Coll().Find(workflowName, taskID)
	if err != nil {
		return e.ErrStopDebugShell.AddDesc(fmt.Sprintf("failed to find task: %s", err))
	}
	if workflowTask.Finished() {
		return e.ErrStopDebugShell.AddDesc("task has been finished")
	}

	var task *commonmodels.JobTask
FOR:
	for _, stage := range workflowTask.Stages {
		for _, jobTask := range stage.Jobs {
			if jobTask.Name == jobName {
				task = jobTask
				break FOR
			}
		}
	}
	if task == nil {
		logger.Error("debug workflow failed: not found job")
		return e.ErrInvalidParam.AddDesc("Job不存在")
	}
	log.Infof("DebugWorkflow: %s, %s, %d", workflowName, jobName, taskID)

	jobTaskSpec := &commonmodels.JobTaskFreestyleSpec{}
	if err := commonmodels.IToi(task.Spec, jobTaskSpec); err != nil {
		logger.Errorf("debug workflow failed: IToi %v", err)
		return e.ErrGetDebugShell.AddDesc("启动调试终端意外失败")
	}

	kubeClient, err := clientmanager.NewKubeClientManager().GetControllerRuntimeClient(jobTaskSpec.Properties.ClusterID)
	if err != nil {
		log.Errorf("debug workflow failed: get kube client error: %s", err)
		return e.ErrGetDebugShell.AddDesc("启动调试终端意外失败: get kube client")
	}

	pods, err := getter.ListPods(jobTaskSpec.Properties.Namespace, labels.Set{"job-name": task.K8sJobName}.AsSelector(), kubeClient)
	if err != nil {
		logger.Errorf("debug workflow failed: list pods %v", err)
		return e.ErrGetDebugShell.AddDesc("启动调试终端意外失败: ListPods")
	}
	if len(pods) == 0 {
		logger.Error("debug workflow failed: list pods num 0")
		return e.ErrGetDebugShell.AddDesc("启动调试终端意外失败: ListPods num 0")
	}
	pod := pods[0]
	switch pod.Status.Phase {
	case corev1.PodRunning:
	default:
		logger.Errorf("debug workflow failed: pod status is %s", pod.Status.Phase)
		return e.ErrGetDebugShell.AddDesc(fmt.Sprintf("Job 状态 %s 无法启动调试终端", pod.Status.Phase))
	}

	var envs []string
	for _, env := range jobTaskSpec.Properties.Envs {
		removeDquoteVal := strings.ReplaceAll(env.Value, `"`, `\"`)
		removeBquoteVal := strings.ReplaceAll(removeDquoteVal, "`", "\\`")
		envs = append(envs, fmt.Sprintf("%s=\"%s\"", env.Key, removeBquoteVal))
	}
	script := ""
	if len(envs) != 0 {
		script += "env " + strings.Join(envs, " ") + " "
	}
	script += "bash\n"

	// 创建会话上下文
	execCtx := &ExecContext{
		ClusterID:     jobTaskSpec.Properties.ClusterID,
		Namespace:     jobTaskSpec.Properties.Namespace,
		PodName:       pod.Name,
		ContainerName: pod.Spec.Containers[0].Name,
		Command:       []string{"/bin/sh", "-c", script},
	}

	// 创建会话选项
	sessionOpt := &TerminalSessionOption{
		SecretEnvs: func() (secrets []string) {
			for _, v := range jobTaskSpec.Properties.Envs {
				if v.IsCredential {
					secrets = append(secrets, v.Value)
				}
			}
			return secrets
		}(),
		Type: Workflow,
	}

	// 创建新会话（传入用户信息）
	newSessionID, pty, err := sessionMgr.CreateSession(c.Writer, c.Request, execCtx, sessionOpt, userInfo)
	if err != nil {
		log.Errorf("failed to create debug session: %v", err)
		return e.ErrGetDebugShell.AddDesc(fmt.Sprintf("failed to create session: %v", err))
	}

	log.Infof("debug session %s created for user %s, workflow %s/%s", newSessionID, userInfo.UserName, workflowName, jobName)

	// 在后台 goroutine 中执行 pod exec
	go func() {
		defer func() {
			log.Infof("debug session %s exec completed, removing session", newSessionID)
			sessionMgr.RemoveSession(newSessionID)
		}()

		sessionMgr.MarkExecStarted(newSessionID)

		// 执行带环境变量的 shell 命令
		err := ExecPod(execCtx.ClusterID, execCtx.Command, pty, execCtx.Namespace, execCtx.PodName, execCtx.ContainerName)

		// 🆕 标记 exec 已完成
		sessionMgr.MarkExecCompleted(newSessionID)

		if err != nil {
			msg := fmt.Sprintf("Exec to pod error! err: %v", err)
			log.Errorf("debug session %s: %s", newSessionID, msg)
			_, _ = pty.Write([]byte(msg))
			pty.Done()
		}
	}()

	return nil
}
