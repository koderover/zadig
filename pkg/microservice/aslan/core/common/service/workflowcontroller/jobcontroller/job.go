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

package jobcontroller

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/instantmessage"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	workflowtool "github.com/koderover/zadig/v2/pkg/tool/workflow"
	"github.com/koderover/zadig/v2/pkg/util"
	"github.com/koderover/zadig/v2/pkg/util/rand"
)

type JobCtl interface {
	Run(ctx context.Context)
	// do some clean stuff when workflow finished, like collect reports or clean up resources.
	Clean(ctx context.Context)
	// SaveInfo is used to update the basic information of the job task to the mongoDB
	SaveInfo(ctx context.Context) error
}

var sendTaskNotifications = func(input *instantmessage.TaskNotifyInput) error {
	return instantmessage.NewWeChatClient().SendTaskNotifications(input)
}

type notificationRuntimeRenderFields struct {
	Title   string
	Content string

	LarkHookDynamic   commonmodels.DynamicRecipients
	LarkGroupDynamic  commonmodels.DynamicRecipients
	LarkPersonDynamic commonmodels.DynamicRecipients
	DingDingDynamic   commonmodels.DynamicRecipients
	MSTeamsDynamic    commonmodels.DynamicRecipients
	MailDynamic       commonmodels.DynamicRecipients
}

// Preserve notification templates while generic job rendering removes unresolved variables.
// NotificationJobCtl resolves them later with the runtime webhook/task context.
func backupNotificationRuntimeRenderFields(job *commonmodels.JobTask) (*notificationRuntimeRenderFields, error) {
	if job == nil || job.JobType != string(config.JobNotification) {
		return nil, nil
	}

	spec, err := decodeNotificationJobTaskSpec(job.Spec)
	if err != nil {
		return nil, err
	}

	resp := &notificationRuntimeRenderFields{
		Title:   spec.Title,
		Content: spec.Content,
	}

	if cfg := spec.LarkHookNotificationConfig; cfg != nil {
		resp.LarkHookDynamic = commonmodels.CloneDynamicRecipients(cfg.DynamicRecipients)
	}
	if cfg := spec.LarkGroupNotificationConfig; cfg != nil {
		resp.LarkGroupDynamic = commonmodels.CloneDynamicRecipients(cfg.DynamicRecipients)
	}
	if cfg := spec.LarkPersonNotificationConfig; cfg != nil {
		resp.LarkPersonDynamic = commonmodels.CloneDynamicRecipients(cfg.DynamicRecipients)
	}
	if cfg := spec.DingDingNotificationConfig; cfg != nil {
		resp.DingDingDynamic = commonmodels.CloneDynamicRecipients(cfg.DynamicRecipients)
	}
	if cfg := spec.MSTeamsNotificationConfig; cfg != nil {
		resp.MSTeamsDynamic = commonmodels.CloneDynamicRecipients(cfg.DynamicRecipients)
	}
	if cfg := spec.MailNotificationConfig; cfg != nil {
		resp.MailDynamic = commonmodels.CloneDynamicRecipients(cfg.DynamicRecipients)
	}

	return resp, nil
}

func restoreNotificationRuntimeRenderFields(job *commonmodels.JobTask, fields *notificationRuntimeRenderFields) (*commonmodels.JobTaskNotificationSpec, error) {
	if job == nil || fields == nil || job.JobType != string(config.JobNotification) {
		return nil, nil
	}

	spec, err := decodeNotificationJobTaskSpec(job.Spec)
	if err != nil {
		return nil, err
	}

	spec.Title = fields.Title
	spec.Content = fields.Content

	if cfg := spec.LarkHookNotificationConfig; cfg != nil {
		cfg.DynamicRecipients = commonmodels.CloneDynamicRecipients(fields.LarkHookDynamic)
	}
	if cfg := spec.LarkGroupNotificationConfig; cfg != nil {
		cfg.DynamicRecipients = commonmodels.CloneDynamicRecipients(fields.LarkGroupDynamic)
	}
	if cfg := spec.LarkPersonNotificationConfig; cfg != nil {
		cfg.DynamicRecipients = commonmodels.CloneDynamicRecipients(fields.LarkPersonDynamic)
	}
	if cfg := spec.DingDingNotificationConfig; cfg != nil {
		cfg.DynamicRecipients = commonmodels.CloneDynamicRecipients(fields.DingDingDynamic)
	}
	if cfg := spec.MSTeamsNotificationConfig; cfg != nil {
		cfg.DynamicRecipients = commonmodels.CloneDynamicRecipients(fields.MSTeamsDynamic)
	}
	if cfg := spec.MailNotificationConfig; cfg != nil {
		cfg.DynamicRecipients = commonmodels.CloneDynamicRecipients(fields.MailDynamic)
	}

	job.Spec = spec
	return spec, nil
}

