/*
Copyright 2026 The KodeRover Authors.

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
	"errors"
	"fmt"
	"html"
	"math"
	"sort"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/llmservice"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/llm"
	runtimejob "github.com/koderover/zadig/v2/pkg/types/job"
	steptypes "github.com/koderover/zadig/v2/pkg/types/step"
)

const (
	aiReleaseSpecialistMaxPromptTokens = 12000
)

const defaultAIReleaseSpecialistSystemPrompt = `你是 Zadig 的 AI 发布专员，负责在人工审批前评估本次发布风险，并给出是否建议继续后续发布动作的结论。

任务语义说明：
- 代码扫描：表示静态检查或安全扫描结果；如果 scan_metrics 存在，应优先参考质量门禁、bug、漏洞、异味、覆盖率等结构化指标，否则只能依据任务状态和错误摘要判断。
- 构建：表示本次变更来源，可提供仓库、分支、tag、commit message、服务或模块信息，用于理解变更范围，不代表变更已验证通过。
- 测试：表示自动化验证结果；如果 test_statistics 存在，应优先参考总用例数、成功数、失败数、错误数、跳过数和通过率，否则只能依据任务状态和错误摘要判断。
- 发布专员：表示当前 AI 评估节点，需要汇总上下文并输出风险结论。
- 部署类任务：表示 AI 节点之前已经确定的发布目标环境、服务和版本信息；你只能依据输入中已经给出的发布目标判断，不要假设存在 AI 节点之后的发布动作。
- 如果输入中带有 sources 或 items 字段，这些字段表示对应上下文来自哪个上游任务；应优先结合其中的 job_name、job_type、status、summary 判断每条信息的来源和含义。

判断约束：
- 你只能依据输入的发布上下文做判断，不要虚构 PR 正文、代码 diff、日志全文、监控告警、集群实时状态或人工结论。
- 如果关键信息缺失，应明确指出缺失项，并优先给出 warning，而不是假设一切正常。
- 如果代码扫描或测试结果出现明确失败、超时、取消、错误摘要，或结构化指标显示质量门禁/通过率不满足额外关注点要求，通常应给出 fail。
- 如果发布目标中明确标记为生产环境，应使用更严格的风险判断标准；如果输入里没有给出生产发布目标，不要自行推断。
- remark、branch、tag、commit message 只能作为风险线索，不要据此臆测未提供的实现细节。

输出要求：
- 只输出一个 JSON 代码块，不要输出额外解释文字。
- JSON schema:
{
  "conclusion": "pass|warning|fail",
  "summary": "一句到三句中文总结",
  "checks": [
    {
      "name": "检查项名称",
      "result": "pass|warning|fail",
      "evidence": "判断依据，必须引用已提供的上下文字段或明确说明缺失项",
      "suggestion": "建议动作"
    }
  ]
}`

type AIReleaseSpecialistPromptDebugResult struct {
	SystemPrompt   string
	Prompt         string
	PromptTokens   int
	PromptTooLarge bool
}

type AIReleaseSpecialistJobCtl struct {
	job         *commonmodels.JobTask
	workflowCtx *commonmodels.WorkflowTaskCtx
	logger      *zap.SugaredLogger
	jobTaskSpec *commonmodels.JobTaskAIReleaseSpecialistSpec
	ack         func()
}

var (
	findWorkflowTaskForAIReleaseSpecialist = func(workflowName string, taskID int64) (*commonmodels.WorkflowTask, error) {
		return commonrepo.NewworkflowTaskv4Coll().Find(workflowName, taskID)
	}
	getAIReleaseSpecialistLLMClient    = llmservice.GetDefaultLLMClient
	getAIReleaseSpecialistConfirmUsers = func(users []*commonmodels.User, taskCreatorUserID string) []*commonmodels.User {
		flatUsers, _ := commonutil.GeneFlatUsersWithCaller(users, taskCreatorUserID)
		return flatUsers
	}
	waitForAIReleaseSpecialistApprove = waitForNativeApprove
)

func NewAIReleaseSpecialistJobCtl(job *commonmodels.JobTask, workflowCtx *commonmodels.WorkflowTaskCtx, ack func(), logger *zap.SugaredLogger) *AIReleaseSpecialistJobCtl {
	jobTaskSpec := &commonmodels.JobTaskAIReleaseSpecialistSpec{}
	if err := commonmodels.IToi(job.Spec, jobTaskSpec); err != nil {
		logger.Error(err)
	}
	job.Spec = jobTaskSpec
	return &AIReleaseSpecialistJobCtl{
		job:         job,
		workflowCtx: workflowCtx,
		logger:      logger,
		jobTaskSpec: jobTaskSpec,
		ack:         ack,
	}
}

func (c *AIReleaseSpecialistJobCtl) Clean(ctx context.Context) {}

func (c *AIReleaseSpecialistJobCtl) Run(ctx context.Context) {
	c.job.Status = config.StatusRunning
	c.ack()
	jobStartTime := time.Now()
	jobCtx := ctx
	cancel := func() {}
	if timeout := c.getJobTimeout(); timeout > 0 {
		jobCtx, cancel = context.WithTimeout(ctx, time.Duration(timeout)*time.Minute)
	}
	defer cancel()

	task, err := findWorkflowTaskForAIReleaseSpecialist(c.workflowCtx.WorkflowName, c.workflowCtx.TaskID)
	if err != nil {
		c.job.Status = config.StatusFailed
		c.job.Error = fmt.Sprintf("find workflow task failed: %v", err)
		c.ack()
		return
	}

	input, err := BuildAIReleaseSpecialistInputFromTask(task, c.job.Name)
	if err != nil {
		c.job.Status = config.StatusFailed
		c.job.Error = fmt.Sprintf("build ai release specialist input failed: %v", err)
		c.ack()
		return
	}
	c.jobTaskSpec.Input = input
	c.jobTaskSpec.SystemPrompt = GetEffectiveAIReleaseSpecialistSystemPrompt(c.jobTaskSpec.SystemPrompt)

	prompt, err := BuildAIReleaseSpecialistPrompt(c.jobTaskSpec.PromptTemplate, c.jobTaskSpec.SystemPrompt, input)
	if err != nil {
		c.job.Status = config.StatusFailed
		c.job.Error = fmt.Sprintf("build ai release specialist prompt failed: %v", err)
		c.ack()
		return
	}

	client, err := getAIReleaseSpecialistLLMClient(jobCtx)
	if err != nil {
		c.job.Status = config.StatusFailed
		c.job.Error = fmt.Sprintf("get default llm client failed: %v", err)
		c.ack()
		return
	}

	options := []llm.ParamOption{
		llm.WithTemperature(0.1),
		llm.WithMaxTokens(3000),
	}
	if client.GetModel() != "" {
		options = append(options, llm.WithModel(client.GetModel()))
	}

	answer, err := client.GetCompletion(jobCtx, prompt, options...)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(jobCtx.Err(), context.DeadlineExceeded) {
			c.job.Status = config.StatusTimeout
			c.job.Error = "ai release specialist timeout"
		} else {
			c.job.Status = config.StatusFailed
			c.job.Error = fmt.Sprintf("llm completion failed: %v", err)
		}
		c.ack()
		return
	}

	result, err := ParseAIReleaseSpecialistResult(answer)
	if err != nil {
		c.job.Status = config.StatusFailed
		c.job.Error = fmt.Sprintf("parse llm result failed: %v", err)
		c.ack()
		return
	}
	c.jobTaskSpec.Result = result
	c.jobTaskSpec.ChangeSummaryText = buildChangeSummaryText(input.ChangeSummary)
	if err := writeAIReleaseSpecialistOutputs(c.workflowCtx, c.job.Key, c.jobTaskSpec.Result); err != nil {
		c.logger.Warnf("marshal ai release specialist result failed: %v", err)
	}
	c.ack()

	if result.Conclusion == "fail" && !c.jobTaskSpec.RequireManualConfirm {
		c.job.Status = config.StatusFailed
		if result.Summary != "" {
			c.job.Error = result.Summary
		} else {
			c.job.Error = "ai release specialist check failed"
		}
		c.ack()
		return
	}

	if c.jobTaskSpec.RequireManualConfirm {
		approvalUsers, err := c.getRuntimeConfirmUsers()
		if err != nil {
			c.job.Status = config.StatusFailed
			c.job.Error = fmt.Sprintf("expand confirm users failed: %v", err)
			c.ack()
			return
		}
		remainingTimeout := c.getRemainingTimeout(jobStartTime)
		if remainingTimeout <= 0 {
			c.job.Status = config.StatusTimeout
			c.job.Error = "ai release specialist timeout"
			c.ack()
			return
		}
		approvalSpec := &commonmodels.JobTaskApprovalSpec{
			Timeout: remainingTimeout,
			Type:    config.NativeApproval,
			NativeApproval: &commonmodels.NativeApproval{
				ApproveUsers:    approvalUsers,
				NeededApprovers: 1,
				Timeout:         int(remainingTimeout),
			},
		}
		c.jobTaskSpec.NativeApproval = approvalSpec.NativeApproval
		c.job.Status = config.StatusWaitingApprove
		c.ack()

		status, err := waitForAIReleaseSpecialistApprove(jobCtx, approvalSpec, c.workflowCtx.WorkflowName, c.job.Name, c.workflowCtx.TaskID, c.ack)
		c.job.Status = status
		if err != nil {
			c.job.Error = err.Error()
		}
		return
	}

	c.job.Status = config.StatusPassed
}

func (c *AIReleaseSpecialistJobCtl) SaveInfo(ctx context.Context) error {
	return commonrepo.NewJobInfoColl().Create(ctx, &commonmodels.JobInfo{
		Type:                c.job.JobType,
		WorkflowName:        c.workflowCtx.WorkflowName,
		WorkflowDisplayName: c.workflowCtx.WorkflowDisplayName,
		TaskID:              c.workflowCtx.TaskID,
		ProductName:         c.workflowCtx.ProjectName,
		StartTime:           c.job.StartTime,
		EndTime:             c.job.EndTime,
		Duration:            c.job.EndTime - c.job.StartTime,
		Status:              string(c.job.Status),
	})
}

func (c *AIReleaseSpecialistJobCtl) getJobTimeout() int64 {
	if c.jobTaskSpec.Timeout > 0 {
		return c.jobTaskSpec.Timeout
	}
	return config.AIReleaseSpecialistDefaultTimeoutMinutes
}

func (c *AIReleaseSpecialistJobCtl) getRemainingTimeout(jobStartTime time.Time) int64 {
	remainingDuration := time.Duration(c.getJobTimeout())*time.Minute - time.Since(jobStartTime)
	if remainingDuration <= 0 {
		return 0
	}
	return int64(math.Ceil(remainingDuration.Minutes()))
}

func (c *AIReleaseSpecialistJobCtl) getRuntimeConfirmUsers() ([]*commonmodels.User, error) {
	flatUsers := getAIReleaseSpecialistConfirmUsers(c.jobTaskSpec.ConfirmUsers, c.workflowCtx.WorkflowTaskCreatorUserID)
	if len(flatUsers) == 0 {
		return nil, fmt.Errorf("confirm users are empty")
	}
	for _, user := range flatUsers {
		if user == nil {
			return nil, fmt.Errorf("confirm user cannot be nil")
		}
		user.Type = setting.UserTypeUser
		if user.UserID == "" {
			return nil, fmt.Errorf("confirm user id cannot be empty")
		}
	}
	return flatUsers, nil
}

func BuildAIReleaseSpecialistInputFromTask(task *commonmodels.WorkflowTask, currentJobName string) (*commonmodels.AIReleaseSpecialistInput, error) {
	input := &commonmodels.AIReleaseSpecialistInput{
		ChangeSummary: &commonmodels.AIChangeSummary{
			Remark: strings.TrimSpace(task.Remark),
		},
	}
	envMap := make(map[string]*commonmodels.Product)

	var (
		releaseTargets []*commonmodels.AIReleaseTargetsSummary
		scanStatuses   []string
		scanSummaries  []string
		scanItems      []*commonmodels.AIReleaseSummaryItem
		testStatuses   []string
		testSummaries  []string
		testItems      []*commonmodels.AIReleaseSummaryItem
	)

	for _, stage := range task.Stages {
		for _, job := range stage.Jobs {
			if job.Name == currentJobName {
				return finalizeAIReleaseSpecialistInput(input, releaseTargets, scanStatuses, scanSummaries, scanItems, testStatuses, testSummaries, testItems), nil
			}

			switch job.JobType {
			case string(config.JobZadigDeploy):
				spec := &commonmodels.JobTaskDeploySpec{}
				if err := commonmodels.IToi(job.Spec, spec); err != nil {
					continue
				}
				target := buildReleaseTargetFromDeploy(job, spec)
				fillReleaseTargetEnvAlias(task.ProjectName, target, envMap)
				releaseTargets = append(releaseTargets, target)
			case string(config.JobZadigHelmDeploy):
				spec := &commonmodels.JobTaskHelmDeploySpec{}
				if err := commonmodels.IToi(job.Spec, spec); err != nil {
					continue
				}
				target := buildReleaseTargetFromHelmDeploy(job, spec)
				fillReleaseTargetEnvAlias(task.ProjectName, target, envMap)
				releaseTargets = append(releaseTargets, target)
			case string(config.JobZadigHelmChartDeploy):
				spec := &commonmodels.JobTaskHelmChartDeploySpec{}
				if err := commonmodels.IToi(job.Spec, spec); err != nil {
					continue
				}
				target := buildReleaseTargetFromHelmChartDeploy(job, spec)
				fillReleaseTargetEnvAlias(task.ProjectName, target, envMap)
				releaseTargets = append(releaseTargets, target)
			case string(config.JobZadigBuild):
				spec := &commonmodels.JobTaskFreestyleSpec{}
				if err := commonmodels.IToi(job.Spec, spec); err != nil {
					continue
				}
				appendChangeSummarySource(input.ChangeSummary, job)
				collectChangeSummaryFromFreestyleSpec(input.ChangeSummary, spec)
			case string(config.JobZadigScanning):
				scanStatuses = append(scanStatuses, fmt.Sprintf("%s:%s", job.OriginName, job.Status))
				summary := buildResultSummaryLine(job)
				scanSummaries = append(scanSummaries, summary)
				item := buildReleaseSummaryItem(job, summary)
				item.ScanMetrics = buildAIScanMetricsFromJob(job)
				scanItems = append(scanItems, item)
			case string(config.JobZadigTesting):
				testStatuses = append(testStatuses, fmt.Sprintf("%s:%s", job.OriginName, job.Status))
				summary := buildResultSummaryLine(job)
				testSummaries = append(testSummaries, summary)
				testReports, err := commonrepo.NewCustomWorkflowTestReportColl().ListByWorkflowJobTaskName(task.WorkflowName, job.Name, task.TaskID)
				if err != nil {
					return nil, err
				}
				item := buildReleaseSummaryItem(job, summary)
				item.TestStatistics = buildAITestStatisticsFromReports(testReports)
				testItems = append(testItems, item)
			}
		}
	}

	return finalizeAIReleaseSpecialistInput(input, releaseTargets, scanStatuses, scanSummaries, scanItems, testStatuses, testSummaries, testItems), nil
}

func finalizeAIReleaseSpecialistInput(input *commonmodels.AIReleaseSpecialistInput, releaseTargets []*commonmodels.AIReleaseTargetsSummary, scanStatuses, scanSummaries []string, scanItems []*commonmodels.AIReleaseSummaryItem, testStatuses, testSummaries []string, testItems []*commonmodels.AIReleaseSummaryItem) *commonmodels.AIReleaseSpecialistInput {
	if len(releaseTargets) > 0 {
		input.ReleaseTargets = mergeReleaseTargets(releaseTargets)
	}
	if len(scanStatuses) > 0 || len(scanSummaries) > 0 || len(scanItems) > 0 {
		input.ScanSummary = &commonmodels.AIScanSummary{
			JobStatuses: uniqueSortedStrings(scanStatuses),
			Summaries:   uniquePreserveOrder(scanSummaries),
			Items:       uniqueReleaseSummaryItems(scanItems),
		}
	}
	if len(testStatuses) > 0 || len(testSummaries) > 0 || len(testItems) > 0 {
		input.TestSummary = &commonmodels.AITestSummary{
			JobStatuses: uniqueSortedStrings(testStatuses),
			Summaries:   uniquePreserveOrder(testSummaries),
			Items:       uniqueReleaseSummaryItems(testItems),
		}
	}
	input.ChangeSummary.Branches = uniqueSortedStrings(input.ChangeSummary.Branches)
	input.ChangeSummary.Tags = uniqueSortedStrings(input.ChangeSummary.Tags)
	input.ChangeSummary.Services = uniqueSortedStrings(input.ChangeSummary.Services)
	input.ChangeSummary.CommitMessages = uniquePreserveOrder(input.ChangeSummary.CommitMessages)
	input.ChangeSummary.Sources = uniqueReleaseContextSources(input.ChangeSummary.Sources)
	return input
}

func buildReleaseTargetFromDeploy(job *commonmodels.JobTask, spec *commonmodels.JobTaskDeploySpec) *commonmodels.AIReleaseTargetsSummary {
	target := &commonmodels.AIReleaseTargetsSummary{
		EnvName:    spec.Env,
		Production: spec.Production,
	}
	if spec.ServiceName != "" {
		target.ServiceNames = append(target.ServiceNames, spec.ServiceName)
	}
	if spec.ServiceModule != "" && spec.Image != "" {
		target.ImageVersions = append(target.ImageVersions, spec.Image)
		target.TargetCount++
	}
	for _, serviceAndImage := range spec.ServiceAndImages {
		if serviceAndImage.Image != "" {
			target.ImageVersions = append(target.ImageVersions, serviceAndImage.Image)
		}
		target.TargetCount++
	}
	target.ServiceNames = uniqueSortedStrings(target.ServiceNames)
	target.ImageVersions = uniquePreserveOrder(target.ImageVersions)
	if target.TargetCount == 0 && len(target.ServiceNames) > 0 {
		target.TargetCount = len(target.ServiceNames)
	}
	target.Items = append(target.Items, buildReleaseTargetItem(job, target))
	return target
}

func buildReleaseTargetFromHelmDeploy(job *commonmodels.JobTask, spec *commonmodels.JobTaskHelmDeploySpec) *commonmodels.AIReleaseTargetsSummary {
	target := &commonmodels.AIReleaseTargetsSummary{
		EnvName:    spec.Env,
		Production: spec.IsProduction,
	}
	if spec.ServiceName != "" {
		target.ServiceNames = append(target.ServiceNames, spec.ServiceName)
	}
	for _, imageAndModule := range spec.ImageAndModules {
		if imageAndModule.Image != "" {
			target.ImageVersions = append(target.ImageVersions, imageAndModule.Image)
		}
		target.TargetCount++
	}
	target.ServiceNames = uniqueSortedStrings(target.ServiceNames)
	target.ImageVersions = uniquePreserveOrder(target.ImageVersions)
	if target.TargetCount == 0 && len(target.ServiceNames) > 0 {
		target.TargetCount = len(target.ServiceNames)
	}
	target.Items = append(target.Items, buildReleaseTargetItem(job, target))
	return target
}

func buildReleaseTargetFromHelmChartDeploy(job *commonmodels.JobTask, spec *commonmodels.JobTaskHelmChartDeploySpec) *commonmodels.AIReleaseTargetsSummary {
	target := &commonmodels.AIReleaseTargetsSummary{
		EnvName: spec.Env,
	}
	if spec.DeployHelmChart != nil {
		target.TargetCount = 1
		if spec.DeployHelmChart.ReleaseName != "" {
			target.ServiceNames = append(target.ServiceNames, spec.DeployHelmChart.ReleaseName)
		}
		if spec.DeployHelmChart.ChartVersion != "" {
			target.ImageVersions = append(target.ImageVersions, spec.DeployHelmChart.ChartVersion)
		}
	}
	target.ServiceNames = uniqueSortedStrings(target.ServiceNames)
	target.ImageVersions = uniquePreserveOrder(target.ImageVersions)
	target.Items = append(target.Items, buildReleaseTargetItem(job, target))
	return target
}

func mergeReleaseTargets(targets []*commonmodels.AIReleaseTargetsSummary) *commonmodels.AIReleaseTargetsSummary {
	merged := &commonmodels.AIReleaseTargetsSummary{}
	for _, target := range targets {
		if target == nil {
			continue
		}
		if merged.EnvName == "" {
			merged.EnvName = target.EnvName
		}
		if merged.EnvAlias == "" {
			merged.EnvAlias = target.EnvAlias
		}
		if target.Production {
			merged.Production = true
		}
		merged.ServiceNames = append(merged.ServiceNames, target.ServiceNames...)
		merged.ImageVersions = append(merged.ImageVersions, target.ImageVersions...)
		merged.TargetCount += target.TargetCount
		merged.Items = append(merged.Items, target.Items...)
	}
	merged.ServiceNames = uniqueSortedStrings(merged.ServiceNames)
	merged.ImageVersions = uniquePreserveOrder(merged.ImageVersions)
	if merged.TargetCount == 0 {
		merged.TargetCount = len(merged.ServiceNames)
	}
	merged.Items = uniqueReleaseTargetItems(merged.Items)
	return merged
}

func fillReleaseTargetEnvAlias(projectName string, target *commonmodels.AIReleaseTargetsSummary, envMap map[string]*commonmodels.Product) {
	if target == nil || strings.TrimSpace(projectName) == "" || strings.TrimSpace(target.EnvName) == "" {
		return
	}
	target.EnvAlias = commonutil.GetEnvAlias(commonutil.GetEnvInfoNoErr(projectName, target.EnvName, envMap))
	for _, item := range target.Items {
		if item == nil {
			continue
		}
		item.EnvAlias = target.EnvAlias
	}
}

func appendChangeSummarySource(changeSummary *commonmodels.AIChangeSummary, job *commonmodels.JobTask) {
	if changeSummary == nil || job == nil {
		return
	}
	changeSummary.Sources = append(changeSummary.Sources, &commonmodels.AIReleaseContextSource{
		JobName: job.OriginName,
		JobType: job.JobType,
	})
}

func buildReleaseSummaryItem(job *commonmodels.JobTask, summary string) *commonmodels.AIReleaseSummaryItem {
	if job == nil {
		return nil
	}
	return &commonmodels.AIReleaseSummaryItem{
		JobName: job.OriginName,
		JobType: job.JobType,
		Status:  string(job.Status),
		Summary: summary,
	}
}

func buildAIScanMetricsFromJob(job *commonmodels.JobTask) *commonmodels.AIScanMetrics {
	if job == nil {
		return nil
	}
	spec := &commonmodels.JobTaskFreestyleSpec{}
	if err := commonmodels.IToi(job.Spec, spec); err != nil {
		return nil
	}
	for _, stepTask := range spec.Steps {
		if stepTask == nil || stepTask.StepType != config.StepSonarGetMetrics {
			continue
		}
		stepSpec := &steptypes.StepSonarGetMetricsSpec{}
		if err := commonmodels.IToi(stepTask.Spec, stepSpec); err != nil || stepSpec.SonarMetrics == nil {
			return nil
		}
		metrics := &commonmodels.AIScanMetrics{
			QualityGateStatus: string(stepSpec.SonarMetrics.QualityGateStatus),
			Ncloc:             strings.TrimSpace(stepSpec.SonarMetrics.Ncloc),
			Bugs:              strings.TrimSpace(stepSpec.SonarMetrics.Bugs),
			Vulnerabilities:   strings.TrimSpace(stepSpec.SonarMetrics.Vulnerabilities),
			CodeSmells:        strings.TrimSpace(stepSpec.SonarMetrics.CodeSmells),
			Coverage:          strings.TrimSpace(stepSpec.SonarMetrics.Coverage),
			CheckQualityGate:  stepSpec.CheckQualityGate,
		}
		if metrics.QualityGateStatus == "" && metrics.Ncloc == "" && metrics.Bugs == "" &&
			metrics.Vulnerabilities == "" && metrics.CodeSmells == "" && metrics.Coverage == "" {
			return nil
		}
		return metrics
	}
	return nil
}

func buildAITestStatisticsFromReports(reports []*commonmodels.CustomWorkflowTestReport) *commonmodels.AITestStatistics {
	if len(reports) == 0 {
		return nil
	}
	stats := &commonmodels.AITestStatistics{
		Reports: make([]*commonmodels.AITestReportSummary, 0, len(reports)),
	}
	for _, report := range reports {
		if report == nil {
			continue
		}
		reportSummary := &commonmodels.AITestReportSummary{
			JobTaskName:    report.JobTaskName,
			TestName:       report.TestName,
			ZadigTestName:  report.ZadigTestName,
			ServiceName:    report.ServiceName,
			ServiceModule:  report.ServiceModule,
			TestCaseNum:    report.TestCaseNum,
			SuccessCaseNum: report.SuccessCaseNum,
			SkipCaseNum:    report.SkipCaseNum,
			FailedCaseNum:  report.FailedCaseNum,
			ErrorCaseNum:   report.ErrorCaseNum,
			TestTime:       report.TestTime,
			PassRate:       buildAITestPassRate(report.SuccessCaseNum, report.TestCaseNum),
		}
		stats.TestCaseNum += report.TestCaseNum
		stats.SuccessCaseNum += report.SuccessCaseNum
		stats.SkipCaseNum += report.SkipCaseNum
		stats.FailedCaseNum += report.FailedCaseNum
		stats.ErrorCaseNum += report.ErrorCaseNum
		stats.Reports = append(stats.Reports, reportSummary)
	}
	if len(stats.Reports) == 0 {
		return nil
	}
	stats.PassRate = buildAITestPassRate(stats.SuccessCaseNum, stats.TestCaseNum)
	return stats
}

func buildAITestPassRate(successCaseNum, testCaseNum int) float64 {
	if testCaseNum <= 0 {
		return 0
	}
	return math.Round(float64(successCaseNum)/float64(testCaseNum)*10000) / 100
}

func buildReleaseTargetItem(job *commonmodels.JobTask, target *commonmodels.AIReleaseTargetsSummary) *commonmodels.AIReleaseTargetItem {
	if job == nil || target == nil {
		return nil
	}
	return &commonmodels.AIReleaseTargetItem{
		JobName:       job.OriginName,
		JobType:       job.JobType,
		EnvName:       target.EnvName,
		EnvAlias:      target.EnvAlias,
		Production:    target.Production,
		ServiceNames:  append([]string{}, target.ServiceNames...),
		ImageVersions: append([]string{}, target.ImageVersions...),
		TargetCount:   target.TargetCount,
	}
}

func uniqueReleaseContextSources(values []*commonmodels.AIReleaseContextSource) []*commonmodels.AIReleaseContextSource {
	seen := map[string]struct{}{}
	resp := make([]*commonmodels.AIReleaseContextSource, 0, len(values))
	for _, value := range values {
		if value == nil {
			continue
		}
		key := strings.TrimSpace(value.JobName) + "|" + strings.TrimSpace(value.JobType)
		if key == "|" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		resp = append(resp, value)
	}
	return resp
}

func uniqueReleaseSummaryItems(values []*commonmodels.AIReleaseSummaryItem) []*commonmodels.AIReleaseSummaryItem {
	seen := map[string]struct{}{}
	resp := make([]*commonmodels.AIReleaseSummaryItem, 0, len(values))
	for _, value := range values {
		if value == nil {
			continue
		}
		key := strings.TrimSpace(value.JobName) + "|" + strings.TrimSpace(value.JobType) + "|" + strings.TrimSpace(value.Status) + "|" + strings.TrimSpace(value.Summary)
		if key == "|||" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		resp = append(resp, value)
	}
	return resp
}

func uniqueReleaseTargetItems(values []*commonmodels.AIReleaseTargetItem) []*commonmodels.AIReleaseTargetItem {
	seen := map[string]struct{}{}
	resp := make([]*commonmodels.AIReleaseTargetItem, 0, len(values))
	for _, value := range values {
		if value == nil {
			continue
		}
		key := strings.TrimSpace(value.JobName) + "|" + strings.TrimSpace(value.JobType) + "|" + strings.TrimSpace(value.EnvName)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		resp = append(resp, value)
	}
	return resp
}

func collectChangeSummaryFromFreestyleSpec(changeSummary *commonmodels.AIChangeSummary, spec *commonmodels.JobTaskFreestyleSpec) {
	if changeSummary == nil || spec == nil {
		return
	}
	for _, step := range spec.Steps {
		if step.StepType != config.StepGit {
			continue
		}
		stepSpec := &steptypes.StepGitSpec{}
		if err := commonmodels.IToi(step.Spec, stepSpec); err != nil {
			continue
		}
		for _, repo := range stepSpec.Repos {
			if repo == nil {
				continue
			}
			if repo.Branch != "" {
				changeSummary.Branches = append(changeSummary.Branches, repo.Branch)
			}
			if repo.Tag != "" {
				changeSummary.Tags = append(changeSummary.Tags, repo.Tag)
			}
			if repo.CommitMessage != "" {
				changeSummary.CommitMessages = append(changeSummary.CommitMessages, compactSingleLine(repo.CommitMessage))
			}
			if repo.RepoName != "" {
				changeSummary.Services = append(changeSummary.Services, repo.RepoName)
			}
		}
	}
	for _, kv := range spec.Properties.Envs {
		switch kv.Key {
		case "SERVICE_NAME":
			if kv.Value != "" {
				changeSummary.Services = append(changeSummary.Services, kv.Value)
			}
		case "SERVICE_MODULE":
			if kv.Value != "" {
				changeSummary.Services = append(changeSummary.Services, kv.Value)
			}
		}
	}
}

func buildResultSummaryLine(job *commonmodels.JobTask) string {
	if strings.TrimSpace(job.Error) != "" {
		return fmt.Sprintf("%s(%s): %s", job.OriginName, job.Status, compactSingleLine(job.Error))
	}
	return fmt.Sprintf("%s(%s)", job.OriginName, job.Status)
}

func BuildAIReleaseSpecialistPrompt(promptTemplate, systemPrompt string, input *commonmodels.AIReleaseSpecialistInput) (string, error) {
	debugResult, err := BuildAIReleaseSpecialistPromptForDebug(promptTemplate, systemPrompt, input)
	if err != nil {
		return "", err
	}
	if debugResult.PromptTooLarge {
		return "", fmt.Errorf("prompt too large: %d tokens", debugResult.PromptTokens)
	}
	return debugResult.Prompt, nil
}

func BuildAIReleaseSpecialistPromptForDebug(promptTemplate, systemPromptOverride string, input *commonmodels.AIReleaseSpecialistInput) (*AIReleaseSpecialistPromptDebugResult, error) {
	inputJSON, err := json.MarshalIndent(input, "", "  ")
	if err != nil {
		return nil, err
	}
	systemPrompt := buildAIReleaseSpecialistSystemPrompt(systemPromptOverride)
	prompt := systemPrompt
	if strings.TrimSpace(promptTemplate) != "" {
		prompt = fmt.Sprintf("%s\n\n额外关注点：\n%s", prompt, strings.TrimSpace(promptTemplate))
	}
	prompt = fmt.Sprintf("%s\n\n发布上下文:\n```json\n%s\n```", prompt, string(inputJSON))
	promptTokens := getAIReleaseSpecialistPromptTokens(prompt)
	return &AIReleaseSpecialistPromptDebugResult{
		SystemPrompt:   systemPrompt,
		Prompt:         prompt,
		PromptTokens:   promptTokens,
		PromptTooLarge: promptTokens > aiReleaseSpecialistMaxPromptTokens,
	}, nil
}

func GetDefaultAIReleaseSpecialistSystemPrompt() string {
	return buildAIReleaseSpecialistSystemPrompt("")
}

func GetEffectiveAIReleaseSpecialistSystemPrompt(systemPrompt string) string {
	return buildAIReleaseSpecialistSystemPrompt(systemPrompt)
}

func buildAIReleaseSpecialistSystemPrompt(systemPromptOverride string) string {
	systemPrompt := strings.TrimSpace(systemPromptOverride)
	if systemPrompt == "" {
		systemPrompt = defaultAIReleaseSpecialistSystemPrompt
	}
	return systemPrompt
}

func getAIReleaseSpecialistPromptTokens(prompt string) int {
	tokenNum, err := llm.NumTokensFromPrompt(prompt, "")
	if err != nil {
		return 0
	}
	return tokenNum
}

func ParseAIReleaseSpecialistResult(answer string) (*commonmodels.AIReleaseSpecialistResult, error) {
	rawText := strings.TrimSpace(answer)
	jsonText := extractJSONCodeBlock(rawText)
	result := &commonmodels.AIReleaseSpecialistResult{}
	if err := json.Unmarshal([]byte(jsonText), result); err != nil {
		return nil, err
	}
	result.Conclusion = normalizeAIResultValue(result.Conclusion)
	for _, check := range result.Checks {
		if check == nil {
			continue
		}
		check.Result = normalizeAIResultValue(check.Result)
		check.Name = strings.TrimSpace(check.Name)
		check.Evidence = strings.TrimSpace(check.Evidence)
		check.Suggestion = strings.TrimSpace(check.Suggestion)
	}
	result.Summary = strings.TrimSpace(result.Summary)
	result.RawText = rawText
	if result.Conclusion == "" {
		return nil, fmt.Errorf("empty conclusion")
	}
	if !isValidAIReleaseSpecialistConclusion(result.Conclusion) {
		return nil, fmt.Errorf("invalid conclusion: %s", result.Conclusion)
	}
	result.Markdown = renderAIReleaseSpecialistResultMarkdown(result)
	return result, nil
}

func extractJSONCodeBlock(text string) string {
	trimmed := strings.TrimSpace(text)
	if strings.HasPrefix(trimmed, "```json") {
		trimmed = strings.TrimPrefix(trimmed, "```json")
		trimmed = strings.TrimSpace(trimmed)
		if strings.HasSuffix(trimmed, "```") {
			trimmed = strings.TrimSuffix(trimmed, "```")
		}
		return strings.TrimSpace(trimmed)
	}
	if strings.HasPrefix(trimmed, "```") {
		trimmed = strings.TrimPrefix(trimmed, "```")
		trimmed = strings.TrimSpace(trimmed)
		if strings.HasSuffix(trimmed, "```") {
			trimmed = strings.TrimSuffix(trimmed, "```")
		}
	}
	return strings.TrimSpace(trimmed)
}

func normalizeAIResultValue(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "pass", "passed", "ok", "success":
		return "pass"
	case "warning", "warn":
		return "warning"
	case "fail", "failed", "error":
		return "fail"
	default:
		return strings.ToLower(strings.TrimSpace(value))
	}
}

func isValidAIReleaseSpecialistConclusion(value string) bool {
	switch value {
	case "pass", "warning", "fail":
		return true
	default:
		return false
	}
}

func translateAIResultValue(value string) string {
	switch normalizeAIResultValue(value) {
	case "pass":
		return "通过"
	case "warning":
		return "需关注"
	case "fail":
		return "不建议继续"
	default:
		return strings.TrimSpace(value)
	}
}

func writeAIReleaseSpecialistOutputs(workflowCtx *commonmodels.WorkflowTaskCtx, jobKey string, result *commonmodels.AIReleaseSpecialistResult) error {
	if workflowCtx == nil || result == nil {
		return nil
	}
	resultJSONBytes, err := json.Marshal(result)
	if err == nil {
		workflowCtx.GlobalContextSet(runtimejob.GetJobOutputKey(jobKey, "RESULT_JSON"), string(resultJSONBytes))
	}
	workflowCtx.GlobalContextSet(runtimejob.GetJobOutputKey(jobKey, "CONCLUSION"), result.Conclusion)
	workflowCtx.GlobalContextSet(runtimejob.GetJobOutputKey(jobKey, "SUMMARY"), result.Summary)
	workflowCtx.GlobalContextSet(runtimejob.GetJobOutputKey(jobKey, "CHECK_COUNT"), fmt.Sprintf("%d", len(result.Checks)))
	workflowCtx.GlobalContextSet(runtimejob.GetJobOutputKey(jobKey, "CHECK_DETAILS_MARKDOWN"), result.Markdown)
	return err
}

func renderAIReleaseSpecialistResultMarkdown(result *commonmodels.AIReleaseSpecialistResult) string {
	if result == nil {
		return ""
	}
	lines := []string{
		"## 发布结论",
		"",
		fmt.Sprintf("结论：%s", safeHTMLText(translateAIResultValue(result.Conclusion))),
	}
	if result.Summary != "" {
		lines = append(lines, "", "## 风险摘要", "", safeHTMLText(result.Summary))
	}
	if len(result.Checks) > 0 {
		lines = append(lines, "", "## 检查项")
		lines = append(lines, renderCheckDetailsHTMLTable(result.Checks))
	}
	if suggestion := renderReleaseSuggestion(result.Conclusion); suggestion != "" {
		lines = append(lines, "", "## 发布建议", "", safeHTMLText(suggestion))
	}
	return strings.Join(lines, "\n")
}

func renderCheckDetailsHTMLTable(checks []*commonmodels.AIReleaseSpecialistCheckItem) string {
	if len(checks) == 0 {
		return ""
	}
	lines := []string{
		`<table style="width:100%;border-collapse:collapse;margin:8px 0 12px;font-size:13px;line-height:1.6;">`,
		`<thead>`,
		`<tr>`,
		fmt.Sprintf(`<th style="%s">检查项</th>`, aiReleaseSpecialistTableHeaderStyle()),
		fmt.Sprintf(`<th style="%s">结果</th>`, aiReleaseSpecialistTableHeaderStyle()),
		fmt.Sprintf(`<th style="%s">判断依据</th>`, aiReleaseSpecialistTableHeaderStyle()),
		fmt.Sprintf(`<th style="%s">建议</th>`, aiReleaseSpecialistTableHeaderStyle()),
		`</tr>`,
		`</thead>`,
		`<tbody>`,
	}
	for idx, check := range checks {
		if check == nil {
			continue
		}
		name := check.Name
		if strings.TrimSpace(name) == "" {
			name = fmt.Sprintf("检查项 %d", idx+1)
		}
		lines = append(lines, fmt.Sprintf(
			`<tr><td style="%s">%s</td><td style="%s">%s</td><td style="%s">%s</td><td style="%s">%s</td></tr>`,
			aiReleaseSpecialistTableCellStyle(),
			safeHTMLText(name),
			aiReleaseSpecialistTableCellStyle(),
			renderAIResultBadge(check.Result),
			aiReleaseSpecialistTableCellStyle(),
			safeHTMLText(check.Evidence),
			aiReleaseSpecialistTableCellStyle(),
			safeHTMLText(check.Suggestion),
		))
	}
	lines = append(lines, `</tbody>`, `</table>`)
	return strings.Join(lines, "\n")
}

func aiReleaseSpecialistTableHeaderStyle() string {
	return "padding:8px 10px;border:1px solid #e5e7eb;background:#f9fafb;text-align:left;font-weight:600;color:#374151;"
}

func aiReleaseSpecialistTableCellStyle() string {
	return "padding:8px 10px;border:1px solid #e5e7eb;vertical-align:top;color:#374151;word-break:break-word;"
}

func renderAIResultBadge(value string) string {
	switch normalizeAIResultValue(value) {
	case "pass":
		return `<span style="color:#16a34a;font-weight:600;">通过</span>`
	case "warning":
		return `<span style="color:#d97706;font-weight:600;">需关注</span>`
	case "fail":
		return `<span style="color:#dc2626;font-weight:600;">不建议继续</span>`
	default:
		return safeHTMLText(translateAIResultValue(value))
	}
}

func renderReleaseSuggestion(conclusion string) string {
	switch normalizeAIResultValue(conclusion) {
	case "pass":
		return "当前未发现明确阻断风险，可继续后续发布流程。"
	case "warning":
		return "建议人工确认上述风险点，再决定是否继续发布。"
	case "fail":
		return "建议暂停发布，优先处理失败项后再重新执行。"
	default:
		return ""
	}
}

func buildChangeSummaryText(changeSummary *commonmodels.AIChangeSummary) string {
	if changeSummary == nil {
		return ""
	}
	parts := make([]string, 0, 5)
	if changeSummary.Remark != "" {
		parts = append(parts, fmt.Sprintf("remark: %s", compactSingleLine(changeSummary.Remark)))
	}
	if len(changeSummary.Services) > 0 {
		parts = append(parts, fmt.Sprintf("services: %s", strings.Join(changeSummary.Services, ", ")))
	}
	if len(changeSummary.Branches) > 0 {
		parts = append(parts, fmt.Sprintf("branches: %s", strings.Join(changeSummary.Branches, ", ")))
	}
	if len(changeSummary.Tags) > 0 {
		parts = append(parts, fmt.Sprintf("tags: %s", strings.Join(changeSummary.Tags, ", ")))
	}
	if len(changeSummary.CommitMessages) > 0 {
		parts = append(parts, fmt.Sprintf("commits: %s", strings.Join(changeSummary.CommitMessages, " | ")))
	}
	return strings.Join(parts, "\n")
}

func compactSingleLine(text string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
}

func safeHTMLText(text string) string {
	return html.EscapeString(compactSingleLine(text))
}

func uniqueSortedStrings(values []string) []string {
	set := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		set[value] = struct{}{}
	}
	resp := make([]string, 0, len(set))
	for value := range set {
		resp = append(resp, value)
	}
	sort.Strings(resp)
	return resp
}

func uniquePreserveOrder(values []string) []string {
	seen := map[string]struct{}{}
	resp := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		resp = append(resp, value)
	}
	return resp
}
