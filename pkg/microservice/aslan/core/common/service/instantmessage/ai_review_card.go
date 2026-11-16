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
	"html"
	"strconv"
	"strings"
	"time"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/webhooknotify"
	"github.com/koderover/zadig/v2/pkg/setting"
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

// aiReviewSummary is the channel-independent content of an AI review notification; text fields are raw and
// each channel renderer escapes them for its own format.
type aiReviewSummary struct {
	Verdict       string
	Color         string
	Title         string
	Conclusion    string
	FailureReason string
	HasReport     bool
	Counts        map[string]int
	FindingsCount int
	ChangedFiles  int
	Publication   *stepspec.AIReviewPublication
	PRID          int
	PRText        string
	Comments      []stepspec.AIReviewInlineComment
	MoreComments  int
	Executor      string
	Duration      string
	DetailURL     string
	Spec          *stepspec.StepAIReviewReportSpec
}

// aiReviewVerdictFailed marks a task that ended without producing a review report.
const aiReviewVerdictFailed = "failed"

func buildAIReviewSummary(input *aiReviewCardInput) *aiReviewSummary {
	spec := input.Spec
	report := spec.Report
	summary := &aiReviewSummary{
		Color:       feishuHeaderTemplateRed,
		Executor:    aiReviewExecutorText(input),
		Duration:    formatAIReviewDuration(input.DurationSecond),
		DetailURL:   input.DetailURL,
		Spec:        spec,
		Publication: spec.Publication,
		PRID:        spec.PR,
	}
	if summary.Publication == nil {
		summary.Publication = &stepspec.AIReviewPublication{}
	}
	failedTitle := aiReviewCardTitle("❗ AI 审查失败", spec.RepoName)
	failureReason := aiReviewFailureReason(spec, input.JobError)
	if report == nil {
		summary.Verdict = aiReviewVerdictFailed
		summary.Title = failedTitle
		summary.Conclusion = "任务执行失败，未生成审查报告。"
		summary.FailureReason = failureReason
		return summary
	}

	summary.HasReport = true
	summary.Counts = report.SeverityCounts()
	summary.FindingsCount = len(report.Findings)
	summary.ChangedFiles = report.Stats.ChangedFiles
	verdict := aiReviewCardVerdict(spec)
	summary.Verdict = string(verdict)
	switch verdict {
	case stepspec.AIReviewVerdictIncomplete:
		// Keep the partial results so the notification matches the PR comment.
		summary.Title = failedTitle
		summary.Conclusion = "审查未完整完成，以下为已完成部分的结果。"
		summary.FailureReason = failureReason
	case stepspec.AIReviewVerdictAbnormal:
		summary.Title = failedTitle
		summary.Conclusion = "审查异常退出，无法给出审查结论。"
		summary.FailureReason = failureReason
	case stepspec.AIReviewVerdictBlocked:
		summary.Title = failedTitle
		summary.Conclusion = fmt.Sprintf("审查失败：%s。", aiReviewBlockedReason(summary.Counts, spec.FailOn))
	case stepspec.AIReviewVerdictPassedWithFindings:
		summary.Color = feishuHeaderTemplateOrange
		summary.Title = aiReviewCardTitle(fmt.Sprintf("⚠️ AI 审查通过，发现 %d 个问题", summary.FindingsCount), spec.RepoName)
		summary.Conclusion = fmt.Sprintf("审查通过，发现 %d 个问题，建议修复后再合并。", summary.FindingsCount)
	default:
		summary.Color = feishuHeaderTemplateGreen
		summary.Title = aiReviewCardTitle("👍 AI 审查通过", spec.RepoName)
		summary.Conclusion = "未发现问题，可以合并。"
	}

	if spec.PR > 0 {
		summary.PRText = aiReviewPRText(spec.PR, summary.Publication)
	}
	summary.Comments = summary.Publication.InlineComments
	if len(summary.Comments) > aiReviewCardMaxInlineComments {
		summary.MoreComments = len(summary.Comments) - aiReviewCardMaxInlineComments
		summary.Comments = summary.Comments[:aiReviewCardMaxInlineComments]
	}
	return summary
}

