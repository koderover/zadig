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

package instantmessage

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	stepspec "github.com/koderover/zadig/v2/pkg/types/step"
)

type aiReviewSeverity struct {
	key   string
	label string
	dot   string
}

// Feishu limits webhook cards to 20 KB (30 KB for app cards); collapsing a panel does not shrink the payload,
// so free-text and the inline comment list are bounded and the rest is left to the task detail page.
const (
	aiReviewCardMaxInlineComments = 10
	aiReviewCardMaxTitleRunes     = 40
	aiReviewCardMaxReasonRunes    = 200
)

var aiReviewSeverities = []aiReviewSeverity{
	{key: "critical", label: "严重", dot: "🔴"},
	{key: "high", label: "高", dot: "🟠"},
	{key: "medium", label: "中", dot: "🟡"},
	{key: "low", label: "低", dot: "🔵"},
}

type aiReviewCardInput struct {
	Spec           *stepspec.StepAIReviewReportSpec
	JobError       string
	Creator        string
	ProjectName    string
	StartTime      int64
	DurationSecond int64
	DetailURL      string
}

// findAIReviewReportSpec returns the AI review report step of a task, or nil when the task is not an AI review.
func findAIReviewReportSpec(task *models.WorkflowTask) (*stepspec.StepAIReviewReportSpec, *models.JobTask) {
	for _, stage := range task.Stages {
		for _, job := range stage.Jobs {
			jobSpec := new(models.JobTaskFreestyleSpec)
			if err := models.IToi(job.Spec, jobSpec); err != nil {
				continue
			}
			for _, step := range jobSpec.Steps {
				if step.StepType != config.StepAIReviewReport {
					continue
				}
				reportSpec := new(stepspec.StepAIReviewReportSpec)
				if err := models.IToi(step.Spec, reportSpec); err != nil {
					continue
				}
				return reportSpec, job
			}
		}
	}
	return nil, nil
}

func buildAIReviewLarkCard(input *aiReviewCardInput) *LarkCard {
	spec := input.Spec
	report := spec.Report
	lc := NewLarkCard()
	lc.SetConfig(true)

	failureReason := fmt.Sprintf("<font color='grey'>失败原因：%s</font>", larkMarkdownText(aiReviewFailureReason(spec, input.JobError), aiReviewCardMaxReasonRunes))
	if report == nil {
		lc.SetHeader(feishuHeaderTemplateRed, aiReviewCardTitle("❗ AI 审查失败", spec.RepoName), feiShuTagText)
		lc.AddI18NElementsZhcnMarkdown("**任务执行失败，未生成审查报告。**")
		lc.AddI18NElementsZhcnMarkdown(failureReason)
		lc.AddI18NElementsZhcnMarkdown(fmt.Sprintf("<font color='grey'>%s，耗时 %s</font>", aiReviewExecutorText(input), formatAIReviewDuration(input.DurationSecond)))
		lc.AddI18NElementsZhcnMarkdown("<font color='grey'>请查看执行日志排查原因。</font>")
		lc.AddI18NElementsZhcnAction("查看任务详情", input.DetailURL)
		return lc
	}

	counts := report.SeverityCounts()
	switch report.Verdict() {
	case stepspec.AIReviewVerdictIncomplete:
		// Keep the partial results so the card matches the PR comment.
		lc.SetHeader(feishuHeaderTemplateRed, aiReviewCardTitle("❗ AI 审查失败", spec.RepoName), feiShuTagText)
		lc.AddI18NElementsZhcnMarkdown("**审查未完整完成，以下为已完成部分的结果。**")
		lc.AddI18NElementsZhcnMarkdown(failureReason)
	case stepspec.AIReviewVerdictBlocked:
		lc.SetHeader(feishuHeaderTemplateRed, aiReviewCardTitle("❗ AI 审查失败", spec.RepoName), feiShuTagText)
		lc.AddI18NElementsZhcnMarkdown(fmt.Sprintf("**审查失败：%s。**", aiReviewBlockedReason(counts, spec.FailOn)))
	case stepspec.AIReviewVerdictPassedWithFindings:
		lc.SetHeader(feishuHeaderTemplateOrange, aiReviewCardTitle(fmt.Sprintf("⚠️ AI 审查通过，发现 %d 个问题", len(report.Findings)), spec.RepoName), feiShuTagText)
		lc.AddI18NElementsZhcnMarkdown(fmt.Sprintf("**审查通过，发现 %d 个问题，建议修复后再合并。**", len(report.Findings)))
	default:
		lc.SetHeader(feishuHeaderTemplateGreen, aiReviewCardTitle("👍 AI 审查通过", spec.RepoName), feiShuTagText)
		lc.AddI18NElementsZhcnMarkdown("**未发现问题，可以合并。**")
	}

	severityItems := make([]string, 0, len(aiReviewSeverities))
	for _, severity := range aiReviewSeverities {
		item := fmt.Sprintf("%s %s %d", severity.dot, severity.label, counts[severity.key])
		if counts[severity.key] > 0 {
			item = "**" + item + "**"
		}
		severityItems = append(severityItems, item)
	}
	lc.AddI18NElementsZhcnMarkdown(strings.Join(severityItems, "  |  "))

	publication := spec.Publication
	if publication == nil {
		publication = &stepspec.AIReviewPublication{}
	}
	lc.AddI18NElementsZhcnMarkdown(fmt.Sprintf("<font color='grey'>已扫描 %d 个文件 · 行内评论 %d 条 · 耗时 %s</font>",
		report.Stats.ChangedFiles, publication.InlinePublished, formatAIReviewDuration(input.DurationSecond)))

	if spec.PR > 0 {
		lc.AddI18NElementsZhcnMarkdown("合并请求 " + aiReviewPRText(spec.PR, publication))
	}

	if len(publication.InlineComments) > 0 {
		shown := publication.InlineComments
		if len(shown) > aiReviewCardMaxInlineComments {
			shown = shown[:aiReviewCardMaxInlineComments]
		}
		lines := make([]string, 0, len(shown)+1)
		for _, comment := range shown {
			label := "问题"
			for _, severity := range aiReviewSeverities {
				if severity.key == comment.Severity {
					label = severity.dot + " " + severity.label
					break
				}
			}
			title := larkMarkdownText(comment.Title, aiReviewCardMaxTitleRunes)
			if title == "" {
				title = "行内评论"
			}
			lines = append(lines, fmt.Sprintf("- [%s：%s](%s)", label, title, comment.URL))
		}
		if rest := len(publication.InlineComments) - len(shown); rest > 0 {
			lines = append(lines, fmt.Sprintf("<font color='grey'>其余 %d 条请在合并请求或任务详情中查看。</font>", rest))
		}
		lc.AddI18NElementsZhcnCollapsiblePanel(
			fmt.Sprintf("查看 %d 条行内评论", len(publication.InlineComments)),
			newLarkMarkdown(strings.Join(lines, "\n")),
		)
	}

	lc.AddI18NElementsZhcnMarkdown(fmt.Sprintf("<font color='grey'>%s</font>", aiReviewExecutorText(input)))
	lc.AddI18NElementsZhcnAction("查看任务详情", input.DetailURL)
	return lc
}