func decodeNotificationJobTaskSpec(raw interface{}) (*commonmodels.JobTaskNotificationSpec, error) {
	if spec, ok := raw.(*commonmodels.JobTaskNotificationSpec); ok && spec != nil {
		return spec, nil
	}

	spec := &commonmodels.JobTaskNotificationSpec{}
	if err := commonmodels.IToi(raw, spec); err != nil {
		return nil, err
	}
	return spec, nil
}

func initJobCtl(job *commonmodels.JobTask, workflowCtx *commonmodels.WorkflowTaskCtx, logger *zap.SugaredLogger, ack func()) JobCtl {
	var jobCtl JobCtl
	switch job.JobType {
	case string(config.JobZadigDeploy):
		jobCtl = NewDeployJobCtl(job, workflowCtx, ack, logger)
	case string(config.JobZadigHelmDeploy):
		jobCtl = NewHelmDeployJobCtl(job, workflowCtx, ack, logger)
	case string(config.JobZadigHelmChartDeploy):
		jobCtl = NewHelmChartDeployJobCtl(job, workflowCtx, ack, logger)
	case string(config.JobZadigRestart):
		jobCtl = NewRestartJobCtl(job, workflowCtx, ack, logger)
	case string(config.JobCustomDeploy):
		jobCtl = NewCustomDeployJobCtl(job, workflowCtx, ack, logger)
	case string(config.JobPlugin):
		jobCtl = NewPluginsJobCtl(job, workflowCtx, ack, logger)
	case string(config.JobK8sCanaryDeploy):
		jobCtl = NewCanaryDeployJobCtl(job, workflowCtx, ack, logger)
	case string(config.JobK8sCanaryRelease):
		jobCtl = NewCanaryReleaseJobCtl(job, workflowCtx, ack, logger)
	case string(config.JobK8sBlueGreenDeploy):
		jobCtl = NewBlueGreenDeployV2JobCtl(job, workflowCtx, ack, logger)
	case string(config.JobK8sBlueGreenRelease):
		jobCtl = NewBlueGreenReleaseV2JobCtl(job, workflowCtx, ack, logger)
	case string(config.JobK8sGrayRelease):
		jobCtl = NewGrayReleaseJobCtl(job, workflowCtx, ack, logger)
	case string(config.JobK8sGrayRollback):
		jobCtl = NewGrayRollbackJobCtl(job, workflowCtx, ack, logger)
	case string(config.JobK8sPatch):
		jobCtl = NewK8sPatchJobCtl(job, workflowCtx, ack, logger)
	case string(config.JobIstioRelease):
		jobCtl = NewIstioReleaseJobCtl(job, workflowCtx, ack, logger)
	case string(config.JobIstioRollback):
		jobCtl = NewIstioRollbackJobCtl(job, workflowCtx, ack, logger)
	case string(config.JobUpdateEnvIstioConfig):
		jobCtl = NewUpdateEnvIstioConfigJobCtl(job, workflowCtx, ack, logger)
	case string(config.JobJira):
		jobCtl = NewJiraJobCtl(job, workflowCtx, ack, logger)
	case string(config.JobNacos):
		jobCtl = NewNacosJobCtl(job, workflowCtx, ack, logger)
	case string(config.JobPingCode):
		jobCtl = NewPingCodeJobCtl(job, workflowCtx, ack, logger)
	case string(config.JobTapd):
		jobCtl = NewTapdJobCtl(job, workflowCtx, ack, logger)
	case string(config.JobApollo):
		jobCtl = NewApolloJobCtl(job, workflowCtx, ack, logger)
	case string(config.JobMeegoTransition):
		jobCtl = NewMeegoTransitionJobCtl(job, workflowCtx, ack, logger)
	case string(config.JobWorkflowTrigger):
		jobCtl = NewWorkflowTriggerJobCtl(job, workflowCtx, ack, logger)
	case string(config.JobOfflineService):
		jobCtl = NewOfflineServiceJobCtl(job, workflowCtx, ack, logger)
	case string(config.JobMseGrayRelease):
		jobCtl = NewMseGrayReleaseJobCtl(job, workflowCtx, ack, logger)
	case string(config.JobMseGrayOffline):
		jobCtl = NewMseGrayOfflineJobCtl(job, workflowCtx, ack, logger)
	case string(config.JobGuanceyunCheck):
		jobCtl = NewGuanceyunCheckJobCtl(job, workflowCtx, ack, logger)
	case string(config.JobGrafana):
		jobCtl = NewGrafanaJobCtl(job, workflowCtx, ack, logger)
	case string(config.JobJenkins):
		jobCtl = NewJenkinsJobCtl(job, workflowCtx, ack, logger)
	case string(config.JobSQL):
		jobCtl = NewSQLJobCtl(job, workflowCtx, ack, logger)
	case string(config.JobBlueKing):
		jobCtl = NewBlueKingJobCtl(job, workflowCtx, ack, logger)
	case string(config.JobApproval):
		jobCtl = NewApprovalJobCtl(job, workflowCtx, ack, logger)
	case string(config.JobNotification):
		jobCtl = NewNotificationJobCtl(job, workflowCtx, ack, logger)
	case string(config.JobSAEDeploy):
		jobCtl = NewSAEDeployJobCtl(job, workflowCtx, ack, logger)
	case string(config.JobApisix):
		jobCtl = NewApisixJobCtl(job, workflowCtx, ack, logger)
	case string(config.JobDMS):
		jobCtl = NewDMSJobCtl(job, workflowCtx, ack, logger)
	default:
		jobCtl = NewFreestyleJobCtl(job, workflowCtx, ack, logger)
	}
	return jobCtl
}

