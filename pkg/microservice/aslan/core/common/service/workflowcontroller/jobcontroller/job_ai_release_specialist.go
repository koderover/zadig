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
	"math"
	"sort"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/llm"
	runtimejob "github.com/koderover/zadig/v2/pkg/types/job"
	steptypes "github.com/koderover/zadig/v2/pkg/types/step"
)

const (
	aiReleaseSpecialistDefaultTimeoutMinutes = 60
	aiReleaseSpecialistMaxPromptTokens       = 12000
)

const defaultAIReleaseSpecialistSystemPrompt = "你是发布前检查助手。\n\n" +
	"请基于下面的发布上下文，输出一个 JSON 代码块，不要输出额外解释文字。\n\n" +
	"JSON schema:\n" +
	"{\n" +
	"  \"conclusion\": \"pass|warning|fail\",\n" +
	"  \"summary\": \"一句到三句中文总结\",\n" +
	"  \"checks\": [\n" +
	"    {\n" +
	"      \"name\": \"检查项名称\",\n" +
	"      \"result\": \"pass|warning|fail\",\n" +
	"      \"evidence\": \"判断依据\",\n" +
	"      \"suggestion\": \"建议动作\"\n" +
	"    }\n" +
	"  ]\n" +
	"}"

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

	task, err := mongodb.NewworkflowTaskv4Coll().Find(c.workflowCtx.WorkflowName, c.workflowCtx.TaskID)
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

	prompt, err := BuildAIReleaseSpecialistPrompt(c.jobTaskSpec.PromptTemplate, input)
	if err != nil {
		c.job.Status = config.StatusFailed
		c.job.Error = fmt.Sprintf("build ai release specialist prompt failed: %v", err)
		c.ack()
		return
	}

	client, err := getDefaultAIReleaseSpecialistLLMClient(jobCtx)
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
	writeAIReleaseSpecialistOutputs(c.workflowCtx, c.job.Key, c.jobTaskSpec.Result)
	c.ack()

	if result.Conclusion == "fail" {
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

		status, err := waitForNativeApprove(jobCtx, approvalSpec, c.workflowCtx.WorkflowName, c.job.Name, c.workflowCtx.TaskID, c.ack)
		c.job.Status = status
		if err != nil {
			c.job.Error = err.Error()
		}
		return
	}

	c.job.Status = config.StatusPassed
}

