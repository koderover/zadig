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
	"github.com/koderover/zadig/v2/pkg/tool/language"
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
	cli, err := s.Client.newAIReviewCommentClient(ctx, codehostID, repoOwner)
	if err != nil {
		return AIReviewPRMetadata{}, fmt.Errorf("publish AI review result: %w", err)
	}
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
			inlineResult, inlineErr = s.Client.createAIReviewInlineComments(publishCtx, cli, projectID, repoOwner, repoName, prID, report)
		}
		if inlineErr != nil {
			inlineResult.Failed = true
			logger.Warnf("failed to publish inline AI review comments as %s: %v", cli.identity, inlineErr)
		}
	}
	comment := formatAIReviewSummaryComment(report, inlineResult)
	summary, summaryErr := s.Client.createAIReviewComment(ctx, cli, projectID, repoOwner, repoName, prID, comment, report.Metadata.Language)
	comments := inlineResult.Comments
	if summaryErr == nil {
		comments = append(comments, summary)
	}
	numericProjectID, title := inlineResult.ProjectID, inlineResult.Title
	metadata := AIReviewPRMetadata{Title: title, Author: inlineResult.Author, URL: inlineResult.URL}
	if title == "" {
		resolvedProjectID, fetched, err := s.Client.getAIReviewPRMetadata(ctx, cli, projectID, repoOwner, repoName, prID)
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
		return metadata, fmt.Errorf("publish AI review result as %s: %w", cli.identity, summaryErr)
	}
	logger.Infof("published AI review result to %s #%d as %s", projectID, prID, cli.identity)
	if inlineErr != nil {
		return metadata, fmt.Errorf("publish inline AI review comments as %s: %w", cli.identity, inlineErr)
	}
	if err := ctx.Err(); err != nil {
		return metadata, fmt.Errorf("publish AI review result: %w", err)
	}
	return metadata, nil
}

func formatAIReviewComment(report *stepspec.AIReviewReport) string {
	return formatAIReviewCommentWithFindings(report, report.Findings, language.Normalize(report.Metadata.Language).Text("审查问题", "Review findings"), -1)
}

func formatAIReviewSummaryDetails(report *stepspec.AIReviewReport, inlineResult aiReviewInlinePublishResult) string {
	lang := language.Normalize(report.Metadata.Language)
	var builder strings.Builder
	if inlineResult.Failed {
		builder.WriteString(lang.Text("行内评论发布未完成，请查看审查任务日志。\n", "Inline comment publication is incomplete. Check the review task logs.\n"))
	}
	if inlineResult.Skipped > 0 {
		fmt.Fprintf(&builder, lang.Text("已跳过 %d 个重复问题，已有未解决线程的问题未重新发送。\n", "Skipped %d duplicate findings already covered by unresolved threads.\n"), inlineResult.Skipped)
	}
	if len(inlineResult.Fallback) > 0 {
		builder.WriteString(lang.Text("\n### 未能发布为行内评论的问题\n", "\n### Findings that could not be published inline\n"))
		writeAIReviewFindings(&builder, inlineResult.Fallback, lang)
	}
	if (report.Incomplete || report.ExitCode == 2) && len(report.Errors) == 0 && len(report.Warnings) == 0 {
		builder.WriteString(lang.Text("审查未完整完成，请查看审查任务日志。\n", "The review is incomplete. Check the review task logs.\n"))
	}
	if len(report.Errors) > 0 {
		builder.WriteString(lang.Text("\n### 错误\n", "\n### Errors\n"))
		for _, reportErr := range report.Errors {
			fmt.Fprintf(&builder, "\n- %s\n", markdownText(reportErr))
		}
	}
	if len(report.Warnings) > 0 {
		builder.WriteString(lang.Text("\n### 警告\n", "\n### Warnings\n"))
		for _, warning := range report.Warnings {
			fmt.Fprintf(&builder, "\n- %s\n", markdownText(warning))
		}
	}
	return strings.TrimSpace(builder.String())
}

