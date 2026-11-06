package scmnotify

import (
	"context"
	"encoding/json"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/reviewfeedback"
	"github.com/xanzy/go-gitlab"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"unicode"

	stepspec "github.com/koderover/zadig/v2/pkg/types/step"
)

func TestAIReviewLanguageFromReport(t *testing.T) {
	finding := stepspec.AIReviewFinding{Severity: "high", Category: "security", File: "main.go", StartLine: 1, EndLine: 1, Problem: "Unsafe access", Evidence: "return value", Suggestion: "Check the value", Confidence: 0.9, Fingerprint: "stable-fingerprint"}
	report := &stepspec.AIReviewReport{Metadata: stepspec.AIReviewMetadata{Language: "en-US", Model: "model"}, Findings: []stepspec.AIReviewFinding{finding}, Incomplete: true, ExitCode: 2}
	for _, body := range []string{formatAIReviewComment(report), formatAIReviewSummaryComment(report, aiReviewInlinePublishResult{Failed: true, Skipped: 1, Fallback: report.Findings}), formatAIReviewInlineComment(finding, report.Metadata.Language)} {
		if strings.ContainsFunc(body, func(r rune) bool { return unicode.Is(unicode.Han, r) }) || !strings.Contains(body, "Security") || !strings.Contains(body, "Evidence") || !strings.Contains(body, aiReviewCommentMarker) {
			t.Fatalf("英文报告的固定文案须全部使用英文，保留标记及代码: %s", body)
		}
	}
	if !isAIReviewSummary(formatAIReviewComment(report)) || !isAIReviewSummary(formatAIReviewSummaryComment(report, aiReviewInlinePublishResult{})) {
		t.Fatal("英文摘要仍须能被归档识别")
	}
	for _, locale := range []string{"", "unknown", "zh-CN"} {
		report.Metadata.Language = locale
		if !strings.Contains(formatAIReviewComment(report), "审查未完整完成") {
			t.Fatal("旧报告缺少语言时默认中文")
		}
	}
	inline := formatAIReviewInlineComment(finding, "en-US")
	if reviewfeedback.FingerprintFromBody(inline) != finding.Fingerprint || isAIReviewSummary(inline) {
		t.Fatal("不能改变反馈指纹或把行内评论识别为摘要")
	}
}

// 归档测试只使用内存 HTTP 响应，不访问真实代码源。
type languageRoundTripper func(*http.Request) (*http.Response, error)

func (f languageRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestAIReviewLanguageArchivePreservesOriginalBody(t *testing.T) {
	original := "## Zadig AI 代码审查\n\n历史中文报告\n" + aiReviewCommentMarker
	writes := 0
	httpClient := &http.Client{Transport: languageRoundTripper(func(r *http.Request) (*http.Response, error) {
		w := httptest.NewRecorder()
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			json.NewEncoder(w).Encode([]any{map[string]any{"id": 1, "body": original, "author": map[string]int{"id": 42}}})
		} else if r.Method == http.MethodPut {
			writes++
			var body struct{ Body string }
			if json.NewDecoder(r.Body).Decode(&body) != nil || body.Body != "<details>\n<summary>Expand Zadig AI Review details</summary>\n\n"+original+"\n\n</details>" {
				t.Fatalf("仅归档说明使用新语言，旧报告原文不得改变: %s", body.Body)
			}
			w.Write([]byte(`{}`))
		} else {
			t.Fatalf("意外请求: %s", r.Method)
		}
		return w.Result(), nil
	})}
	cli, err := gitlab.NewClient("test", gitlab.WithBaseURL("http://gitlab.test"), gitlab.WithHTTPClient(httpClient))
	if err != nil {
		t.Fatal(err)
	}
	if err := archiveGitLabAIReviewSummaries(context.Background(), cli, "1", 108, 10, 42, "en-US"); err != nil || writes != 1 {
		t.Fatalf("归档须保留作者及历史摘要识别: %v %d", err, writes)
	}
}