func (c *AIReleaseSpecialistJobCtl) SaveInfo(ctx context.Context) error {
	return mongodb.NewJobInfoColl().Create(ctx, &commonmodels.JobInfo{
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
	return aiReleaseSpecialistDefaultTimeoutMinutes
}

func (c *AIReleaseSpecialistJobCtl) getRemainingTimeout(jobStartTime time.Time) int64 {
	remainingDuration := time.Duration(c.getJobTimeout())*time.Minute - time.Since(jobStartTime)
	if remainingDuration <= 0 {
		return 0
	}
	return int64(math.Ceil(remainingDuration.Minutes()))
}

func (c *AIReleaseSpecialistJobCtl) getRuntimeConfirmUsers() ([]*commonmodels.User, error) {
	flatUsers, _ := commonutil.GeneFlatUsersWithCaller(c.jobTaskSpec.ConfirmUsers, c.workflowCtx.WorkflowTaskCreatorUserID)
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

	var (
		releaseTargets []*commonmodels.AIReleaseTargetsSummary
		scanStatuses   []string
		scanSummaries  []string
		testStatuses   []string
		testSummaries  []string
	)

	for _, stage := range task.Stages {
		for _, job := range stage.Jobs {
			if job.Name == currentJobName {
				return finalizeAIReleaseSpecialistInput(input, releaseTargets, scanStatuses, scanSummaries, testStatuses, testSummaries), nil
			}

			switch job.JobType {
			case string(config.JobZadigDeploy):
				spec := &commonmodels.JobTaskDeploySpec{}
				if err := commonmodels.IToi(job.Spec, spec); err != nil {
					continue
				}
				releaseTargets = append(releaseTargets, buildReleaseTargetFromDeploy(spec))
			case string(config.JobZadigHelmDeploy):
				spec := &commonmodels.JobTaskHelmDeploySpec{}
				if err := commonmodels.IToi(job.Spec, spec); err != nil {
					continue
				}
				releaseTargets = append(releaseTargets, buildReleaseTargetFromHelmDeploy(spec))
			case string(config.JobZadigHelmChartDeploy):
				spec := &commonmodels.JobTaskHelmChartDeploySpec{}
				if err := commonmodels.IToi(job.Spec, spec); err != nil {
					continue
				}
				releaseTargets = append(releaseTargets, buildReleaseTargetFromHelmChartDeploy(spec))
			case string(config.JobZadigBuild):
				spec := &commonmodels.JobTaskFreestyleSpec{}
				if err := commonmodels.IToi(job.Spec, spec); err != nil {
					continue
				}
				collectChangeSummaryFromFreestyleSpec(input.ChangeSummary, spec)
			case string(config.JobZadigScanning):
				scanStatuses = append(scanStatuses, fmt.Sprintf("%s:%s", job.OriginName, job.Status))
				scanSummaries = append(scanSummaries, buildResultSummaryLine(job))
			case string(config.JobZadigTesting):
				testStatuses = append(testStatuses, fmt.Sprintf("%s:%s", job.OriginName, job.Status))
				testSummaries = append(testSummaries, buildResultSummaryLine(job))
			}
		}
	}

	return finalizeAIReleaseSpecialistInput(input, releaseTargets, scanStatuses, scanSummaries, testStatuses, testSummaries), nil
}

func finalizeAIReleaseSpecialistInput(input *commonmodels.AIReleaseSpecialistInput, releaseTargets []*commonmodels.AIReleaseTargetsSummary, scanStatuses, scanSummaries, testStatuses, testSummaries []string) *commonmodels.AIReleaseSpecialistInput {
	if len(releaseTargets) > 0 {
		input.ReleaseTargets = mergeReleaseTargets(releaseTargets)
	}
	if len(scanStatuses) > 0 || len(scanSummaries) > 0 {
		input.ScanSummary = &commonmodels.AIScanSummary{
			JobStatuses: uniqueSortedStrings(scanStatuses),
			Summaries:   uniquePreserveOrder(scanSummaries),
		}
	}
	if len(testStatuses) > 0 || len(testSummaries) > 0 {
		input.TestSummary = &commonmodels.AITestSummary{
			JobStatuses: uniqueSortedStrings(testStatuses),
			Summaries:   uniquePreserveOrder(testSummaries),
		}
	}
	input.ChangeSummary.Branches = uniqueSortedStrings(input.ChangeSummary.Branches)
	input.ChangeSummary.Tags = uniqueSortedStrings(input.ChangeSummary.Tags)
	input.ChangeSummary.Services = uniqueSortedStrings(input.ChangeSummary.Services)
	input.ChangeSummary.CommitMessages = uniquePreserveOrder(input.ChangeSummary.CommitMessages)
	return input
}

func buildReleaseTargetFromDeploy(spec *commonmodels.JobTaskDeploySpec) *commonmodels.AIReleaseTargetsSummary {
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
		if spec.ServiceName != "" {
			target.ServiceNames = append(target.ServiceNames, spec.ServiceName)
		}
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
	return target
}

func buildReleaseTargetFromHelmDeploy(spec *commonmodels.JobTaskHelmDeploySpec) *commonmodels.AIReleaseTargetsSummary {
	target := &commonmodels.AIReleaseTargetsSummary{
		EnvName:    spec.Env,
		Production: spec.IsProduction,
	}
	if spec.ServiceName != "" {
		target.ServiceNames = append(target.ServiceNames, spec.ServiceName)
	}
	for _, imageAndModule := range spec.ImageAndModules {
		if spec.ServiceName != "" {
			target.ServiceNames = append(target.ServiceNames, spec.ServiceName)
		}
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
	return target
}

func buildReleaseTargetFromHelmChartDeploy(spec *commonmodels.JobTaskHelmChartDeploySpec) *commonmodels.AIReleaseTargetsSummary {
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
	}
	merged.ServiceNames = uniqueSortedStrings(merged.ServiceNames)
	merged.ImageVersions = uniquePreserveOrder(merged.ImageVersions)
	if merged.TargetCount == 0 {
		merged.TargetCount = len(merged.ServiceNames)
	}
	return merged
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

func BuildAIReleaseSpecialistPrompt(promptTemplate string, input *commonmodels.AIReleaseSpecialistInput) (string, error) {
	debugResult, err := BuildAIReleaseSpecialistPromptForDebug(promptTemplate, "", input)
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
	systemPrompt := buildAIReleaseSpecialistSystemPrompt(string(inputJSON), systemPromptOverride)
	prompt := systemPrompt
	if strings.TrimSpace(promptTemplate) != "" {
		prompt = fmt.Sprintf("%s\n\n%s", strings.TrimSpace(promptTemplate), systemPrompt)
	}
	promptTokens := getAIReleaseSpecialistPromptTokens(prompt)
	return &AIReleaseSpecialistPromptDebugResult{
		SystemPrompt:   systemPrompt,
		Prompt:         prompt,
		PromptTokens:   promptTokens,
		PromptTooLarge: promptTokens > aiReleaseSpecialistMaxPromptTokens,
	}, nil
}

func buildAIReleaseSpecialistSystemPrompt(inputJSON, systemPromptOverride string) string {
	systemPrompt := strings.TrimSpace(systemPromptOverride)
	if systemPrompt == "" {
		systemPrompt = defaultAIReleaseSpecialistSystemPrompt
	}
	return fmt.Sprintf("%s\n\n发布上下文:\n```json\n%s\n```", systemPrompt, inputJSON)
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

func writeAIReleaseSpecialistOutputs(workflowCtx *commonmodels.WorkflowTaskCtx, jobKey string, result *commonmodels.AIReleaseSpecialistResult) {
	if workflowCtx == nil || result == nil {
		return
	}
	resultJSONBytes, _ := json.Marshal(result)
	workflowCtx.GlobalContextSet(runtimejob.GetJobOutputKey(jobKey, "RESULT_JSON"), string(resultJSONBytes))
	workflowCtx.GlobalContextSet(runtimejob.GetJobOutputKey(jobKey, "CONCLUSION"), result.Conclusion)
	workflowCtx.GlobalContextSet(runtimejob.GetJobOutputKey(jobKey, "SUMMARY"), result.Summary)
	workflowCtx.GlobalContextSet(runtimejob.GetJobOutputKey(jobKey, "CHECK_COUNT"), fmt.Sprintf("%d", len(result.Checks)))
	workflowCtx.GlobalContextSet(runtimejob.GetJobOutputKey(jobKey, "CHECK_DETAILS_MARKDOWN"), renderCheckDetailsMarkdown(result.Checks))
}

func renderCheckDetailsMarkdown(checks []*commonmodels.AIReleaseSpecialistCheckItem) string {
	if len(checks) == 0 {
		return ""
	}
	lines := make([]string, 0, len(checks)*4)
	for _, check := range checks {
		if check == nil {
			continue
		}
		lines = append(lines, fmt.Sprintf("- %s [%s]", safeMarkdownText(check.Name), safeMarkdownText(check.Result)))
		if check.Evidence != "" {
			lines = append(lines, fmt.Sprintf("  - 依据: %s", safeMarkdownText(check.Evidence)))
		}
		if check.Suggestion != "" {
			lines = append(lines, fmt.Sprintf("  - 建议: %s", safeMarkdownText(check.Suggestion)))
		}
	}
	return strings.Join(lines, "\n")
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

func safeMarkdownText(text string) string {
	return strings.ReplaceAll(compactSingleLine(text), "\n", " ")
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

func getDefaultAIReleaseSpecialistLLMClient(ctx context.Context) (llm.ILLM, error) {
	llmIntegration, err := mongodb.NewLLMIntegrationColl().FindDefault(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to find default llm integration, err: %w", err)
	}

	llmConfig := llm.LLMConfig{
		ProviderName: llmIntegration.ProviderName,
		Token:        llmIntegration.Token,
		BaseURL:      llmIntegration.BaseURL,
		Model:        llmIntegration.Model,
	}
	if llmIntegration.EnableProxy {
		llmConfig.Proxy = config.ProxyHTTPSAddr()
	}

	llmClient, err := llm.NewClient(llmConfig.ProviderName)
	if err != nil {
		return nil, fmt.Errorf("could not create the llm client for %s: %w", llmConfig.ProviderName, err)
	}
	if err := llmClient.Configure(llmConfig); err != nil {
		return nil, fmt.Errorf("could not configure the llm client for %s: %w", llmConfig.ProviderName, err)
	}
	return llmClient, nil
}
