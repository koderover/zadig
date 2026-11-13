package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"trpc.group/trpc-go/trpc-agent-go/agent"
	"trpc.group/trpc-go/trpc-agent-go/event"
	"trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/session"
	"trpc.group/trpc-go/trpc-agent-go/session/inmemory"
	"trpc.group/trpc-go/trpc-agent-go/tool"
	"trpc.group/trpc-go/trpc-agent-go/tool/function"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
	"github.com/koderover/zadig/v2/pkg/tool/language"
	"github.com/koderover/zadig/v2/pkg/types"
)

func TestConfiguredRunnerFinalizesAfterSixToolIterations(t *testing.T) {
	store, mock := mockStore(t)
	platform := testPlatform(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("缺参期间不应查询业务接口: %s", r.URL)
	})
	platform.LoadRuntime = func(context.Context) (*types.AIAssistantRuntime, error) {
		return &types.AIAssistantRuntime{AIAssistantConfig: types.AIAssistantConfig{Enabled: true, HistoryRetentionDays: 90}}, nil
	}
	s := &AgentService{Store: store, Platform: platform}
	e := reportExecution("service_diagnosis")
	e.RootID = e.Run.ID
	e.Run.Context = models.PageContext{}
	for i := 0; i < 7; i++ {
		mock.ExpectQuery("SELECT id,context,resource_scopes").WillReturnRows(sqlmock.NewRows([]string{"id", "context", "scopes"}).AddRow(e.Run.ID, "{}", "[]"))
		if i < 6 {
			expectSkillInputSave(mock, e, "awaiting_input")
		}
	}
	calls := 0
	previous := model.DefaultNewHTTPClient
	t.Cleanup(func() { model.DefaultNewHTTPClient = previous })
	model.DefaultNewHTTPClient = func(...model.HTTPClientOption) model.HTTPClient {
		return &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			calls++
			raw, _ := io.ReadAll(r.Body)
			var request map[string]any
			if err := json.Unmarshal(raw, &request); err != nil {
				t.Fatal(err)
			}
			delta := map[string]any{"role": "assistant"}
			reason := "tool_calls"
			if calls <= 6 {
				delta["tool_calls"] = []any{map[string]any{"index": 0, "id": fmt.Sprintf("call-%d", calls), "type": "function", "function": map[string]string{"name": "service_diagnosis", "arguments": "{}"}}}
			} else {
				tools, _ := request["tools"].([]any)
				if calls != 7 || len(tools) != 0 {
					t.Fatal("第七次模型调用必须无工具收尾，不能增加执行上限")
				}
				if !strings.Contains(string(raw), "本轮工具调用已达到上限") {
					t.Fatal("收尾请求必须包含进度和阻塞原因说明")
				}
				delta["content"] = "草稿已保存，还需要项目和环境信息。"
				reason = "stop"
			}
			chunk := common.JSON(map[string]any{"id": "response", "object": "chat.completion.chunk", "choices": []any{map[string]any{"index": 0, "delta": delta, "finish_reason": reason}}})
			stream := "data: " + string(chunk) + "\n\ndata: [DONE]\n\n"
			return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(stream)), Request: r}, nil
		})}
	}
	mem := inmemory.NewSessionService()
	defer mem.Close()
	cfg := &types.AIAssistantRuntime{AIAssistantConfig: types.AIAssistantConfig{LLMIntegrationID: "selected"}, Protocol: "openai", Provider: "other", Model: "test", Token: "model-token", BaseURL: "http://model.internal/v1"}
	r, err := makeRunner(cfg, mem, platform, token("user"), s)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	ctx, cancel := context.WithTimeout(context.WithValue(context.Background(), executionKey{}, e), 10*time.Second)
	defer cancel()
	events, err := r.Run(ctx, e.User, e.Run.ConversationID, model.NewUserMessage("帮我诊断"), agent.WithRequestID(e.Run.ID))
	if err != nil {
		t.Fatal(err)
	}
	answer := ""
	tools := 0
	for item := range events {
		if item.Response == nil {
			continue
		}
		if item.Error != nil {
			t.Fatalf("达到上限应正常收尾: %+v", item.Error)
		}
		if item.IsPartial {
			continue
		}
		for _, choice := range item.Choices {
			if choice.Message.Role == model.RoleTool {
				tools++
			}
			if choice.Message.Role == model.RoleAssistant {
				answer = choice.Message.Content
			}
		}
	}
	if calls != 7 || tools != 6 || answer != "草稿已保存，还需要项目和环境信息。" || e.Outcome != "awaiting_input" {
		t.Fatalf("收尾不符合预期: calls=%d tools=%d answer=%q outcome=%s", calls, tools, answer, e.Outcome)
	}
}

