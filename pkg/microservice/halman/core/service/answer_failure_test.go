package service

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"trpc.group/trpc-go/trpc-agent-go/agent"
	"trpc.group/trpc-go/trpc-agent-go/event"
	"trpc.group/trpc-go/trpc-agent-go/model"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
	"github.com/koderover/zadig/v2/pkg/tool/language"
	"github.com/koderover/zadig/v2/pkg/types"
)

func TestMissingAnswerFailure(t *testing.T) {
	for _, tc := range []struct {
		name, reason, reasoning string
		toolCalls               []model.ToolCall
		wantZH, wantEN          string
	}{
		{name: "output limit takes priority over reasoning", reason: "length", reasoning: "思考",
			wantZH: "模型输出已达到长度上限，未能生成完整回答，请简化问题后重试",
			wantEN: "The model reached its output limit without a complete answer. Simplify your request and try again."},
		{name: "filtered", reason: "content_filter", reasoning: "思考",
			wantZH: "模型响应被内容过滤，未能生成回答，请调整问题后重试",
			wantEN: "The model response was filtered. Rephrase your request and try again."},
		{name: "tools without final answer", reason: "tool_calls", reasoning: "思考", toolCalls: []model.ToolCall{{ID: "call"}},
			wantZH: "工具调用后模型未生成最终回答，请重试",
			wantEN: "The model did not produce a final answer after calling tools. Please try again."},
		{name: "reasoning only", reason: "stop", reasoning: "思考",
			wantZH: "模型仅返回了推理内容，未生成回答，请重试",
			wantEN: "The model returned only reasoning without an answer. Please try again."},
		{name: "missing finish reason", reasoning: " \n ",
			wantZH: "模型未返回回答内容，请重试", wantEN: "The model returned no answer. Please try again."},
		{name: "unknown finish reason", reason: "unknown",
			wantZH: "模型未返回回答内容，请重试", wantEN: "The model returned no answer. Please try again."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			choice := model.Choice{Message: model.Message{ReasoningContent: tc.reasoning, ToolCalls: tc.toolCalls}}
			if tc.reason != "" {
				choice.FinishReason = model.StringPtr(tc.reason)
			}
			for lang, want := range map[language.Language]string{language.Chinese: tc.wantZH, language.English: tc.wantEN} {
				if got := missingAnswerFailure(choice, lang); got != want {
					t.Fatalf("%s: got %q, want %q", lang, got, want)
				}
			}
		})
	}
}

type answerFailureRunner struct{ responses []*model.Response }

func (answerFailureRunner) Close() error { return nil }
func (r answerFailureRunner) Run(context.Context, string, string, model.Message, ...agent.RunOption) (<-chan *event.Event, error) {
	ch := make(chan *event.Event, len(r.responses))
	for _, response := range r.responses {
		ch <- &event.Event{Response: response}
	}
	close(ch)
	return ch, nil
}

func TestExecutePersistsMissingAnswerCauseFromLastCompleteResponse(t *testing.T) {
	length := &model.Response{Choices: []model.Choice{{FinishReason: model.StringPtr("length"), Message: model.Message{Role: model.RoleAssistant, ReasoningContent: "思考"}}}}
	empty := &model.Response{Choices: []model.Choice{{FinishReason: model.StringPtr("stop"), Message: model.Message{Role: model.RoleAssistant}}}}
	partial := length.Clone()
	partial.IsPartial = true
	for _, tc := range []struct {
		name      string
		responses []*model.Response
		want      string
	}{
		{"length", []*model.Response{length}, "模型输出已达到长度上限，未能生成完整回答，请简化问题后重试"},
		{"last response replaces earlier cause", []*model.Response{length, empty}, "模型未返回回答内容，请重试"},
		{"partial does not replace final cause", []*model.Response{empty, partial}, "模型未返回回答内容，请重试"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store, mock := mockStore(t)
			mock.ExpectBegin()
			mock.ExpectQuery("SELECT id FROM ai_conversation .*FOR UPDATE").WithArgs("conversation", "user").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("conversation"))
			mock.ExpectQuery("SELECT status,model_id,model_name FROM ai_run .*FOR UPDATE").WithArgs("run").WillReturnRows(sqlmock.NewRows([]string{"status", "model_id", "model_name"}).AddRow("running", "model-id", "model-name"))
			mock.ExpectExec("UPDATE ai_run SET status").WithArgs("failed", tc.want, sqlmock.AnyArg(), 0, 0, sqlmock.AnyArg(), false, "run").WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectExec("INSERT INTO ai_usage_daily").WithArgs(sqlmock.AnyArg(), "user", "", "model-id", "model-name", 0, 0, sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectExec("UPDATE ai_conversation SET last_active_at").WithArgs(sqlmock.AnyArg(), "conversation").WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectCommit()
			s := &AgentService{Store: store}
			run := &models.Run{ID: "run", ConversationID: "conversation", CreatedAt: time.Now()}
			ctx, cancel := context.WithCancel(context.Background())
			s.wg.Add(1)
			s.execute(ctx, cancel, answerFailureRunner{tc.responses}, "user", run, "问题", &types.AIAssistantRuntime{})
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
			if run.AnswerAvailable {
				t.Fatal("缺少回答时不能标记为可用")
			}
		})
	}
}