func aiReviewCardTitle(prefix, repoName string) string {
	if repoName == "" {
		return prefix
	}
	return prefix + " · " + repoName
}

// aiReviewBlockedReason names the found severities that tripped the quality gate, e.g. "存在严重、高危问题".
func aiReviewBlockedReason(counts map[string]int, failOn []string) string {
	blocking := make(map[string]bool, len(failOn))
	for _, severity := range failOn {
		blocking[strings.ToLower(strings.TrimSpace(severity))] = true
	}
	if len(blocking) == 0 {
		// Tasks created before fail_on was recorded: fall back to the default gate.
		blocking["critical"], blocking["high"] = true, true
	}
	names := map[string]string{"critical": "严重", "high": "高危", "medium": "中危", "low": "低危"}
	found := make([]string, 0, len(aiReviewSeverities))
	for _, severity := range aiReviewSeverities {
		if blocking[severity.key] && counts[severity.key] > 0 {
			found = append(found, names[severity.key])
		}
	}
	if len(found) == 0 {
		return "存在阻断问题"
	}
	return "存在" + strings.Join(found, "、") + "问题"
}

func aiReviewFailureReason(spec *stepspec.StepAIReviewReportSpec, jobError string) string {
	if spec.Report != nil {
		for _, reportErr := range spec.Report.Errors {
			if reportErr = strings.TrimSpace(reportErr); reportErr != "" {
				return reportErr
			}
		}
		return "审查未完整完成"
	}
	if spec.CollectionError != "" {
		return spec.CollectionError
	}
	if jobError = strings.TrimSpace(jobError); jobError != "" {
		return jobError
	}
	return "未知"
}

func aiReviewPRText(prID int, publication *stepspec.AIReviewPublication) string {
	ref := publication.PRRef
	if ref == "" {
		ref = "#" + strconv.Itoa(prID)
	}
	text := ref
	if publication.SourceBranch != "" && publication.TargetBranch != "" {
		text = fmt.Sprintf("%s · %s → %s", ref, publication.SourceBranch, publication.TargetBranch)
	}
	text = larkMarkdownText(text, aiReviewCardMaxReasonRunes)
	if publication.PRURL == "" {
		return text
	}
	return fmt.Sprintf("[%s](%s)", text, publication.PRURL)
}

func aiReviewExecutorText(input *aiReviewCardInput) string {
	return fmt.Sprintf("%s 于 %s 在项目「%s」执行", larkMarkdownText(input.Creator, aiReviewCardMaxTitleRunes),
		time.Unix(input.StartTime, 0).Format("2006-01-02 15:04:05"), larkMarkdownText(input.ProjectName, aiReviewCardMaxTitleRunes))
}

func formatAIReviewDuration(seconds int64) string {
	if seconds < 0 {
		seconds = 0
	}
	return (time.Duration(seconds) * time.Second).String()
}

// larkMarkdownText flattens text to one line, truncates it to maxRunes and escapes characters that would break lark markdown.
func larkMarkdownText(value string, maxRunes int) string {
	value = strings.Join(strings.Fields(value), " ")
	if runes := []rune(value); len(runes) > maxRunes {
		value = string(runes[:maxRunes]) + "…"
	}
	return strings.NewReplacer("<", "&lt;", ">", "&gt;", "[", "\\[", "]", "\\]", "*", "\\*").Replace(value)
}

func isAIReviewCompletedStatus(status config.Status) bool {
	for _, completed := range config.CompletedStatus() {
		if status == completed {
			return true
		}
	}
	return false
}
