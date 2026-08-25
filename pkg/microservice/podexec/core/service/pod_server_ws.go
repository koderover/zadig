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
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	auditservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/terminalaudit"
	"github.com/koderover/zadig/v2/pkg/shared/terminalaudit"
	"github.com/koderover/zadig/v2/pkg/tool/clientmanager"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/setting"
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

	podName := c.Param("podName")
	containerName := c.Param("containerName")

	if podName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("containerName can't be empty,please check!")
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

	pty, err := NewTerminalSession(c.Writer, c.Request, nil)
	if err != nil {
		log.Errorf("get pty failed: %v", err)
		ctx.RespErr = e.ErrInternalError.AddDesc(fmt.Sprintf("get pty failed: %v", err))
		return
	}
	initialCols, initialRows := readTerminalSizeFromQuery(c)
	finalStatus := commonmodels.TerminalSessionStatusFinished
	var audit *auditservice.AuditSession
	defer func() {
		_ = pty.Close()
		if audit == nil {
			return
		}
		if err := audit.Close(finalStatus); err != nil {
			log.Errorf("close terminal audit recorder failed: %v", err)
		}
	}()

	kubeCli, err := clientmanager.NewKubeClientManager().GetKubernetesClientSet(clusterID)
	if err != nil {
		msg := fmt.Sprintf("get kubecli err :%v", err)
		log.Errorf(msg)
		_, _ = pty.Write([]byte(msg))

		ctx.RespErr = e.ErrInternalError.AddDesc(fmt.Sprintf("get kubecli err :%v", err))
		return
	}

	pod, err := ValidatePod(kubeCli, namespace, podName, containerName)
	if err != nil {
		msg := fmt.Sprintf("Validate pod error! err: %v", err)
		log.Errorf(msg)
		_, _ = pty.Write([]byte(msg))

		ctx.RespErr = e.ErrInternalError.AddDesc(fmt.Sprintf("Validate pod error! err: %v", err))
		return
	}
	secrets, secretErr := collectContainerSecretValues(c.Request.Context(), kubeCli, pod, namespace, containerName)
	if secretErr != nil {
		log.Warnf("collect pod secret values for terminal audit failed, continuing without audit: %v", secretErr)
	} else {
		meta := &auditservice.SessionMeta{
			SessionType:   commonmodels.TerminalSessionTypePodExec,
			Protocol:      "k8s-exec",
			UserID:        ctx.UserID,
			Username:      ctx.UserName,
			Account:       ctx.Account,
			ProjectName:   productName,
			EnvName:       envName,
			ServiceName:   pod.Labels[setting.ServiceLabel],
			TargetName:    fmt.Sprintf("%s/%s", podName, containerName),
			RemoteAddr:    pod.Status.PodIP,
			ClusterID:     clusterID,
			Namespace:     namespace,
			PodName:       podName,
			ContainerName: containerName,
			ClientIP:      c.ClientIP(),
			UserAgent:     c.Request.UserAgent(),
			InitialCols:   initialCols,
			InitialRows:   initialRows,
			Secrets:       secrets,
		}
		session, auditErr := auditservice.NewAuditSession(meta, func() {
			_ = pty.Close()
		})
		if auditErr != nil {
			log.Errorf("create podexec terminal audit recorder failed, continuing without audit: %v", auditErr)
		} else {
			audit = session
			log.Infof("created podexec terminal audit session, sessionID=%s project=%s env=%s pod=%s container=%s", audit.SessionID, productName, envName, podName, containerName)
			pty.attachAudit(audit.SessionID, audit)
		}
	}

	log.Infof("start pod exec stream, sessionID=%s clusterID=%s namespace=%s pod=%s container=%s", pty.sessionID, clusterID, namespace, podName, containerName)
	err = ExecPod(clusterID, []string{"/bin/sh"}, pty, namespace, podName, containerName)
	log.Infof("finish pod exec stream, sessionID=%s err=%v", pty.sessionID, err)
	if err == nil || isExpectedTerminalClose(err) {
		return
	}
	finalStatus = commonmodels.TerminalSessionStatusFailed
	msg := fmt.Sprintf("Exec to pod error! err: %v", err)
	log.Errorf(msg)
	_, _ = pty.Write([]byte(msg))

	ctx.RespErr = e.ErrInternalError.AddDesc(fmt.Sprintf("Exec to pod error! err: %v", err))
	return
}

func DebugWorkflow(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	logger := ctx.Logger
	taskID, err := strconv.ParseInt(c.Param("taskID"), 10, 64)
	if err != nil {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("无效 task ID")
		return
	}

	ctx.RespErr = debugWorkflow(c, ctx, c.Param("workflowName"), c.Param("jobName"), taskID, logger)
	return
}