func formatAIReviewSummaryComment(report *stepspec.AIReviewReport, inlineResult aiReviewInlinePublishResult) string {
	lang := language.Normalize(report.Metadata.Language)
	status := lang.Text("🟢 **审查通过**", "🟢 **Review passed**")
	message := lang.Text("代码质量良好，未发现明确问题。逻辑清晰，符合规范。🎉", "No clear issues were found. The code looks good. 🎉")
	if len(report.Findings) > 0 {
		message = fmt.Sprintf(lang.Text("**⚠️ 发现 %d 个问题，建议修复后再合并。**", "**⚠️ Found %d issues. Address them before merging.**"), len(report.Findings))
	}
	switch {
	case report.Incomplete || report.ExitCode == 2:
		status = lang.Text("🟡 **审查未完整完成**", "🟡 **Review incomplete**")
		message = lang.Text("**⚠️ 审查未完整完成，请查看错误和警告。**", "**⚠️ The review is incomplete. Check the errors and warnings.**")
	case report.ExitCode == 1:
		status = lang.Text("🔴 **发现阻断问题**", "🔴 **Blocking issues found**")
	}
	duration := lang.Text("未知", "Unknown")
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
	fmt.Fprintf(&builder, lang.Text("### 🤖 Zadig AI Review\n\n%s · 模型 `%s` · 已扫描 `%d` 个文件 · 耗时 %s\n\n---\n\n%s\n\n`🟥 严重: %d` | `🟧 高: %d` | `🟨 中: %d` | `🟦 低: %d`\n", "### 🤖 Zadig AI Review\n\n%s · Model `%s` · Scanned `%d` files · Duration %s\n\n---\n\n%s\n\n`🟥 Critical: %d` | `🟧 High: %d` | `🟨 Medium: %d` | `🟦 Low: %d`\n"), status, markdownInline(report.Metadata.Model), report.Stats.ChangedFiles, duration, message, counts["critical"], counts["high"], counts["medium"], counts["low"])
	if len(inlineResult.Links) > 0 {
		fmt.Fprintf(&builder, lang.Text("\n<details>\n<summary>🔗 <strong>查看 %d 条行内评论</strong></summary>\n\n", "\n<details>\n<summary>🔗 <strong>View %d inline comments</strong></summary>\n\n"), len(inlineResult.Links))
		labels := map[string]string{"critical": lang.Text("🟥 严重", "🟥 Critical"), "high": lang.Text("🟧 高", "🟧 High"), "medium": lang.Text("🟨 中", "🟨 Medium"), "low": lang.Text("🟦 低", "🟦 Low"), "info": lang.Text("ℹ️ 提示", "ℹ️ Info")}
		for _, link := range inlineResult.Links {
			label := labels[strings.ToLower(strings.TrimSpace(link.Finding.Severity))]
			if label == "" {
				label = lang.Text("ℹ️ 问题", "ℹ️ Finding")
			}
			fmt.Fprintf(&builder, lang.Text("- [%s：%s · `%s:%d`](<%s>)\n", "- [%s: %s · `%s:%d`](<%s>)\n"), label, aiReviewFindingTitle(link.Finding, lang), markdownInline(link.Finding.File), link.Line, link.URL)
		}
		builder.WriteString("\n</details>\n")
	} else if inlineResult.Published > 0 {
		fmt.Fprintf(&builder, lang.Text("\n已发布 %d 条行内评论，暂未获取跳转链接。\n", "\nPublished %d inline comments; links are not yet available.\n"), inlineResult.Published)
	}
	// Retain details only when publication or review diagnostics need explanation.
	if inlineResult.Failed || inlineResult.Skipped > 0 || len(inlineResult.Fallback) > 0 || report.Incomplete || report.ExitCode == 2 || len(report.Errors) > 0 || len(report.Warnings) > 0 {
		details := formatAIReviewSummaryDetails(report, inlineResult)
		fmt.Fprintf(&builder, lang.Text("\n<details>\n<summary>点击查看审查明细</summary>\n\n%s\n\n</details>\n", "\n<details>\n<summary>View review details</summary>\n\n%s\n\n</details>\n"), details)
	}
	builder.WriteString(lang.Text("\n---\n\n*AI 自动生成，仅供参考，请以人工审查为准。*\n\n欢迎直接给本条评论添加 👍 (准确) 或 👎 (误报)\n\n", "\n---\n\n*AI-generated. Use human review to make the final decision.*\n\nReact to this comment with 👍 (accurate) or 👎 (false positive).\n\n") + aiReviewCommentMarker)
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
	lang := language.Normalize(report.Metadata.Language)
	status := lang.Text("✅ 审查通过", "✅ Review passed")
	switch {
	case report.Incomplete || report.ExitCode == 2:
		status = lang.Text("⚠️ 审查未完整完成", "⚠️ Review incomplete")
	case report.ExitCode == 1:
		status = lang.Text("❌ 发现阻断问题", "❌ Blocking issues found")
	}

	var builder strings.Builder
	builder.WriteString(lang.Text("## Zadig AI 代码审查\n\n", "## Zadig AI Review\n\n"))
	fmt.Fprintf(&builder, "**%s**\n\n", status)
	fmt.Fprintf(
		&builder,
		lang.Text("- 审查范围：`%s` → `%s`\n- 变更文件：%d\n- 问题数量：%d\n- 模型：`%s`\n", "- Review range: `%s` → `%s`\n- Changed files: %d\n- Findings: %d\n- Model: `%s`\n"),
		markdownInline(report.Metadata.From),
		markdownInline(report.Metadata.To),
		report.Stats.ChangedFiles,
		len(report.Findings),
		markdownInline(report.Metadata.Model),
	)
	if len(report.Stats.BySeverity) > 0 {
		fmt.Fprintf(
			&builder,
			lang.Text("- 严重级别：严重 %d / 高 %d / 中 %d / 低 %d\n", "- Severity: critical %d / high %d / medium %d / low %d\n"),
			report.Stats.BySeverity["critical"],
			report.Stats.BySeverity["high"],
			report.Stats.BySeverity["medium"],
			report.Stats.BySeverity["low"],
		)
	}
	if inlinePublished >= 0 && len(report.Findings) > 0 {
		fmt.Fprintf(&builder, lang.Text("- 行内评论：%d / %d\n", "- Inline comments: %d / %d\n"), inlinePublished, len(report.Findings))
	}

	if len(report.Findings) == 0 {
		builder.WriteString(lang.Text("\n未发现经过验证的问题。\n", "\nNo verified issues were found.\n"))
	} else if inlinePublished >= 0 && len(findings) == 0 {
		builder.WriteString(lang.Text("\n所有问题均已发布为行内评论。\n", "\nAll findings have been published as inline comments.\n"))
	} else {
		fmt.Fprintf(&builder, "\n### %s\n", heading)
		writeAIReviewFindings(&builder, findings, lang)
	}
	if len(report.Errors) > 0 {
		builder.WriteString(lang.Text("\n### 错误\n", "\n### Errors\n"))
		for _, reportErr := range report.Errors {
			fmt.Fprintf(&builder, "\n- %s", markdownText(reportErr))
		}
		builder.WriteByte('\n')
	}
	if len(report.Warnings) > 0 {
		builder.WriteString(lang.Text("\n### 警告\n", "\n### Warnings\n"))
		for _, warning := range report.Warnings {
			fmt.Fprintf(&builder, "\n- %s", markdownText(warning))
		}
		builder.WriteByte('\n')
	}
	builder.WriteString(lang.Text("\n---\n\n请使用 👍 / 👎 评价本次审查。\n\n", "\n---\n\nRate this review with 👍 / 👎.\n\n") + aiReviewCommentMarker)
	return builder.String()
}

