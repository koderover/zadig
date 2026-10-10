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

package scmnotify

import (
	"context"
	"fmt"
	"path"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/reviewfeedback"
	stepspec "github.com/koderover/zadig/v2/pkg/types/step"
)

const aiReviewCommentMarker = "<!-- zadig-ai-review -->"

type aiReviewInlinePublishResult struct {
	Links     []aiReviewInlineLink
	Published int
	Skipped   int
	Failed    bool
	Title     string
	Author    string
	URL       string
	ProjectID int
	Threads   []models.AIReviewInlineThread
	Fallback  []stepspec.AIReviewFinding
	Comments  []reviewfeedback.PublishedComment
}

type aiReviewInlineLink struct {
	URL     string
	Finding stepspec.AIReviewFinding
	Line    int
}

type AIReviewPRMetadata struct {
	Title  string
	Author string
	URL    string
}

func (s *Service) PublishAIReviewReport(projectName string, codehostID int, repoOwner, repoName string, prID int, report *stepspec.AIReviewReport, logger *zap.SugaredLogger) (AIReviewPRMetadata, error) {
	if report == nil || prID <= 0 {
		return AIReviewPRMetadata{}, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	deadline, _ := ctx.Deadline()
	projectID := strings.TrimLeft(repoOwner+"/"+repoName, "/")
	inlineResult := aiReviewInlinePublishResult{}
	var inlineErr error
	if len(report.Findings) > 0 {
		publishCtx, cancel := context.WithTimeout(ctx, 4*time.Minute)
		defer cancel()
		token, lockErr := reviewfeedback.AcquirePublication(publishCtx, codehostID, repoOwner, repoName, prID, deadline.Add(time.Minute))
		if lockErr != nil {
			inlineErr = lockErr
		} else {
			defer func() {
				releaseCtx, releaseCancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer releaseCancel()
				if err := reviewfeedback.ReleasePublication(releaseCtx, codehostID, repoOwner, repoName, prID, token); err != nil {
					logger.Warnf("release AI review publication lease: %v", err)
				}
			}()
			inlineResult, inlineErr = s.Client.createAIReviewInlineComments(publishCtx, codehostID, projectID, repoOwner, repoName, prID, report)
		}
		if inlineErr != nil {
			inlineResult.Failed = true
			logger.Warnf("failed to publish inline AI review comments: %v", inlineErr)
		}
	}
	comment := formatAIReviewSummaryComment(report, inlineResult)
	summary, summaryErr := s.Client.CreateAIReviewComment(ctx, codehostID, projectID, repoOwner, repoName, prID, comment)
	comments := inlineResult.Comments
	if summaryErr == nil {
		comments = append(comments, summary)
	}
	numericProjectID, title := inlineResult.ProjectID, inlineResult.Title
	metadata := AIReviewPRMetadata{Title: title, Author: inlineResult.Author, URL: inlineResult.URL}
	if title == "" {
		resolvedProjectID, fetched, err := s.Client.getAIReviewPRMetadata(ctx, codehostID, projectID, repoOwner, repoName, prID)
		if err != nil {
			logger.Warnf("resolve AI review PR metadata: %v", err)
		} else {
			numericProjectID, metadata = resolvedProjectID, fetched
		}
		title = metadata.Title
	}
	// Register confirmed comments even when publication exhausted its deadline.
	registerCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := reviewfeedback.Register(registerCtx, projectName, title, codehostID, numericProjectID, repoOwner, repoName, prID, comments, inlineResult.Threads); err != nil {
		return metadata, fmt.Errorf("register AI review feedback comments: %w", err)
	}
	if summaryErr != nil {
		return metadata, fmt.Errorf("publish AI review result: %w", summaryErr)
	}
	logger.Infof("published AI review result to %s #%d", projectID, prID)
	if inlineErr != nil {
		return metadata, fmt.Errorf("publish inline AI review comments: %w", inlineErr)
	}
	if err := ctx.Err(); err != nil {
		return metadata, fmt.Errorf("publish AI review result: %w", err)
	}
	return metadata, nil
}

func formatAIReviewComment(report *stepspec.AIReviewReport) string {
	return formatAIReviewCommentWithFindings(report, report.Findings, "审查问题", -1)
}

func formatAIReviewSummaryDetails(report *stepspec.AIReviewReport, inlineResult aiReviewInlinePublishResult) string {
	var builder strings.Builder
	if inlineResult.Failed {
		builder.WriteString("行内评论发布未完成，请查看审查任务日志。\n")
	}
	if inlineResult.Skipped > 0 {
		fmt.Fprintf(&builder, "已跳过 %d 个重复问题，已有未解决线程的问题未重新发送。\n", inlineResult.Skipped)
	}
	if len(inlineResult.Fallback) > 0 {
		builder.WriteString("\n### 未能发布为行内评论的问题\n")
		writeAIReviewFindings(&builder, inlineResult.Fallback)
	}
	if (report.Incomplete || report.ExitCode == 2) && len(report.Errors) == 0 && len(report.Warnings) == 0 {
		builder.WriteString("审查未完整完成，请查看审查任务日志。\n")
	}
	if len(report.Errors) > 0 {
		builder.WriteString("\n### 错误\n")
		for _, reportErr := range report.Errors {
			fmt.Fprintf(&builder, "\n- %s\n", markdownText(reportErr))
		}
	}
	if len(report.Warnings) > 0 {
		builder.WriteString("\n### 警告\n")
		for _, warning := range report.Warnings {
			fmt.Fprintf(&builder, "\n- %s\n", markdownText(warning))
		}
	}
	return strings.TrimSpace(builder.String())
}

func formatAIReviewSummaryComment(report *stepspec.AIReviewReport, inlineResult aiReviewInlinePublishResult) string {
	status := "🟢 **审查通过**"
	message := "代码质量良好，未发现明确问题。逻辑清晰，符合规范。🎉"
	if len(report.Findings) > 0 {
		message = fmt.Sprintf("**⚠️ 发现 %d 个问题，建议修复后再合并。**", len(report.Findings))
	}
	switch {
	case report.Incomplete || report.ExitCode == 2:
		status = "🟡 **审查未完整完成**"
		message = "**⚠️ 审查未完整完成，请查看错误和警告。**"
	case report.ExitCode == 1:
		status = "🔴 **发现阻断问题**"
	}
	duration := "未知"
	if report.DurationMS > 60000 {
		duration = fmt.Sprintf("%.2fm", float64(report.DurationMS)/60000)
	} else if report.DurationMS > 0 {
		duration = strconv.FormatFloat(float64(report.DurationMS)/1000, 'f', -1, 64) + "s"
	}
	counts := make(map[string]int)
	for _, finding := range report.Findings {
		counts[strings.ToLower(strings.TrimSpace(finding.Severity))]++
	}
	var builder strings.Builder
	fmt.Fprintf(&builder, "### 🤖 Zadig AI Review\n\n%s · 模型 `%s` · 已扫描 `%d` 个文件 · 耗时 %s\n\n---\n\n%s\n\n`🟥 严重: %d` | `🟧 高: %d` | `🟨 中: %d` | `🟦 低: %d`\n", status, markdownInline(report.Metadata.Model), report.Stats.ChangedFiles, duration, message, counts["critical"], counts["high"], counts["medium"], counts["low"])
	if len(inlineResult.Links) > 0 {
		fmt.Fprintf(&builder, "\n<details>\n<summary>🔗 <strong>查看 %d 条行内评论</strong></summary>\n\n", len(inlineResult.Links))
		labels := map[string]string{"critical": "🟥 严重", "high": "🟧 高", "medium": "🟨 中", "low": "🟦 低", "info": "ℹ️ 提示"}
		for _, link := range inlineResult.Links {
			label := labels[strings.ToLower(strings.TrimSpace(link.Finding.Severity))]
			if label == "" {
				label = "ℹ️ 问题"
			}
			fmt.Fprintf(&builder, "- [%s：%s · `%s:%d`](<%s>)\n", label, aiReviewFindingTitle(link.Finding), markdownInline(link.Finding.File), link.Line, link.URL)
		}
		builder.WriteString("\n</details>\n")
	} else if inlineResult.Published > 0 {
		fmt.Fprintf(&builder, "\n已发布 %d 条行内评论，暂未获取跳转链接。\n", inlineResult.Published)
	}
	// Retain details only when publication or review diagnostics need explanation.
	if inlineResult.Failed || inlineResult.Skipped > 0 || len(inlineResult.Fallback) > 0 || report.Incomplete || report.ExitCode == 2 || len(report.Errors) > 0 || len(report.Warnings) > 0 {
		details := formatAIReviewSummaryDetails(report, inlineResult)
		fmt.Fprintf(&builder, "\n<details>\n<summary>点击查看审查明细</summary>\n\n%s\n\n</details>\n", details)
	}
	builder.WriteString("\n---\n\n*AI 自动生成，仅供参考，请以人工审查为准。*\n\n欢迎直接给本条评论添加 👍 (准确) 或 👎 (误报)\n\n" + aiReviewCommentMarker)
	return builder.String()
}

func filterAIReviewFindings(findings []stepspec.AIReviewFinding, threads []models.AIReviewInlineThread) ([]stepspec.AIReviewFinding, int) {
	open := make(map[string]bool)
	for _, thread := range threads {
		if thread.Fingerprint != "" && !thread.Resolved && !thread.Deleted {
			open[thread.Fingerprint] = true
		}
	}
	seen := make(map[string]bool)
	filtered := make([]stepspec.AIReviewFinding, 0, len(findings))
	skipped := 0
	for _, finding := range findings {
		if finding.Fingerprint != "" {
			if open[finding.Fingerprint] || seen[finding.Fingerprint] {
				skipped++
				continue
			}
			seen[finding.Fingerprint] = true
		}
		filtered = append(filtered, finding)
	}
	return filtered, skipped
}

func formatAIReviewCommentWithFindings(report *stepspec.AIReviewReport, findings []stepspec.AIReviewFinding, heading string, inlinePublished int) string {
	status := "✅ 审查通过"
	switch {
	case report.Incomplete || report.ExitCode == 2:
		status = "⚠️ 审查未完整完成"
	case report.ExitCode == 1:
		status = "❌ 发现阻断问题"
	}

	var builder strings.Builder
	builder.WriteString("## Zadig AI 代码审查\n\n")
	fmt.Fprintf(&builder, "**%s**\n\n", status)
	fmt.Fprintf(
		&builder,
		"- 审查范围：`%s` → `%s`\n- 变更文件：%d\n- 问题数量：%d\n- 模型：`%s`\n",
		markdownInline(report.Metadata.From),
		markdownInline(report.Metadata.To),
		report.Stats.ChangedFiles,
		len(report.Findings),
		markdownInline(report.Metadata.Model),
	)
	if len(report.Stats.BySeverity) > 0 {
		fmt.Fprintf(
			&builder,
			"- 严重级别：严重 %d / 高 %d / 中 %d / 低 %d\n",
			report.Stats.BySeverity["critical"],
			report.Stats.BySeverity["high"],
			report.Stats.BySeverity["medium"],
			report.Stats.BySeverity["low"],
		)
	}
	if inlinePublished >= 0 && len(report.Findings) > 0 {
		fmt.Fprintf(&builder, "- 行内评论：%d / %d\n", inlinePublished, len(report.Findings))
	}

	if len(report.Findings) == 0 {
		builder.WriteString("\n未发现经过验证的问题。\n")
	} else if inlinePublished >= 0 && len(findings) == 0 {
		builder.WriteString("\n所有问题均已发布为行内评论。\n")
	} else {
		fmt.Fprintf(&builder, "\n### %s\n", heading)
		writeAIReviewFindings(&builder, findings)
	}
	if len(report.Errors) > 0 {
		builder.WriteString("\n### 错误\n")
		for _, reportErr := range report.Errors {
			fmt.Fprintf(&builder, "\n- %s", markdownText(reportErr))
		}
		builder.WriteByte('\n')
	}
	if len(report.Warnings) > 0 {
		builder.WriteString("\n### 警告\n")
		for _, warning := range report.Warnings {
			fmt.Fprintf(&builder, "\n- %s", markdownText(warning))
		}
		builder.WriteByte('\n')
	}
	builder.WriteString("\n---\n\n请使用 👍 / 👎 评价本次审查。\n\n" + aiReviewCommentMarker)
	return builder.String()
}

func writeAIReviewFindings(builder *strings.Builder, findings []stepspec.AIReviewFinding) {
	for i, finding := range findings {
		fmt.Fprintf(
			builder,
			"\n#### %d. [%s] %s\n\n`%s:%d-%d` · `%s` · 置信度 %.2f\n\n%s\n",
			i+1,
			markdownText(aiReviewSeverityName(finding.Severity)),
			aiReviewFindingTitle(finding),
			markdownInline(finding.File),
			finding.StartLine,
			finding.EndLine,
			markdownInline(aiReviewCategoryName(finding)),
			finding.Confidence,
			markdownText(finding.Problem),
		)
		if finding.Evidence != "" {
			fmt.Fprintf(builder, "\n**证据**\n\n%s\n", formatAIReviewEvidence(finding.Evidence, finding.File))
		}
		if finding.Suggestion != "" {
			fmt.Fprintf(builder, "\n**建议**\n\n%s\n", formatAIReviewSuggestion(finding.Suggestion, finding.File))
		}
	}
}

func formatAIReviewInlineComment(finding stepspec.AIReviewFinding) string {
	var builder strings.Builder
	fmt.Fprintf(
		&builder,
		"**[%s] %s**\n\n`%s` · 置信度 %.2f\n\n%s\n",
		markdownText(aiReviewSeverityName(finding.Severity)),
		aiReviewFindingTitle(finding),
		markdownInline(aiReviewCategoryName(finding)),
		finding.Confidence,
		markdownText(finding.Problem),
	)
	if finding.Evidence != "" {
		fmt.Fprintf(&builder, "\n**证据**\n\n%s\n", formatAIReviewEvidence(finding.Evidence, finding.File))
	}
	if finding.Suggestion != "" {
		fmt.Fprintf(&builder, "\n**建议**\n\n%s\n", formatAIReviewSuggestion(finding.Suggestion, finding.File))
	}
	builder.WriteString("\n---\n\n请使用 👍 / 👎 评价本次审查。\n\n" + aiReviewCommentMarker + reviewfeedback.FingerprintMarker(finding.Fingerprint))
	return builder.String()
}

func findAIReviewAddedLine(patch string, startLine, endLine int) (int, bool) {
	if startLine <= 0 || endLine < startLine {
		return 0, false
	}
	newLine := 0
	inHunk := false
	anchor := 0
	for _, line := range strings.Split(strings.ReplaceAll(patch, "\r\n", "\n"), "\n") {
		if strings.HasPrefix(line, "@@ ") {
			parsed, ok := parseAIReviewHunkNewStart(line)
			if !ok {
				inHunk = false
				continue
			}
			newLine = parsed
			inHunk = true
			continue
		}
		if !inHunk || line == "" {
			continue
		}
		switch line[0] {
		case '+':
			if newLine >= startLine && newLine <= endLine {
				anchor = newLine
			}
			newLine++
		case ' ':
			newLine++
		case '-':
			// Deleted lines do not advance the new-file line number.
		case '\\':
			// "No newline at end of file" marker.
		default:
			inHunk = false
		}
	}
	return anchor, anchor > 0
}

func parseAIReviewHunkNewStart(header string) (int, bool) {
	fields := strings.Fields(header)
	if len(fields) < 3 || !strings.HasPrefix(fields[2], "+") {
		return 0, false
	}
	value := strings.TrimPrefix(fields[2], "+")
	if comma := strings.IndexByte(value, ','); comma >= 0 {
		value = value[:comma]
	}
	line, err := strconv.Atoi(value)
	return line, err == nil && line > 0
}

func aiReviewFindingTitle(finding stepspec.AIReviewFinding) string {
	title := singleLineText(finding.Title)
	if title == "" {
		title = aiReviewCategoryName(finding)
	}
	if title == "" {
		title = "未命名问题"
	}
	return markdownText(title)
}

func aiReviewSeverityName(severity string) string {
	switch strings.ToLower(strings.TrimSpace(severity)) {
	case "critical":
		return "严重"
	case "high":
		return "高"
	case "medium":
		return "中"
	case "low":
		return "低"
	case "info":
		return "提示"
	default:
		return singleLineText(severity)
	}
}

func aiReviewCategoryName(finding stepspec.AIReviewFinding) string {
	if name := singleLineText(finding.CategoryName); name != "" {
		return name
	}
	switch strings.ToLower(strings.TrimSpace(finding.Category)) {
	case "reliability":
		return "正确性与可靠性"
	case "correctness":
		return "正确性"
	case "security":
		return "安全性"
	case "performance":
		return "性能"
	case "maintainability":
		return "可维护性"
	case "readability":
		return "可读性"
	case "style":
		return "代码风格"
	default:
		return singleLineText(finding.Category)
	}
}

func singleLineText(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func formatAIReviewEvidence(evidence, file string) string {
	evidence = strings.ReplaceAll(evidence, "\r\n", "\n")
	evidence = strings.Trim(evidence, "\n")
	if !strings.Contains(evidence, "\n") {
		return markdownText(evidence)
	}
	return formatAIReviewCodeBlock(evidence, file)
}

func formatAIReviewSuggestion(suggestion, file string) string {
	suggestion = strings.ReplaceAll(suggestion, "\r\n", "\n")
	suggestion = strings.Trim(suggestion, "\n")

	var formatted []string
	for _, paragraph := range strings.Split(suggestion, "\n\n") {
		paragraph = strings.Trim(paragraph, "\n")
		if paragraph == "" {
			continue
		}
		lines := strings.Split(paragraph, "\n")
		codeStart := findAIReviewCodeStart(lines)
		if codeStart < 0 {
			formatted = append(formatted, markdownText(paragraph))
			continue
		}
		if codeStart > 0 {
			formatted = append(formatted, markdownText(strings.Join(lines[:codeStart], "\n")))
		}
		formatted = append(formatted, formatAIReviewCodeBlock(strings.Join(lines[codeStart:], "\n"), file))
	}
	return strings.Join(formatted, "\n\n")
}

func findAIReviewCodeStart(lines []string) int {
	if len(lines) < 2 {
		return -1
	}
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if len(line) > len(strings.TrimLeft(line, " \t")) ||
			strings.Contains(trimmed, " := ") ||
			strings.HasPrefix(trimmed, "#include") {
			return i
		}
		for _, prefix := range []string{
			"if ", "for ", "func ", "switch ", "select ", "return ",
			"var ", "const ", "type ", "package ", "import ",
			"let ", "class ", "def ", "try ", "catch ", "else",
			"while ", "do ", "when ", "match ", "pub ", "fn ",
			"{", "}", "[", "]",
		} {
			if strings.HasPrefix(trimmed, prefix) {
				return i
			}
		}
	}
	return -1
}

func formatAIReviewCodeBlock(code, file string) string {
	fence := strings.Repeat("`", longestBacktickRun(code)+1)
	if len(fence) < 3 {
		fence = "```"
	}
	return fmt.Sprintf("%s%s\n%s\n%s", fence, aiReviewCodeLanguage(file), code, fence)
}

func longestBacktickRun(value string) int {
	longest, current := 0, 0
	for _, char := range value {
		if char == '`' {
			current++
			if current > longest {
				longest = current
			}
			continue
		}
		current = 0
	}
	return longest
}

func aiReviewCodeLanguage(file string) string {
	switch strings.ToLower(path.Ext(file)) {
	case ".go":
		return "go"
	case ".js", ".jsx", ".mjs", ".cjs":
		return "javascript"
	case ".ts", ".tsx", ".mts", ".cts":
		return "typescript"
	case ".py":
		return "python"
	case ".java":
		return "java"
	case ".kt", ".kts":
		return "kotlin"
	case ".rs":
		return "rust"
	case ".sh", ".bash":
		return "bash"
	case ".yaml", ".yml":
		return "yaml"
	case ".json":
		return "json"
	case ".xml":
		return "xml"
	case ".html", ".htm":
		return "html"
	case ".css":
		return "css"
	case ".sql":
		return "sql"
	case ".md", ".markdown":
		return "markdown"
	case ".c", ".h":
		return "c"
	case ".cc", ".cpp", ".cxx", ".hh", ".hpp":
		return "cpp"
	default:
		return "text"
	}
}

func markdownInline(value string) string {
	return strings.ReplaceAll(markdownText(value), "`", "\\`")
}

func markdownText(value string) string {
	// Prevent model-generated text from notifying users or teams in the target repository.
	return strings.ReplaceAll(value, "@", "@\u200b")
}