func runJob(ctx context.Context, job *commonmodels.JobTask, workflowCtx *commonmodels.WorkflowTaskCtx, logger *zap.SugaredLogger, ack func()) {
	jobCtl := initJobCtl(job, workflowCtx, logger, ack)
	defer func(jobInfo *JobCtl) {
		if err := recover(); err != nil {
			errMsg := fmt.Sprintf("job: %s panic: %v", job.Name, err)
			logger.Errorf(errMsg)
			debug.PrintStack()
			job.Status = config.StatusFailed
			job.Error = errMsg
			setJobFinalStatusContext(job, workflowCtx)
		}
		job.EndTime = time.Now().Unix()
		logger.Infof("finish job: %s,status: %s", job.Name, job.Status)
		setJobFinalStatusContext(job, workflowCtx)
		ack()
		sendJobNotifications(workflowCtx, job, job.Status, logger)
		if workflowCtx.IsDebug {
			logger.Infof("skip updating debug job info into db")
			return
		}
		logger.Infof("updating job info into db...")
		err := jobCtl.SaveInfo(ctx)
		if err != nil {
			logger.Errorf("update job info: %s into db error: %v", err)
		}
	}(&jobCtl)

	setJobStartTimeContext(job, workflowCtx)

	// should skip passed job when workflow task be restarted
	if job.Status == config.StatusPassed || job.Status == config.StatusSkipped {
		return
	}
	// @note render global variables for every job.
	workflowCtx.GlobalContextEach(func(k, v string) bool {
		b, _ := json.Marshal(job)
		v = strings.Trim(v, "\n")

		jsonEscapeValue, err := util.JsonEscapeString(string(v))
		if err != nil {
			logger.Errorf("failed to escape value %s, error: %v", string(v), err)
			jsonEscapeValue = string(v)
		}

		replacedString := strings.ReplaceAll(string(b), k, jsonEscapeValue)
		if err := json.Unmarshal([]byte(replacedString), &job); err != nil {
			logger.Errorf("unmarshal job error: %v", err)
		}
		return true
	})

	notificationFields, err := backupNotificationRuntimeRenderFields(job)
	if err != nil {
		logger.Errorf("backup notification runtime fields error: %v", err)
		job.Status = config.StatusFailed
		job.Error = err.Error()
		return
	}

	// remove all the unrendered variable, replacing then with empty string
	b, _ := json.Marshal(job)
	variableRegexp := regexp.MustCompile(config.VariableRegEx)
	replacedJob := variableRegexp.ReplaceAll(b, []byte(""))
	if err := json.Unmarshal([]byte(replacedJob), &job); err != nil {
		logger.Errorf("unmarshal job error: %v", err)
		job.Status = config.StatusFailed
		job.Error = err.Error()
		return
	}
	if restoredSpec, err := restoreNotificationRuntimeRenderFields(job, notificationFields); err != nil {
		logger.Errorf("restore notification runtime fields error: %v", err)
		job.Status = config.StatusFailed
		job.Error = err.Error()
		return
	} else if restoredSpec != nil {
		if ctl, ok := jobCtl.(*NotificationJobCtl); ok {
			ctl.jobTaskSpec = restoredSpec
			ctl.job.Spec = restoredSpec
		}
	}

	// Check execute policy before running the job
	if !shouldExecuteJob(job) {
		logger.Infof("skipping job: %s due to execute policy", job.Name)
		job.Status = config.StatusSkipped
		job.StartTime = time.Now().Unix()
		job.EndTime = time.Now().Unix()
		ack()
		return
	}

	job.Status = config.StatusPrepare
	job.StartTime = time.Now().Unix()
	job.K8sJobName = getJobName(workflowCtx.WorkflowName, workflowCtx.TaskID)
	ack()

	sendJobNotifications(workflowCtx, job, config.StatusPrepare, logger)

	logger.Infof("start job: %s,status: %s", job.Name, job.Status)

	jobCtl.Run(ctx)

	// if the job is in a failed state, do the error handling policy
	if (job.Status == config.StatusFailed || job.Status == config.StatusTimeout) && job.ErrorPolicy != nil {
		switch job.ErrorPolicy.Policy {
		case config.JobErrorPolicyStop:
			return
		case config.JobErrorPolicyIgnoreError:
			job.Status = config.StatusUnstable
		case config.JobErrorPolicyRetry:
			retryJob(ctx, workflowCtx.WorkflowName, workflowCtx.TaskID, job, jobCtl, ack, job.ErrorPolicy.MaximumRetry)
		case config.JobErrorPolicyManualCheck:
			waitForManualErrorHandling(ctx, workflowCtx, job, ack, logger)
		}
	}
}