func TestRunErrorClassificationAndRedactedOriginalLog(t *testing.T) {
	var output bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&output, nil)))
	defer slog.SetDefault(previous)
	e := reportExecution("project_onboarding")
	e.Language = language.Chinese
	e.OnboardingDraft = &models.OnboardingInput{}
	for _, test := range []struct{ kind, message, wantType, wantMessage string }{
		{model.ErrorTypeFlowError, "max tool iterations (6) exceeded", "tool_iteration_limit", "本轮处理已达上限，草稿已保存，可以继续补充信息"},
		{agent.ErrorTypeStopAgentError, "max LLM calls (8) exceeded", "llm_call_limit", "本轮处理已达上限，草稿已保存，可以继续补充信息"},
		{model.ErrorTypeAPIError, "provider unavailable; token=hidden-token", "model_service_error", "模型服务调用失败，请稍后重试"},
		{model.ErrorTypeFlowError, "callback failed; password=hidden-password", "execution_error", "任务处理失败，请查看运行状态后重试"},
	} {
		h := &session.AppendEventContext{Context: context.WithValue(context.Background(), executionKey{}, e), Event: event.NewResponseEvent("inv", "assistant", &model.Response{Error: &model.ResponseError{Type: test.kind, Message: test.message}})}
		if err := (&AgentService{}).beforePersist(h, func() error {
			if h.Event.Error.Type != test.wantType || h.Event.Error.Message != test.wantMessage {
				t.Fatalf("持久化错误没有分类: %+v", h.Event.Error)
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	if strings.Contains(output.String(), "hidden-") || !strings.Contains(output.String(), "max tool iterations (6) exceeded") || !strings.Contains(output.String(), `"run_id":"run"`) {
		t.Fatalf("日志应保留脱敏后的原始原因和关联: %s", output.String())
	}
}

func TestPrivateEventSnapshotMasksStoredArgumentsWithoutChangingExecution(t *testing.T) {
	args := []byte(`{"builds":[{"name":"build","variables":{"k1":"hidden-secret","PORT":"8080"}}]}`)
	original := event.NewResponseEvent("inv", "assistant", &model.Response{Choices: []model.Choice{{Message: model.Message{Role: model.RoleAssistant, ToolCalls: []model.ToolCall{{ID: "call", Function: model.FunctionDefinitionParam{Name: "project_onboarding", Arguments: args}}}}}}})
	original.Extensions = map[string]json.RawMessage{"trpc_agent.tool_call_args": common.JSON(map[string]string{"call": string(args)})}
	copy := privateEventSnapshot(original)
	if copy.ID != original.ID || strings.Contains(string(common.JSON(copy)), "hidden-secret") || !strings.Contains(string(common.JSON(original)), "hidden-secret") {
		t.Fatal("存储副本须隐藏变量，保持事件身份和实际执行参数")
	}
}

func TestOnboardingParameterErrorsRemainSpecificWithoutFrameworkErrorLogs(t *testing.T) {
	inner := function.NewFunctionTool(func(context.Context, struct{}) (any, error) {
		return nil, errors.New("变量 tier 不在模板可选值中")
	}, function.WithName("project_onboarding"), function.WithOutputSchema(&tool.Schema{}), function.WithDescription("测试参数错误"))
	wrapped := privateOnboardingTool{inner}
	result, err := wrapped.Call(context.Background(), []byte(`{}`))
	failure, ok := result.(*onboardingToolFailure)
	if err != nil || !ok || failure.Message != "变量 tier 不在模板可选值中" || failure.Status != "validation_failed" {
		t.Fatalf("业务错误必须有具体原因: %v %v", result, err)
	}
	e := reportExecution("project_onboarding")
	_, _ = recordToolFailure(context.WithValue(context.Background(), executionKey{}, e), &tool.AfterToolArgs{ToolName: "project_onboarding", Result: failure})
	if e.ToolErrorCount != 1 || !strings.Contains(incompleteSkillFailure(e, language.Chinese), "变量 tier 不在模板可选值中") {
		t.Fatal("结构化业务错误仍须计入连续失败和最终错误说明")
	}
}

type runErrorRunner struct{ responses []*model.Response }

func (runErrorRunner) Close() error { return nil }
func (r runErrorRunner) Run(context.Context, string, string, model.Message, ...agent.RunOption) (<-chan *event.Event, error) {
	ch := make(chan *event.Event, len(r.responses))
	for _, response := range r.responses {
		ch <- &event.Event{Response: response}
	}
	close(ch)
	return ch, nil
}

func TestExecuteLimitPreservesSavedDraftForContinuation(t *testing.T) {
	for _, test := range []struct{ kind, message, status, failure string }{
		{model.ErrorTypeFlowError, "max tool iterations (6) exceeded", "awaiting_input", "本轮处理已达上限，草稿已保存，可以继续补充信息"},
		{model.ErrorTypeAPIError, "provider unavailable", "failed", "模型服务调用失败，请稍后重试"},
	} {
		t.Run(test.status, func(t *testing.T) {
			store, mock := mockStore(t)
			mock.ExpectBegin()
			mock.ExpectQuery("SELECT id FROM ai_conversation .*FOR UPDATE").WithArgs("conversation", "user").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("conversation"))
			mock.ExpectQuery("SELECT status,model_id,model_name FROM ai_run .*FOR UPDATE").WithArgs("run").WillReturnRows(sqlmock.NewRows([]string{"status", "model_id", "model_name"}).AddRow("running", "model-id", "model-name"))
			mock.ExpectExec("UPDATE ai_run SET status").WithArgs(test.status, test.failure, sqlmock.AnyArg(), 0, 0, sqlmock.AnyArg(), false, "run").WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectExec("INSERT INTO ai_usage_daily").WithArgs(sqlmock.AnyArg(), "user", "", "model-id", "model-name", 0, 0, sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectExec("UPDATE ai_conversation SET last_active_at").WithArgs(sqlmock.AnyArg(), "conversation").WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectCommit()
			run := &models.Run{ID: "run", ConversationID: "conversation", CreatedAt: time.Now()}
			e := &execution{Run: run, Language: language.Chinese, OnboardingDraft: &models.OnboardingInput{ProjectName: "saved"}}
			s := &AgentService{Store: store}
			ctx, cancel := context.WithCancel(context.WithValue(context.Background(), executionKey{}, e))
			s.wg.Add(1)
			s.execute(ctx, cancel, runErrorRunner{responses: []*model.Response{{Error: &model.ResponseError{Type: test.kind, Message: test.message}}}}, "user", run, "问题", &types.AIAssistantRuntime{})
			if e.OnboardingDraft.ProjectName != "saved" || run.AnswerAvailable {
				t.Fatal("保留草稿，不能把缺失的最终回答标成可用")
			}
		})
	}
}
