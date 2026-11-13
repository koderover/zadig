package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
	"trpc.group/trpc-go/trpc-agent-go/agent"
	"trpc.group/trpc-go/trpc-agent-go/event"
	"trpc.group/trpc-go/trpc-agent-go/model"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
	"github.com/koderover/zadig/v2/pkg/types"
)

type failedLogRunner struct{}

func (failedLogRunner) Close() error { return nil }
func (failedLogRunner) Run(context.Context, string, string, model.Message, ...agent.RunOption) (<-chan *event.Event, error) {
	return nil, errors.New("hidden-model-error")
}

func TestRunLogsCorrelatePersistenceFailureWithoutExposingInputs(t *testing.T) {
	var output bytes.Buffer
	original := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&output, nil)))
	t.Cleanup(func() { slog.SetDefault(original) })
	store, mock := mockStore(t)
	for i := 0; i < 3; i++ {
		mock.ExpectBegin().WillReturnError(&mysql.MySQLError{Number: 1054, Message: "hidden-database-input"})
	}
	s := &AgentService{Store: store, active: make(map[string]*activeRun)}
	run := &models.Run{ID: "run-for-log", SkillID: "workflow_diagnosis", CreatedAt: time.Now()}
	ctx, cancel := context.WithCancel(context.Background())
	s.wg.Add(1)
	s.execute(ctx, cancel, failedLogRunner{}, "user", run, "hidden-user-message", &types.AIAssistantRuntime{})
	text := output.String()
	for _, expected := range []string{`"run_id":"run-for-log"`, `"mysql_error_code":1054`, `"result_saved":false`, `"result_status":"failed"`, `"skill_id":"workflow_diagnosis"`} {
		if !strings.Contains(text, expected) {
			t.Fatalf("日志缺少关联或排错信息 %s: %s", expected, text)
		}
	}
	if strings.Contains(text, "hidden-") || strings.Count(text, `"level":"INFO"`) != 1 || strings.Count(text, `"level":"ERROR"`) != 1 {
		t.Fatalf("日志应保留一次错误和一次结束摘要，不输出输入或原始异常: %s", text)
	}
}

type responseLogRunner struct{ response *model.Response }

func (responseLogRunner) Close() error { return nil }
func (r responseLogRunner) Run(context.Context, string, string, model.Message, ...agent.RunOption) (<-chan *event.Event, error) {
	ch := make(chan *event.Event, 2)
	partial := r.response.Clone()
	partial.IsPartial = true
	ch <- &event.Event{Response: partial}
	ch <- &event.Event{Response: r.response}
	close(ch)
	return ch, nil
}

func TestModelResponseLogsFinishReasonAndUsageWithoutContent(t *testing.T) {
	for _, withUsage := range []bool{true, false} {
		t.Run(fmt.Sprintf("usage_%t", withUsage), func(t *testing.T) {
			var output bytes.Buffer
			original := slog.Default()
			slog.SetDefault(slog.New(slog.NewJSONHandler(&output, nil)))
			t.Cleanup(func() { slog.SetDefault(original) })
			response := &model.Response{ID: "response-for-log", Choices: []model.Choice{{
				FinishReason: model.StringPtr("length"),
				Message:      model.Message{Role: model.RoleAssistant, ReasoningContent: "hidden-reasoning"},
			}}}
			if withUsage {
				response.Usage = &model.Usage{PromptTokens: 100, CompletionTokens: 8192, TotalTokens: 8292,
					CompletionTokensDetails: model.CompletionTokensDetails{ReasoningTokens: 8192}}
			}
			store, mock := mockStore(t)
			for i := 0; i < 3; i++ {
				mock.ExpectBegin().WillReturnError(errors.New("hidden-database-error"))
			}
			s := &AgentService{Store: store}
			ctx, cancel := context.WithCancel(context.Background())
			s.wg.Add(1)
			s.execute(ctx, cancel, responseLogRunner{response}, "user", &models.Run{ID: "run-for-log", CreatedAt: time.Now()}, "hidden-user-message", &types.AIAssistantRuntime{})
			count := 0
			for _, line := range strings.Split(strings.TrimSpace(output.String()), "\n") {
				var entry map[string]any
				if err := json.Unmarshal([]byte(line), &entry); err != nil {
					t.Fatal(err)
				}
				if entry["msg"] != "AI 模型响应已结束" {
					continue
				}
				count++
				if entry["run_id"] != "run-for-log" || entry["response_id"] != "response-for-log" || entry["finish_reason"] != "length" || entry["has_content"] != false || entry["has_reasoning"] != true || entry["tool_call_count"] != float64(0) {
					t.Fatalf("响应日志缺少排错字段: %s", line)
				}
				if withUsage {
					usage := entry["usage"].(map[string]any)
					if usage["prompt_tokens"] != float64(100) || usage["completion_tokens"] != float64(8192) || usage["total_tokens"] != float64(8292) || usage["completion_tokens_details"].(map[string]any)["reasoning_tokens"] != float64(8192) {
						t.Fatalf("响应日志缺少用量: %s", line)
					}
				} else if entry["usage"] != nil {
					t.Fatalf("缺失的用量不能记为零: %s", line)
				}
			}
			if count != 1 || strings.Contains(output.String(), "hidden-") {
				t.Fatalf("只记录完整响应一次，且不能泄露内容: %s", output.String())
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
