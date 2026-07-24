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
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	terminalaudit "github.com/koderover/zadig/v2/pkg/shared/terminalaudit"
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
	if productName == "" {
		productName = c.Param("productName")
	}
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
	defer func() {
		_ = pty.Close()
	}()
	initialCols, initialRows := readTerminalSizeFromQuery(c)
	finalStatus := commonmodels.TerminalSessionStatusFinished
	var audit *terminalaudit.AuditSession
	defer func() {
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
	secrets, err := collectContainerSecretValues(c.Request.Context(), kubeCli, pod, namespace, containerName)
	if err != nil {
		msg := fmt.Sprintf("collect pod secret values for terminal audit failed: %v", err)
		log.Errorf(msg)
		_, _ = pty.Write([]byte(msg))
		ctx.RespErr = e.ErrInternalError.AddDesc(msg)
		return
	}

	meta := &terminalaudit.SessionMeta{
		SessionType: commonmodels.TerminalSessionTypePodExec,
		Protocol:    "k8s-exec",
		UserID:      ctx.UserID,
		Username:    ctx.UserName,
		Account:     ctx.Account,
		ProjectName: productName,
		EnvName:     envName,
		ServiceName: resolvePodServiceName(pod),
		TargetName:  fmt.Sprintf("%s/%s", podName, containerName),
		RemoteAddr: func() string {
			if pod != nil {
				return pod.Status.PodIP
			}
			return ""
		}(),
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
	audit, err = terminalaudit.NewAuditSession(meta, func() {
		_ = pty.Close()
	})
	if err != nil {
		log.Errorf("create podexec terminal audit recorder failed: %v", err)
		ctx.RespErr = e.ErrInternalError.AddDesc(fmt.Sprintf("create terminal audit session failed: %v", err))
		return
	}
	log.Infof("created podexec terminal audit session, sessionID=%s project=%s env=%s pod=%s container=%s", audit.SessionID, productName, envName, podName, containerName)
	pty.SetupAudit(audit)

	log.Infof("start pod exec stream, sessionID=%s clusterID=%s namespace=%s pod=%s container=%s", pty.SessionID, clusterID, namespace, podName, containerName)
	err = ExecPod(clusterID, []string{"/bin/sh"}, pty, namespace, podName, containerName)
	log.Infof("finish pod exec stream, sessionID=%s err=%v", pty.SessionID, err)
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

	credValues := func() (secrets []string) {
		for _, v := range jobTaskSpec.Properties.Envs {
			if v.IsCredential {
				secrets = append(secrets, v.Value)
			}
		}
		return secrets
	}()

	pty, err := NewTerminalSession(c.Writer, c.Request, nil)
	if err != nil {
		log.Errorf("get pty failed: %v", err)
		return e.ErrGetDebugShell.AddDesc(fmt.Sprintf("get pty failed: %v", err))
	}
	initialCols, initialRows := readTerminalSizeFromQuery(c)
	finalStatus := commonmodels.TerminalSessionStatusFinished
	var audit *terminalaudit.AuditSession
	defer func() {
		if err := audit.Close(finalStatus); err != nil {
			log.Errorf("close workflow terminal audit recorder failed: %v", err)
		}
		_ = pty.Close()
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

	meta := &terminalaudit.SessionMeta{
		SessionType:   commonmodels.TerminalSessionTypeWorkflowDebug,
		Protocol:      "k8s-exec",
		UserID:        ctx.UserID,
		Username:      ctx.UserName,
		Account:       ctx.Account,
		ProjectName:   workflowTask.ProjectName,
		WorkflowName:  workflowName,
		JobName:       jobName,
		TaskID:        taskID,
		TargetName:    fmt.Sprintf("%s/%s", pod.Name, pod.Spec.Containers[0].Name),
		RemoteAddr:    pod.Status.PodIP,
		ClusterID:     jobTaskSpec.Properties.ClusterID,
		Namespace:     jobTaskSpec.Properties.Namespace,
		PodName:       pod.Name,
		ContainerName: pod.Spec.Containers[0].Name,
		ClientIP:      c.ClientIP(),
		UserAgent:     c.Request.UserAgent(),
		InitialCols:   initialCols,
		InitialRows:   initialRows,
		Secrets:       credValues,
	}
	audit, err = terminalaudit.NewAuditSession(meta, func() {
		_ = pty.Close()
	})
	if err != nil {
		log.Errorf("create workflow terminal audit recorder failed: %v", err)
		return e.ErrGetDebugShell.AddDesc(fmt.Sprintf("create terminal audit session failed: %v", err))
	}
	pty.SetupAudit(audit)
	pty.OutputSanitizer = terminalaudit.NewSanitizer(credValues, nil)

	err = ExecPod(jobTaskSpec.Properties.ClusterID, []string{"/bin/sh", "-c", script}, pty, jobTaskSpec.Properties.Namespace, pod.Name, pod.Spec.Containers[0].Name)
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
	cols := 135
	rows := 40
	if value, err := strconv.Atoi(c.Query("cols")); err == nil && value > 0 {
		cols = value
	}
	if value, err := strconv.Atoi(c.Query("rows")); err == nil && value > 0 {
		rows = value
	}
	return cols, rows
}

func isExpectedTerminalClose(err error) bool {
	if errors.Is(err, io.EOF) {
		return true
	}
	errText := strings.ToLower(err.Error())
	return strings.Contains(errText, "websocket: close") ||
		strings.Contains(errText, "close sent") ||
		strings.Contains(errText, "use of closed network connection") ||
		strings.Contains(errText, "next reader") ||
		strings.Contains(errText, "eof")
}

func collectContainerSecretValues(ctx context.Context, kubeCli kubernetes.Interface, pod *corev1.Pod, namespace, containerName string) ([]string, error) {
	if kubeCli == nil || pod == nil {
		return nil, nil
	}
	envFrom, envs, found := findContainerSecretRefs(pod, containerName)
	if !found {
		return nil, nil
	}

	secretValues := make([]string, 0)
	secretNames := make(map[string]bool)
	for _, envFromSource := range envFrom {
		if envFromSource.SecretRef != nil && envFromSource.SecretRef.Name != "" {
			optional := optionalBool(envFromSource.SecretRef.Optional)
			if existedOptional, ok := secretNames[envFromSource.SecretRef.Name]; !ok || existedOptional {
				secretNames[envFromSource.SecretRef.Name] = optional
			}
		}
	}
	for secretName, optional := range secretNames {
		secret, err := kubeCli.CoreV1().Secrets(namespace).Get(ctx, secretName, metav1.GetOptions{})
		if err != nil {
			if optional && apierrors.IsNotFound(err) {
				continue
			}
			return nil, fmt.Errorf("get secret %s: %w", secretName, err)
		}
		for _, value := range secret.Data {
			if len(value) > 0 {
				secretValues = append(secretValues, string(value))
			}
		}
	}

	for _, envVar := range envs {
		if envVar.ValueFrom == nil || envVar.ValueFrom.SecretKeyRef == nil {
			continue
		}
		ref := envVar.ValueFrom.SecretKeyRef
		if ref.Name == "" || ref.Key == "" {
			continue
		}
		secret, err := kubeCli.CoreV1().Secrets(namespace).Get(ctx, ref.Name, metav1.GetOptions{})
		if err != nil {
			if optionalBool(ref.Optional) && apierrors.IsNotFound(err) {
				continue
			}
			return nil, fmt.Errorf("get secret %s: %w", ref.Name, err)
		}
		value := secret.Data[ref.Key]
		if len(value) > 0 {
			secretValues = append(secretValues, string(value))
		}
	}
	return secretValues, nil
}

func findContainerSecretRefs(pod *corev1.Pod, containerName string) ([]corev1.EnvFromSource, []corev1.EnvVar, bool) {
	for i := range pod.Spec.Containers {
		if pod.Spec.Containers[i].Name == containerName {
			return pod.Spec.Containers[i].EnvFrom, pod.Spec.Containers[i].Env, true
		}
	}
	for i := range pod.Spec.EphemeralContainers {
		if pod.Spec.EphemeralContainers[i].Name == containerName {
			return pod.Spec.EphemeralContainers[i].EnvFrom, pod.Spec.EphemeralContainers[i].Env, true
		}
	}
	return nil, nil, false
}

func optionalBool(value *bool) bool {
	return value != nil && *value
}

func resolvePodServiceName(pod *corev1.Pod) string {
	if pod == nil || len(pod.Labels) == 0 {
		return ""
	}
	return pod.Labels[setting.ServiceLabel]
}