func (s *aiReviewSummary) severityItems(bold func(string) string) string {
	items := make([]string, 0, len(aiReviewSeverities))
	for _, severity := range aiReviewSeverities {
		item := fmt.Sprintf("%s %s %d", severity.dot, severity.label, s.Counts[severity.key])
		if s.Counts[severity.key] > 0 && bold != nil {
			item = bold(item)
		}
		items = append(items, item)
	}
	return strings.Join(items, "  |  ")
}

func (s *aiReviewSummary) statsText() string {
	return fmt.Sprintf("已扫描 %d 个文件 · 行内评论 %d 条 · 耗时 %s", s.ChangedFiles, s.Publication.InlinePublished, s.Duration)
}

func (s *aiReviewSummary) commentsTitle() string {
	return fmt.Sprintf("查看 %d 条行内评论", len(s.Publication.InlineComments))
}

// commentsListTitle heads the expanded comment list on channels without a collapsible panel.
func (s *aiReviewSummary) commentsListTitle() string {
	return fmt.Sprintf("行内评论（共 %d 条）：", len(s.Publication.InlineComments))
}

func (s *aiReviewSummary) moreCommentsText() string {
	return fmt.Sprintf("其余 %d 条请在合并请求或任务详情中查看。", s.MoreComments)
}

func aiReviewCommentLabel(comment stepspec.AIReviewInlineComment) (string, string) {
	label := "问题"
	for _, severity := range aiReviewSeverities {
		if severity.key == comment.Severity {
			label = severity.dot + " " + severity.label
			break
		}
	}
	title := comment.Title
	if strings.TrimSpace(title) == "" {
		title = "行内评论"
	}
	return label, title
}

func buildAIReviewLarkCard(input *aiReviewCardInput) *LarkCard {
	s := buildAIReviewSummary(input)
	lc := NewLarkCard()
	lc.SetConfig(true)
	lc.SetHeader(s.Color, s.Title, feiShuTagText)
	lc.AddI18NElementsZhcnMarkdown("**" + s.Conclusion + "**")
	if s.FailureReason != "" {
		lc.AddI18NElementsZhcnMarkdown(fmt.Sprintf("<font color='grey'>失败原因：%s</font>", larkMarkdownText(s.FailureReason, aiReviewCardMaxReasonRunes)))
	}
	if !s.HasReport {
		lc.AddI18NElementsZhcnMarkdown(fmt.Sprintf("<font color='grey'>%s，耗时 %s</font>", larkMarkdownText(s.Executor, aiReviewCardMaxReasonRunes), s.Duration))
		lc.AddI18NElementsZhcnMarkdown("<font color='grey'>请查看执行日志排查原因。</font>")
		lc.AddI18NElementsZhcnAction("查看任务详情", s.DetailURL)
		return lc
	}

	lc.AddI18NElementsZhcnMarkdown(s.severityItems(func(item string) string { return "**" + item + "**" }))
	lc.AddI18NElementsZhcnMarkdown(fmt.Sprintf("<font color='grey'>%s</font>", s.statsText()))
	if s.PRText != "" {
		lc.AddI18NElementsZhcnMarkdown("合并请求 " + markdownLink(larkMarkdownText(s.PRText, aiReviewCardMaxReasonRunes), s.Publication.PRURL))
	}
	if len(s.Comments) > 0 {
		lines := make([]string, 0, len(s.Comments)+1)
		for _, comment := range s.Comments {
			label, title := aiReviewCommentLabel(comment)
			lines = append(lines, fmt.Sprintf("- [%s：%s](%s)", label, larkMarkdownText(title, aiReviewCardMaxTitleRunes), comment.URL))
		}
		if s.MoreComments > 0 {
			lines = append(lines, fmt.Sprintf("<font color='grey'>%s</font>", s.moreCommentsText()))
		}
		lc.AddI18NElementsZhcnCollapsiblePanel(s.commentsTitle(), newLarkMarkdown(strings.Join(lines, "\n")))
	}
	lc.AddI18NElementsZhcnMarkdown(fmt.Sprintf("<font color='grey'>%s</font>", larkMarkdownText(s.Executor, aiReviewCardMaxReasonRunes)))
	lc.AddI18NElementsZhcnAction("查看任务详情", s.DetailURL)
	return lc
}

