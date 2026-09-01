package service

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	auditservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/terminalaudit"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	"github.com/koderover/zadig/v2/pkg/shared/terminalaudit"
	"github.com/koderover/zadig/v2/pkg/tool/clientmanager"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/kube/getter"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

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
	if pod.Status.Phase != corev1.PodRunning {
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
