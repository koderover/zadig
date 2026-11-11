package ai

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/llm"
	"go.uber.org/zap"
)

func TestAnalyzeProjectCompletionBudgetAndTruncation(t *testing.T) {
	for _, truncated := range []bool{false, true} {
		t.Run(map[bool]string{false: "complete", true: "truncated"}[truncated], func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var request struct {
					MaxTokens int `json:"max_tokens"`
				}
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil || request.MaxTokens != 16384 {
					t.Errorf("unexpected completion budget: %+v, %v", request, err)
				}
				reason := "end_turn"
				if truncated {
					reason = "max_tokens"
				}
				json.NewEncoder(w).Encode(map[string]any{"content": []map[string]string{{"type": "text", "text": "report"}}, "stop_reason": reason})
			}))
			defer server.Close()
			client, err := llm.NewClientByProtocol(llm.ProtocolAnthropic)
			if err != nil {
				t.Fatal(err)
			}
			if err := client.Configure(llm.LLMConfig{BaseURL: server.URL, Model: "test"}); err != nil {
				t.Fatal(err)
			}
			answer := &analysisAnswer{answer: map[string]string{}, m: &sync.Mutex{}}
			err = AnalyzeProject("分析项目", &ProjectData{ProjectName: "p"}, client, answer, zap.NewNop().Sugar())
			if truncated {
				if !errors.Is(err, llm.ErrMaxTokensExceeded) || len(answer.answer) != 0 {
					t.Fatalf("truncated report must fail without saving a partial answer: %v, %+v", err, answer.answer)
				}
			} else if err != nil || answer.answer["p"] != "report" {
				t.Fatalf("complete report must be retained: %v, %+v", err, answer.answer)
			}
		})
	}
}

type promptParseClient struct {
	llm.ILLM
	response, prompt string
	err              error
	calls            int
}

func (c *promptParseClient) GetModel() string { return "test-model" }

func (c *promptParseClient) GetCompletion(_ context.Context, prompt string, _ ...llm.ParamOption) (string, error) {
	c.calls++
	c.prompt = prompt
	return c.response, c.err
}

func useAnalysisTimezone(t *testing.T, loc *time.Location) {
	t.Helper()
	previous := time.Local
	time.Local = loc
	t.Cleanup(func() { time.Local = previous })
}

func TestParseAnalysisPromptCalendarAndTimezone(t *testing.T) {
	for _, loc := range []*time.Location{time.UTC, time.FixedZone("Asia/Shanghai", 8*3600), time.FixedZone("UTC-05", -5*3600)} {
		t.Run(loc.String(), func(t *testing.T) {
			useAnalysisTimezone(t, loc)
			now := time.Date(2025, 3, 31, 12, 30, 0, 0, loc)
			for _, tc := range []struct{ name, start, end string }{
				{"yesterday", "2025-03-30 00:00:00", "2025-03-31 00:00:00"},
				{"last_week", "2025-03-24 00:00:00", "2025-03-31 00:00:00"},
				{"last_month", "2025-02-01 00:00:00", "2025-03-01 00:00:00"},
				{"recent_month", "2025-02-28 12:30:00", "2025-03-31 12:30:00"},
				{"leap_day", "2024-02-29 00:00:00", "2024-03-01 00:00:00"},
				{"year_boundary", "2024-12-31 00:00:00", "2025-01-01 00:00:00"},
				{"future_end", "2025-03-01 00:00:00", "2025-04-01 00:00:00"},
			} {
				t.Run(tc.name, func(t *testing.T) {
					response, err := json.Marshal(map[string]any{"project_list": []string{"p"}, "job_list": []string{"build"}, "start_time": tc.start, "end_time": tc.end})
					if err != nil {
						t.Fatal(err)
					}
					client := &promptParseClient{response: string(response)}
					input, err := parseAnalysisPrompt("分析 p 的构建情况", client, []string{"p", "other"}, now)
					if err != nil {
						t.Fatal(err)
					}
					start, _ := time.ParseInLocation(analysisTimeLayout, tc.start, loc)
					end, _ := time.ParseInLocation(analysisTimeLayout, tc.end, loc)
					if input.StartTime != start.Unix() || input.EndTime != min(end.Unix(), now.Unix()) || !reflect.DeepEqual(input.ProjectList, []string{"p"}) || !reflect.DeepEqual(input.JobList, []string{"build"}) {
						t.Fatalf("unexpected analysis range: %+v", input)
					}
					if client.calls != 1 {
						t.Fatalf("expected one LLM call, got %d", client.calls)
					}
					for _, expected := range []string{now.Format(analysisTimeLayout), loc.String(), now.Format("-07:00"), "上周一零点到本周一零点", "不得默认最近30天", "分析 p 的构建情况"} {
						if !strings.Contains(client.prompt, expected) {
							t.Fatalf("prompt missing %q", expected)
						}
					}
				})
			}
		})
	}
}