// buildAIReviewMarkdown renders the summary for markdown channels (DingTalk, WeCom, MS Teams). These channels have no
// collapsible panel, so the capped inline comment list is shown directly. The first line is the title, which
// MS Teams strips and shows as the card title.
func buildAIReviewMarkdown(input *aiReviewCardInput, webHookType setting.NotifyWebHookType) (string, string) {
	s := buildAIReviewSummary(input)
	blocks := []string{"### " + markdownText(s.Title, aiReviewCardMaxReasonRunes), "**" + s.Conclusion + "**"}
	if s.FailureReason != "" {
		blocks = append(blocks, "失败原因："+markdownText(s.FailureReason, aiReviewCardMaxReasonRunes))
	}
	if s.HasReport {
		blocks = append(blocks, s.severityItems(func(item string) string { return "**" + item + "**" }), s.statsText())
		if s.PRText != "" {
			blocks = append(blocks, "合并请求 "+markdownLink(markdownText(s.PRText, aiReviewCardMaxReasonRunes), s.Publication.PRURL))
		}
		if len(s.Comments) > 0 {
			lines := []string{s.commentsListTitle()}
			for _, comment := range s.Comments {
				label, title := aiReviewCommentLabel(comment)
				lines = append(lines, "- "+markdownLink(label+"："+markdownText(title, aiReviewCardMaxTitleRunes), comment.URL))
			}
			blocks = append(blocks, strings.Join(lines, "\n"))
			if s.MoreComments > 0 {
				blocks = append(blocks, s.moreCommentsText())
			}
		}
		blocks = append(blocks, markdownText(s.Executor, aiReviewCardMaxReasonRunes))
	} else {
		blocks = append(blocks, markdownText(s.Executor, aiReviewCardMaxReasonRunes)+"，耗时 "+s.Duration, "请查看执行日志排查原因。")
	}
	if webHookType == setting.NotifyWebHookTypeWechatWork {
		// WeCom markdown messages have no button, so the detail link goes into the text.
		blocks = append(blocks, markdownLink("查看任务详情", s.DetailURL))
	}
	return s.Title, strings.Join(blocks, "\n\n")
}

// buildAIReviewMailContent renders the summary as the HTML fragment placed in the mail template; the template
// uses text/template, so every value is escaped here.
func buildAIReviewMailContent(input *aiReviewCardInput) (string, string) {
	s := buildAIReviewSummary(input)
	esc := html.EscapeString
	link := func(text, url string) string {
		if url == "" {
			return esc(text)
		}
		return fmt.Sprintf(`<a href="%s" target="_blank">%s</a>`, esc(url), esc(text))
	}
	lines := []string{"<b>" + esc(s.Conclusion) + "</b>"}
	if s.FailureReason != "" {
		lines = append(lines, "失败原因："+esc(plainText(s.FailureReason, aiReviewCardMaxReasonRunes)))
	}
	if s.HasReport {
		lines = append(lines, esc(s.severityItems(nil)), esc(s.statsText()))
		if s.PRText != "" {
			lines = append(lines, "合并请求 "+link(plainText(s.PRText, aiReviewCardMaxReasonRunes), s.Publication.PRURL))
		}
		if len(s.Comments) > 0 {
			lines = append(lines, esc(s.commentsListTitle()))
			for _, comment := range s.Comments {
				label, title := aiReviewCommentLabel(comment)
				lines = append(lines, "· "+link(label+"："+plainText(title, aiReviewCardMaxTitleRunes), comment.URL))
			}
			if s.MoreComments > 0 {
				lines = append(lines, esc(s.moreCommentsText()))
			}
		}
		lines = append(lines, esc(plainText(s.Executor, aiReviewCardMaxReasonRunes)))
	} else {
		lines = append(lines, esc(plainText(s.Executor, aiReviewCardMaxReasonRunes))+"，耗时 "+esc(s.Duration), "请查看执行日志排查原因。")
	}
	return s.Title, strings.Join(lines, "\n")
}

