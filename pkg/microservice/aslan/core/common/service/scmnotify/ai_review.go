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
	"strings"

	"go.uber.org/zap"

	stepspec "github.com/koderover/zadig/v2/pkg/types/step"
)

func (s *Service) PublishAIReviewReport(codehostID int, repoOwner, repoName string, prID int, report *stepspec.AIReviewReport, logger *zap.SugaredLogger) error {
	if report == nil || prID <= 0 {
		return nil
	}
	projectID := strings.TrimLeft(repoOwner+"/"+repoName, "/")
	comment := formatAIReviewComment(report)
	if err := s.Client.CreateAIReviewComment(codehostID, projectID, repoOwner, repoName, prID, comment); err != nil {
		return fmt.Errorf("publish AI review result: %w", err)
	}
	logger.Infof("published AI review result to %s #%d", projectID, prID)
	return nil
}

func formatAIReviewComment(report *stepspec.AIReviewReport) string {
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

	if len(report.Findings) == 0 {
		builder.WriteString("\n未发现经过验证的问题。\n")
	} else {
		builder.WriteString("\n### Findings\n")
		for i, finding := range report.Findings {
			fmt.Fprintf(
				&builder,
				"\n#### %d. [%s] %s\n\n`%s:%d-%d` · `%s` · confidence %.2f\n\n%s\n",
				i+1,
				strings.ToUpper(markdownText(finding.Severity)),
				markdownText(finding.Title),
				markdownInline(finding.File),
				finding.StartLine,
				finding.EndLine,
				markdownInline(finding.Category),
				finding.Confidence,
				markdownText(finding.Problem),
			)
			if finding.Evidence != "" {
				fmt.Fprintf(&builder, "\n**证据**\n\n%s\n", markdownText(finding.Evidence))
			}
			if finding.Suggestion != "" {
				fmt.Fprintf(&builder, "\n**建议**\n\n%s\n", markdownText(finding.Suggestion))
			}
		}
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
	builder.WriteString("\n<!-- zadig-ai-review -->")
	return builder.String()
}

func markdownInline(value string) string {
	return strings.ReplaceAll(markdownText(value), "`", "\\`")
}

func markdownText(value string) string {
	// Prevent model-generated text from notifying users or teams in the target repository.
	return strings.ReplaceAll(value, "@", "@\u200b")
}