func TestParseAnalysisPromptResponseValidation(t *testing.T) {
	useAnalysisTimezone(t, time.UTC)
	now := time.Date(2025, 3, 31, 12, 30, 0, 0, time.UTC)
	valid := `{"start_time":"2025-03-01 00:00:00","end_time":"2025-03-31 00:00:00"}`
	for _, tc := range []struct {
		name, response, wantError string
	}{
		{"plain", valid, ""},
		{"json_fence", " \n```json\n" + valid + "\n``` \n", ""},
		{"plain_fence", "```\n" + valid + "\n```", ""},
		{"missing", `{}`, "请明确分析时间范围"},
		{"ambiguous", `{"start_time":"","end_time":""}`, "请明确分析时间范围"},
		{"null", `null`, "请明确分析时间范围"},
		{"missing_end", `{"start_time":"2025-03-01 00:00:00"}`, "请明确分析时间范围"},
		{"invalid_json", `broken`, "格式无效"},
		{"multiple_objects", valid + valid, "格式无效"},
		{"numeric_time", `{"start_time":1740787200}`, "格式无效"},
		{"wrong_project_type", `{"project_list":"p"}`, "格式无效"},
		{"invalid_date", strings.Replace(valid, "2025-03-01", "2025-02-29", 1), "时间格式无效"},
		{"invalid_format", strings.Replace(valid, "2025-03-01", "2025/03/01", 1), "时间格式无效"},
		{"fractional_seconds", strings.Replace(valid, "00:00:00", "00:00:00.5", 1), "时间格式无效"},
		{"reversed", strings.Replace(valid, "2025-03-01", "2025-04-01", 1), "时间范围无效"},
		{"equal", strings.Replace(valid, "2025-03-01", "2025-03-31", 1), "时间范围无效"},
		{"future", strings.ReplaceAll(valid, "2025-03", "2025-05"), "时间范围无效"},
		{"epoch", strings.Replace(valid, "2025-03-01", "1970-01-01", 1), "时间范围无效"},
		{"unknown_project", strings.Replace(valid, "{", `{"project_list":["unknown"],`, 1), "分析范围无效"},
		{"unknown_job", strings.Replace(valid, "{", `{"job_list":["unknown"],`, 1), "分析范围无效"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := &promptParseClient{response: tc.response}
			input, err := parseAnalysisPrompt("分析项目", client, []string{"p"}, now)
			if tc.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantError) || input != nil {
					t.Fatalf("expected %q and no usable range, got %+v, %v", tc.wantError, input, err)
				}
			} else if err != nil || !reflect.DeepEqual(input.ProjectList, []string{"p"}) || !reflect.DeepEqual(input.JobList, []string{"build", "test", "deploy", "release"}) {
				t.Fatalf("default projects and jobs must be retained: %+v, %v", input, err)
			}
			if client.calls != 1 {
				t.Fatalf("must not retry invalid responses, calls = %d", client.calls)
			}
		})
	}
	upstream := errors.New("model unavailable")
	client := &promptParseClient{err: upstream}
	if input, err := parseAnalysisPrompt("分析项目", client, nil, now); input != nil || !errors.Is(err, upstream) || client.calls != 1 {
		t.Fatalf("must stop on model failure: %+v, %v", input, err)
	}
}

func TestAnalysisExplicitRangeRemainsAuthoritative(t *testing.T) {
	now := time.Now().Unix()
	args := &AiAnalysisReq{
		Prompt:      "分析昨天的情况",
		ProjectList: []string{"p"},
		StartTime:   now - 30*86400,
		EndTime:     now,
	}
	original := *args
	if err := ValidateExplicitAnalysisArgs(args); err != nil || !reflect.DeepEqual(*args, original) {
		t.Fatalf("显式范围不应按问题中的时间重新解释: %+v, %v", args, err)
	}
}

func TestRemovedTimeParserKeepsEnterpriseServiceCode(t *testing.T) {
	if setting.Enterprise != 15 {
		t.Fatalf("移除时间服务不能改变 Enterprise 服务编号: %d", setting.Enterprise)
	}
}