func debugWorkflow(c *gin.Context, ctx *internalhandler.Context, workflowName, jobName string, taskID int64, logger *zap.SugaredLogger) error {
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

	var credValues []string
	for _, v := range jobTaskSpec.Properties.Envs {
		if v.IsCredential {
			credValues = append(credValues, v.Value)
		}
	}

	pty, err := NewTerminalSession(c.Writer, c.Request, nil)
	if err != nil {
		log.Errorf("get pty failed: %v", err)
		return e.ErrGetDebugShell.AddDesc(fmt.Sprintf("get pty failed: %v", err))
	}
	initialCols, initialRows := readTerminalSizeFromQuery(c)
	finalStatus := commonmodels.TerminalSessionStatusFinished
	var audit *auditservice.AuditSession
	defer func() {
		_ = pty.Close()
		if audit == nil {
			return
		}
		if err := audit.Close(finalStatus); err != nil {
			log.Errorf("close workflow terminal audit recorder failed: %v", err)
		}
	}()

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
	containerName := pod.Spec.Containers[0].Name

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

	// Browser-side credential masking must apply regardless of whether audit
	// recording is available.
	pty.outputSanitizer = terminalaudit.NewSanitizer(credValues)

	meta := &auditservice.SessionMeta{
		SessionType:   commonmodels.TerminalSessionTypeWorkflowDebug,
		Protocol:      "k8s-exec",
		UserID:        ctx.UserID,
		Username:      ctx.UserName,
		Account:       ctx.Account,
		ProjectName:   workflowTask.ProjectName,
		WorkflowName:  workflowName,
		JobName:       jobName,
		TaskID:        taskID,
		TargetName:    fmt.Sprintf("%s/%s", pod.Name, containerName),
		RemoteAddr:    pod.Status.PodIP,
		ClusterID:     jobTaskSpec.Properties.ClusterID,
		Namespace:     jobTaskSpec.Properties.Namespace,
		PodName:       pod.Name,
		ContainerName: containerName,
		ClientIP:      c.ClientIP(),
		UserAgent:     c.Request.UserAgent(),
		InitialCols:   initialCols,
		InitialRows:   initialRows,
		Secrets:       credValues,
	}
	session, auditErr := auditservice.NewAuditSession(meta, func() {
		_ = pty.Close()
	})
	if auditErr != nil {
		log.Errorf("create workflow terminal audit recorder failed, continuing without audit: %v", auditErr)
	} else {
		audit = session
		pty.attachAudit(audit.SessionID, audit)
	}

	err = ExecPod(jobTaskSpec.Properties.ClusterID, []string{"/bin/sh", "-c", script}, pty, jobTaskSpec.Properties.Namespace, pod.Name, containerName)
	if err == nil || isExpectedTerminalClose(err) {
		return nil
	}
	finalStatus = commonmodels.TerminalSessionStatusFailed
	msg := fmt.Sprintf("Exec to pod error! err: %v", err)
	log.Errorf(msg)
	_, _ = pty.Write([]byte(msg))

	return e.ErrGetDebugShell.AddDesc(fmt.Sprintf("Exec to pod error! err: %v", err))
}

func readTerminalSizeFromQuery(c *gin.Context) (int, int) {
	cols, _ := strconv.Atoi(c.Query("cols"))
	rows, _ := strconv.Atoi(c.Query("rows"))
	return cols, rows
}

func isExpectedTerminalClose(err error) bool {
	if errors.Is(err, io.EOF) || errors.Is(err, context.Canceled) || errors.Is(err, net.ErrClosed) || errors.Is(err, websocket.ErrCloseSent) {
		return true
	}
	var closeErr *websocket.CloseError
	if !errors.As(err, &closeErr) {
		return false
	}
	return closeErr.Code == websocket.CloseNormalClosure || closeErr.Code == websocket.CloseGoingAway
}

func collectContainerSecretValues(ctx context.Context, kubeCli kubernetes.Interface, pod *corev1.Pod, namespace, containerName string) ([]string, error) {
	var envFrom []corev1.EnvFromSource
	var envs []corev1.EnvVar
	for i := range pod.Spec.Containers {
		if pod.Spec.Containers[i].Name == containerName {
			envFrom, envs = pod.Spec.Containers[i].EnvFrom, pod.Spec.Containers[i].Env
			break
		}
	}
	if envFrom == nil && envs == nil {
		for i := range pod.Spec.EphemeralContainers {
			if pod.Spec.EphemeralContainers[i].Name == containerName {
				envFrom, envs = pod.Spec.EphemeralContainers[i].EnvFrom, pod.Spec.EphemeralContainers[i].Env
				break
			}
		}
	}

	type secretRequest struct {
		optional bool
		all      bool
		keys     map[string]struct{}
	}
	requests := make(map[string]*secretRequest)
	for _, source := range envFrom {
		if source.SecretRef == nil || source.SecretRef.Name == "" {
			continue
		}
		name := source.SecretRef.Name
		optional := source.SecretRef.Optional != nil && *source.SecretRef.Optional
		request, ok := requests[name]
		if !ok {
			request = &secretRequest{optional: optional}
			requests[name] = request
		} else if !optional {
			request.optional = false
		}
		request.all = true
	}
	for _, envVar := range envs {
		if envVar.ValueFrom == nil || envVar.ValueFrom.SecretKeyRef == nil {
			continue
		}
		ref := envVar.ValueFrom.SecretKeyRef
		if ref.Name == "" || ref.Key == "" {
			continue
		}
		optional := ref.Optional != nil && *ref.Optional
		request, ok := requests[ref.Name]
		if !ok {
			request = &secretRequest{optional: optional, keys: make(map[string]struct{})}
			requests[ref.Name] = request
		} else if !optional {
			request.optional = false
		}
		if request.keys == nil {
			request.keys = make(map[string]struct{})
		}
		request.keys[ref.Key] = struct{}{}
	}

	secretValues := make([]string, 0)
	for name, request := range requests {
		secret, err := kubeCli.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			if request.optional && apierrors.IsNotFound(err) {
				continue
			}
			return nil, fmt.Errorf("get secret %s: %w", name, err)
		}
		if request.all {
			for _, value := range secret.Data {
				if len(value) > 0 {
					secretValues = append(secretValues, string(value))
				}
			}
			continue
		}
		for key := range request.keys {
			if value := secret.Data[key]; len(value) > 0 {
				secretValues = append(secretValues, string(value))
			}
		}
	}
	return secretValues, nil
}