func sendJobNotifications(workflowCtx *commonmodels.WorkflowTaskCtx, job *commonmodels.JobTask, status config.Status, logger *zap.SugaredLogger) {
	if workflowCtx == nil || job == nil || !instantmessage.HasTaskNotifyCtls(job.NotifyCtls, status) {
		return
	}

	statusTextKeyOverride := ""
	if status == config.StatusPrepare {
		statusTextKeyOverride = "taskStatusExecutionStarted"
	}

	if err := sendTaskNotifications(&instantmessage.TaskNotifyInput{
		WorkflowName:          workflowCtx.WorkflowName,
		TaskID:                workflowCtx.TaskID,
		Job:                   job,
		NotifyCtls:            job.NotifyCtls,
		Status:                status,
		StatusTextKeyOverride: statusTextKeyOverride,
	}); err != nil {
		logger.Warnf("send task notification failed, job: %s, status: %s, error: %v", job.Name, status, err)
	}
}

func retryJob(ctx context.Context, workflowName string, taskID int64, job *commonmodels.JobTask, jobCtl JobCtl, ack func(), maxRetry int) {
	retryCount := 1

retryLoop:
	for retryCount <= maxRetry {
		select {
		case <-ctx.Done():
			job.Status = config.StatusCancelled
			job.Error = fmt.Sprintf("controller shutdown, marking job as cancelled.")
			return
		default:
			time.Sleep(10 * time.Second)
			job.RetryCount = retryCount
			job.Status = config.StatusPrepare
			job.StartTime = time.Now().Unix()
			job.K8sJobName = getJobName(workflowName, taskID)
			ack()

			jobCtl.Run(ctx)

			if job.Status == config.StatusPassed {
				break retryLoop
			}

			retryCount++
		}
	}
}

func waitForManualErrorHandling(ctx context.Context, workflowCtx *commonmodels.WorkflowTaskCtx, job *commonmodels.JobTask, ack func(), logger *zap.SugaredLogger) {
	originalStatus := job.Status
	job.Status = config.StatusManualApproval
	ack()
	sendJobNotifications(workflowCtx, job, config.StatusWaitingApprove, logger)

	for {
		time.Sleep(1 * time.Second)
		select {
		case <-ctx.Done():
			job.Status = config.StatusCancelled
			job.Error = fmt.Sprintf("controller shutdown, marking job as cancelled.")
			return
		default:
			decision, userID, username, err := workflowtool.GetJobErrorHandlingDecision(workflowCtx.WorkflowName, job.Name, workflowCtx.TaskID)
			if err != nil {
				continue
			}

			switch decision {
			case workflowtool.JobErrorDecisionIgnore:
				job.Status = config.StatusUnstable
				job.ErrorHandlerUserID = userID
				job.ErrorHandlerUserName = username
				ack()
				return
			case workflowtool.JobErrorDecisionReject:
				job.Status = originalStatus
				job.ErrorHandlerUserID = userID
				job.ErrorHandlerUserName = username
				ack()
				return
			default:
				continue
			}
		}
	}
}