// buildAIReviewWebhookPayload exposes the review outcome to webhook receivers with the full inline comment list.
func buildAIReviewWebhookPayload(input *aiReviewCardInput) *webhooknotify.WorkflowNotifyAIReview {
	s := buildAIReviewSummary(input)
	payload := &webhooknotify.WorkflowNotifyAIReview{
		Verdict:         s.Verdict,
		Title:           s.Title,
		Conclusion:      s.Conclusion,
		FailureReason:   s.FailureReason,
		RepoName:        s.Spec.RepoName,
		PR:              s.PRID,
		PRURL:           s.Publication.PRURL,
		SourceBranch:    s.Publication.SourceBranch,
		TargetBranch:    s.Publication.TargetBranch,
		FindingsCount:   s.FindingsCount,
		SeverityCounts:  s.Counts,
		ChangedFiles:    s.ChangedFiles,
		InlinePublished: s.Publication.InlinePublished,
	}
	for _, comment := range s.Publication.InlineComments {
		payload.InlineComments = append(payload.InlineComments, &webhooknotify.WorkflowNotifyAIReviewComment{
			Severity: comment.Severity,
			Title:    comment.Title,
			URL:      comment.URL,
		})
	}
	return payload
}

func markdownLink(text, url string) string {
	if url == "" {
		return text
	}
	return fmt.Sprintf("[%s](%s)", text, url)
}

// aiReviewCardVerdict adjusts the report verdict to the task outcome: with the quality gate off,
// the run script turns exit code 1 into success, so blocking findings only mean "passed with findings".
func aiReviewCardVerdict(spec *stepspec.StepAIReviewReportSpec) stepspec.AIReviewVerdict {
	verdict := spec.Report.Verdict()
	if verdict == stepspec.AIReviewVerdictBlocked && spec.QualityGateEnabled != nil && !*spec.QualityGateEnabled {
		if len(spec.Report.Findings) > 0 {
			return stepspec.AIReviewVerdictPassedWithFindings
		}
		return stepspec.AIReviewVerdictPassed
	}
	return verdict
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
		if spec.Report.Verdict() == stepspec.AIReviewVerdictAbnormal {
			return fmt.Sprintf("审查进程异常退出（退出码 %d）", spec.Report.ExitCode)
		}
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
	if publication.SourceBranch != "" && publication.TargetBranch != "" {
		return fmt.Sprintf("%s · %s → %s", ref, publication.SourceBranch, publication.TargetBranch)
	}
	return ref
}

func aiReviewExecutorText(input *aiReviewCardInput) string {
	return fmt.Sprintf("%s 于 %s 在项目「%s」执行", plainText(input.Creator, aiReviewCardMaxTitleRunes),
		time.Unix(input.StartTime, 0).Format("2006-01-02 15:04:05"), plainText(input.ProjectName, aiReviewCardMaxTitleRunes))
}

func formatAIReviewDuration(seconds int64) string {
	if seconds < 0 {
		seconds = 0
	}
	return (time.Duration(seconds) * time.Second).String()
}

// plainText flattens text to one line and truncates it to maxRunes.
func plainText(value string, maxRunes int) string {
	value = strings.Join(strings.Fields(value), " ")
	if runes := []rune(value); len(runes) > maxRunes {
		value = string(runes[:maxRunes]) + "…"
	}
	return value
}

// larkMarkdownText is plainText with the characters that would break lark markdown escaped.
func larkMarkdownText(value string, maxRunes int) string {
	return strings.NewReplacer("<", "&lt;", ">", "&gt;", "[", "\\[", "]", "\\]", "*", "\\*").Replace(plainText(value, maxRunes))
}

// markdownText is plainText with markdown link and emphasis characters escaped.
func markdownText(value string, maxRunes int) string {
	return strings.NewReplacer("[", "\\[", "]", "\\]", "*", "\\*").Replace(plainText(value, maxRunes))
}

func isAIReviewCompletedStatus(status config.Status) bool {
	for _, completed := range config.CompletedStatus() {
		if status == completed {
			return true
		}
	}
	return false
}
