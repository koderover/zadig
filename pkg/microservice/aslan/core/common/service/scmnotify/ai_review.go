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
	"fmt"
	"path"
	"strconv"
	"strings"

	"go.uber.org/zap"

	stepspec "github.com/koderover/zadig/v2/pkg/types/step"
)

const aiReviewCommentMarker = "<!-- zadig-ai-review -->"

type aiReviewInlinePublishResult struct {
	Published int
	Fallback  []stepspec.AIReviewFinding
}

func (s *Service) PublishAIReviewReport(codehostID int, repoOwner, repoName string, prID int, report *stepspec.AIReviewReport, logger *zap.SugaredLogger) error {
	if report == nil || prID <= 0 {
		return nil
	}
	projectID := strings.TrimLeft(repoOwner+"/"+repoName, "/")
	inlineResult := aiReviewInlinePublishResult{}
	var inlineErr error
	if len(report.Findings) > 0 {
		inlineResult, inlineErr = s.Client.createAIReviewInlineComments(codehostID, projectID, repoOwner, repoName, prID, report)
		if inlineErr != nil {
			inlineResult = aiReviewInlinePublishResult{Fallback: report.Findings}
			logger.Warnf("failed to publish inline AI review comments: %v", inlineErr)
		}
	}
	comment := formatAIReviewSummaryComment(report, inlineResult)
	if err := s.Client.CreateAIReviewComment(codehostID, projectID, repoOwner, repoName, prID, comment); err != nil {
		return fmt.Errorf("publish AI review result: %w", err)
	}
	logger.Infof("published AI review result to %s #%d", projectID, prID)
	if inlineErr != nil {
		return fmt.Errorf("publish inline AI review comments: %w", inlineErr)
	}
	return nil
}

func formatAIReviewComment(report *stepspec.AIReviewReport) string {
	return formatAIReviewCommentWithFindings(report, report.Findings, "Findings", -1)
}

func formatAIReviewSummaryComment(report *stepspec.AIReviewReport, inlineResult aiReviewInlinePublishResult) string {
	return formatAIReviewCommentWithFindings(report, inlineResult.Fallback, "未能发布为行内评论的 Findings", inlineResult.Published)
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
	builder.WriteString("## Zadig AI Review\n\n")
	fmt.Fprintf(&builder, "**%s**\n\n", status)
	fmt.Fprintf(
		&builder,
		"- 审查范围：`%s` → `%s`\n- 变更文件：%d\n- Findings：%d\n- 模型：`%s`\n",
		markdownInline(report.Metadata.From),
		markdownInline(report.Metadata.To),
		report.Stats.ChangedFiles,
		len(report.Findings),
		markdownInline(report.Metadata.Model),
	)
	if len(report.Stats.BySeverity) > 0 {
		fmt.Fprintf(
			&builder,
			"- 严重级别：critical %d / high %d / medium %d / low %d\n",
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
		builder.WriteString("\n所有 finding 均已发布为行内评论。\n")
	} else {
		fmt.Fprintf(&builder, "\n### %s\n", heading)
		writeAIReviewFindings(&builder, findings)
	}
	if len(report.Errors) > 0 {
		builder.WriteString("\n### Errors\n")
		for _, reportErr := range report.Errors {
			fmt.Fprintf(&builder, "\n- %s", markdownText(reportErr))
		}
		builder.WriteByte('\n')
	}
	if len(report.Warnings) > 0 {
		builder.WriteString("\n### Warnings\n")
		for _, warning := range report.Warnings {
			fmt.Fprintf(&builder, "\n- %s", markdownText(warning))
		}
		builder.WriteByte('\n')
	}
	builder.WriteString("\n" + aiReviewCommentMarker)
	return builder.String()
}

func writeAIReviewFindings(builder *strings.Builder, findings []stepspec.AIReviewFinding) {
	for i, finding := range findings {
		fmt.Fprintf(
			builder,
			"\n#### %d. [%s] %s\n\n`%s:%d-%d` · `%s` · confidence %.2f\n\n%s\n",
			i+1,
			strings.ToUpper(markdownText(finding.Severity)),
			aiReviewFindingTitle(finding),
			markdownInline(finding.File),
			finding.StartLine,
			finding.EndLine,
			markdownInline(finding.Category),
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
		"**[%s] %s**\n\n`%s` · confidence %.2f\n\n%s\n",
		strings.ToUpper(markdownText(finding.Severity)),
		aiReviewFindingTitle(finding),
		markdownInline(finding.Category),
		finding.Confidence,
		markdownText(finding.Problem),
	)
	if finding.Evidence != "" {
		fmt.Fprintf(&builder, "\n**证据**\n\n%s\n", formatAIReviewEvidence(finding.Evidence, finding.File))
	}
	if finding.Suggestion != "" {
		fmt.Fprintf(&builder, "\n**建议**\n\n%s\n", formatAIReviewSuggestion(finding.Suggestion, finding.File))
	}
	builder.WriteString("\n" + aiReviewCommentMarker)
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
		title = singleLineText(finding.Category)
	}
	if title == "" {
		title = "未命名问题"
	}
	return markdownText(title)
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