func RunJobs(ctx context.Context, jobs []*commonmodels.JobTask, workflowCtx *commonmodels.WorkflowTaskCtx, concurrency int, logger *zap.SugaredLogger, ack func()) {
	if concurrency == 1 {
		for _, job := range jobs {
			runJob(ctx, job, workflowCtx, logger, ack)
			if jobStatusFailed(job.Status) {
				return
			}
		}
		return
	}
	jobPool := NewPool(ctx, jobs, workflowCtx, concurrency, logger, ack)
	jobPool.Run()
}

func CleanWorkflowJobs(ctx context.Context, workflowTask *commonmodels.WorkflowTask, workflowCtx *commonmodels.WorkflowTaskCtx, logger *zap.SugaredLogger, ack func()) {
	for _, stage := range workflowTask.Stages {
		for _, job := range stage.Jobs {
			if workflowTask.Status == config.StatusPause && job.JobType == string(config.JobK8sBlueGreenRelease) && job.Status == "" {
				continue
			}

			jobCtl := initJobCtl(job, workflowCtx, logger, ack)
			jobCtl.Clean(ctx)
		}
	}
}

func CleanPendingBlueGreenReleaseJobs(ctx context.Context, workflowTask *commonmodels.WorkflowTask, workflowCtx *commonmodels.WorkflowTaskCtx, logger *zap.SugaredLogger, ack func()) {
	for _, stage := range workflowTask.Stages {
		for _, job := range stage.Jobs {
			if job.JobType != string(config.JobK8sBlueGreenRelease) || job.Status != "" {
				continue
			}

			jobCtl := initJobCtl(job, workflowCtx, logger, ack)
			jobCtl.Clean(ctx)
		}
	}
}

// Pool is a worker group that runs a number of tasks at a
// configured concurrency.
type Pool struct {
	Jobs        []*commonmodels.JobTask
	workflowCtx *commonmodels.WorkflowTaskCtx
	concurrency int
	jobsChan    chan *commonmodels.JobTask
	logger      *zap.SugaredLogger
	ack         func()
	ctx         context.Context
	wg          sync.WaitGroup
}

// NewPool initializes a new pool with the given tasks and
// at the given concurrency.
func NewPool(ctx context.Context, jobs []*commonmodels.JobTask, workflowCtx *commonmodels.WorkflowTaskCtx, concurrency int, logger *zap.SugaredLogger, ack func()) *Pool {
	return &Pool{
		Jobs:        jobs,
		concurrency: concurrency,
		workflowCtx: workflowCtx,
		jobsChan:    make(chan *commonmodels.JobTask),
		logger:      logger,
		ack:         ack,
		ctx:         ctx,
	}
}

// Run runs all job within the pool and blocks until it's
// finished.
func (p *Pool) Run() {
	for i := 0; i < p.concurrency; i++ {
		go p.work()
	}

	p.wg.Add(len(p.Jobs))
	for _, task := range p.Jobs {
		p.jobsChan <- task
	}

	// all workers return
	close(p.jobsChan)

	p.wg.Wait()
}

// The work loop for any single goroutine.
func (p *Pool) work() {
	for job := range p.jobsChan {
		runJob(p.ctx, job, p.workflowCtx, p.logger, p.ack)
		p.wg.Done()
	}
}

func saveFile(src io.Reader, localFile string) error {
	out, err := os.Create(localFile)
	if err != nil {
		return err
	}

	defer out.Close()

	_, err = io.Copy(out, src)
	return err
}

func getJobName(workflowName string, taskID int64) string {
	// max lenth of workflowName was 32, so job name was unique in one task.
	base := strings.Replace(
		strings.ToLower(
			fmt.Sprintf(
				"%s-%d-",
				workflowName,
				taskID,
			),
		),
		"_", "-", -1,
	)
	return rand.GenerateName(base)
}