func writeAIReviewFindings(builder *strings.Builder, findings []stepspec.AIReviewFinding, lang language.Language) {
	for i, finding := range findings {
		fmt.Fprintf(
			builder,
			lang.Text("\n#### %d. [%s] %s\n\n`%s:%d-%d` · `%s` · 置信度 %.2f\n\n%s\n", "\n#### %d. [%s] %s\n\n`%s:%d-%d` · `%s` · Confidence %.2f\n\n%s\n"),
			i+1,
			markdownText(aiReviewSeverityName(finding.Severity, lang)),
			aiReviewFindingTitle(finding, lang),
			markdownInline(finding.File),
			finding.StartLine,
			finding.EndLine,
			markdownInline(aiReviewCategoryName(finding, lang)),
			finding.Confidence,
			markdownText(finding.Problem),
		)
		if finding.Evidence != "" {
			fmt.Fprintf(builder, lang.Text("\n**证据**\n\n%s\n", "\n**Evidence**\n\n%s\n"), formatAIReviewEvidence(finding.Evidence, finding.File))
		}
		if finding.Suggestion != "" {
			fmt.Fprintf(builder, lang.Text("\n**建议**\n\n%s\n", "\n**Suggestion**\n\n%s\n"), formatAIReviewSuggestion(finding.Suggestion, finding.File))
		}
	}
}

func formatAIReviewInlineComment(finding stepspec.AIReviewFinding, locale string) string {
	lang := language.Normalize(locale)
	var builder strings.Builder
	fmt.Fprintf(
		&builder,
		lang.Text("**[%s] %s**\n\n`%s` · 置信度 %.2f\n\n%s\n", "**[%s] %s**\n\n`%s` · Confidence %.2f\n\n%s\n"),
		markdownText(aiReviewSeverityName(finding.Severity, lang)),
		aiReviewFindingTitle(finding, lang),
		markdownInline(aiReviewCategoryName(finding, lang)),
		finding.Confidence,
		markdownText(finding.Problem),
	)
	if finding.Evidence != "" {
		fmt.Fprintf(&builder, lang.Text("\n**证据**\n\n%s\n", "\n**Evidence**\n\n%s\n"), formatAIReviewEvidence(finding.Evidence, finding.File))
	}
	if finding.Suggestion != "" {
		fmt.Fprintf(&builder, lang.Text("\n**建议**\n\n%s\n", "\n**Suggestion**\n\n%s\n"), formatAIReviewSuggestion(finding.Suggestion, finding.File))
	}
	builder.WriteString(lang.Text("\n---\n\n请使用 👍 / 👎 评价本次审查。\n\n", "\n---\n\nRate this review with 👍 / 👎.\n\n") + aiReviewCommentMarker + reviewfeedback.FingerprintMarker(finding.Fingerprint))
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

func aiReviewFindingTitle(finding stepspec.AIReviewFinding, lang language.Language) string {
	title := singleLineText(finding.Title)
	if title == "" {
		title = aiReviewCategoryName(finding, lang)
	}
	if title == "" {
		title = lang.Text("未命名问题", "Untitled finding")
	}
	return markdownText(title)
}

func aiReviewSeverityName(severity string, lang language.Language) string {
	switch strings.ToLower(strings.TrimSpace(severity)) {
	case "critical":
		return lang.Text("严重", "Critical")
	case "high":
		return lang.Text("高", "High")
	case "medium":
		return lang.Text("中", "Medium")
	case "low":
		return lang.Text("低", "Low")
	case "info":
		return lang.Text("提示", "Info")
	default:
		return singleLineText(severity)
	}
}

func aiReviewCategoryName(finding stepspec.AIReviewFinding, lang language.Language) string {
	if name := singleLineText(finding.CategoryName); name != "" {
		return name
	}
	switch strings.ToLower(strings.TrimSpace(finding.Category)) {
	case "reliability":
		return lang.Text("正确性与可靠性", "Correctness and reliability")
	case "correctness":
		return lang.Text("正确性", "Correctness")
	case "security":
		return lang.Text("安全性", "Security")
	case "performance":
		return lang.Text("性能", "Performance")
	case "maintainability":
		return lang.Text("可维护性", "Maintainability")
	case "readability":
		return lang.Text("可读性", "Readability")
	case "style":
		return lang.Text("代码风格", "Code style")
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