func jobStatusFailed(status config.Status) bool {
	if status == config.StatusCancelled || status == config.StatusFailed || status == config.StatusTimeout || status == config.StatusReject {
		return true
	}
	return false
}

func logError(job *commonmodels.JobTask, msg string, logger *zap.SugaredLogger) {
	logger.Error(msg)
	job.Status = config.StatusFailed
	job.Error = msg
}

func getMatchedRegistries(image string, registries []*commonmodels.RegistryNamespace) []*commonmodels.RegistryNamespace {
	resp := []*commonmodels.RegistryNamespace{}
	for _, registry := range registries {
		registryPrefix := registry.RegAddr
		if len(registry.Namespace) > 0 {
			registryPrefix = fmt.Sprintf("%s/%s", registry.RegAddr, registry.Namespace)
		}
		registryPrefix = strings.TrimPrefix(registryPrefix, "http://")
		registryPrefix = strings.TrimPrefix(registryPrefix, "https://")
		if strings.HasPrefix(image, registryPrefix) {
			resp = append(resp, registry)
		}
	}
	return resp
}

// evaluateExecuteRule evaluates a single execute rule against the global context
func evaluateExecuteRule(rule *commonmodels.JobExecuteRule) bool {
	ruleValue := rule.Value
	value := rule.Field

	log.Infof("value: %s", value)
	log.Infof("ruleValue: %s", ruleValue)

	switch rule.Verb {
	case string(config.ApplicationFilterActionEq):
		return value == ruleValue
	case string(config.ApplicationFilterActionNe):
		return value != ruleValue
	case string(config.ApplicationFilterActionBeginsWith):
		return strings.HasPrefix(value, ruleValue)
	case string(config.ApplicationFilterActionNotBeginsWith):
		return !strings.HasPrefix(value, ruleValue)
	case string(config.ApplicationFilterActionEndsWith):
		return strings.HasSuffix(value, ruleValue)
	case string(config.ApplicationFilterActionNotEndsWith):
		return !strings.HasSuffix(value, ruleValue)
	case string(config.ApplicationFilterActionContains):
		return strings.Contains(value, ruleValue)
	case string(config.ApplicationFilterActionNotContains):
		return !strings.Contains(value, ruleValue)
	default:
		return false
	}
}

// setJobStartTimeContext sets the global context variable for job start time
// Format: .job.<jobKey>.util.startTime
func setJobStartTimeContext(job *commonmodels.JobTask, workflowCtx *commonmodels.WorkflowTaskCtx) {
	startTimeStr := fmt.Sprintf("%d", job.StartTime)
	contextKey := fmt.Sprintf("{{.job.%s.util.startTime}}", job.Key)
	workflowCtx.GlobalContextSet(contextKey, startTimeStr)
}

// setJobStatusContext sets the global context variable for job status
// Format: .job.<jobKey>.status
func setJobFinalStatusContext(job *commonmodels.JobTask, workflowCtx *commonmodels.WorkflowTaskCtx) {
	statusStr := string(job.Status)
	contextKey := fmt.Sprintf("{{.job.%s.status}}", job.Key)
	workflowCtx.GlobalContextSet(contextKey, statusStr)
}

// shouldExecuteJob determines whether a job should be executed based on its execute policy
func shouldExecuteJob(job *commonmodels.JobTask) bool {
	if job.ExecutePolicy == nil || len(job.ExecutePolicy.Rules) == 0 {
		// No execute policy means the job should run
		return true
	}

	var rulesMatch bool

	matchRule := job.ExecutePolicy.MatchRule
	if matchRule == "" {
		matchRule = config.JobExecutePolicyMatchRuleAll
	}

	switch matchRule {
	case config.JobExecutePolicyMatchRuleAny:
		rulesMatch = false
		for _, rule := range job.ExecutePolicy.Rules {
			if evaluateExecuteRule(rule) {
				rulesMatch = true
				break
			}
		}
	case config.JobExecutePolicyMatchRuleAll:
		fallthrough
	default:
		rulesMatch = true
		for _, rule := range job.ExecutePolicy.Rules {
			if !evaluateExecuteRule(rule) {
				rulesMatch = false
				break
			}
		}
	}

	if job.ExecutePolicy.Type == config.JobExecutePolicyTypeSkip {
		return !rulesMatch
	} else if job.ExecutePolicy.Type == config.JobExecutePolicyTypeExecute {
		return rulesMatch
	}

	return true
}
