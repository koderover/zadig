package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
	"github.com/koderover/zadig/v2/pkg/tool/language"
	"github.com/koderover/zadig/v2/pkg/types"
	"trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/tool"
)

func TestToolFailureLimitAcrossQueryTypes(t *testing.T) {
	for _, names := range [][]string{
		{"query_catalog", "query_catalog", "query_catalog"},
		{"workflow_log_window", "service_diagnosis", "environment_inspection_history"},
	} {
		t.Run(strings.Join(names, "/"), func(t *testing.T) {
			e := &execution{}
			ctx := context.WithValue(context.Background(), executionKey{}, e)
			callbacks := tool.NewCallbacks().RegisterBeforeTool(stopAfterToolFailures).RegisterAfterTool(recordToolFailure)
			for _, name := range names {
				if result, err := callbacks.RunBeforeTool(ctx, &tool.BeforeToolArgs{ToolName: name}); err != nil || result != nil {
					t.Fatalf("must allow the first three attempts: %v", err)
				}
				callbacks.RunAfterTool(ctx, &tool.AfterToolArgs{ToolName: name, Error: errors.New("missing parameter")})
			}
			// The fourth call may already be queued in the same model response.
			if result, err := callbacks.RunBeforeTool(ctx, &tool.BeforeToolArgs{ToolName: "read_saved_operation"}); err != nil || result == nil || result.CustomResult == nil {
				t.Fatal("must block further calls, including a different tool in the same batch")
			}
			callbacks.RunAfterTool(ctx, &tool.AfterToolArgs{ToolName: names[0]})
			if e.ToolErrorCount != 3 || len(e.ToolFailures) == 0 {
				t.Fatal("must retain the stop condition and original cause")
			}
			req := &model.Request{Tools: map[string]tool.Tool{"query_catalog": nil, "service_diagnosis": nil}}
			applyToolFailureLimit(e, req)
			if len(req.Tools) != 0 || len(req.Messages) != 1 {
				t.Fatal("next model call must answer using existing errors without tools")
			}
		})
	}
}

func TestToolFailureLimitResetsAfterSuccess(t *testing.T) {
	e := &execution{}
	ctx := context.WithValue(context.Background(), executionKey{}, e)
	fail := func() {
		recordToolFailure(ctx, &tool.AfterToolArgs{ToolName: "query_catalog", Error: errors.New("missing parameter")})
	}
	fail()
	fail()
	recordToolFailure(ctx, &tool.AfterToolArgs{ToolName: "workflow_log_window"})
	fail()
	fail()
	req := &model.Request{Tools: map[string]tool.Tool{"query_catalog": nil}}
	applyToolFailureLimit(e, req)
	if e.ToolErrorCount != 2 || len(req.Tools) != 1 || len(req.Messages) != 0 {
		t.Fatal("a success before the limit must reset consecutive failures")
	}
	// A follow-up user message starts a new execution with no inherited limit.
	if _, err := stopAfterToolFailures(context.WithValue(context.Background(), executionKey{}, &execution{}), nil); err != nil {
		t.Fatal("a new run must allow queries again")
	}
}

func TestToolFailureLimitTracksUserSpecifiedTarget(t *testing.T) {
	e := &execution{Message: "查询 kr-poc/repo-one 和 kr-poc/repo-two"}
	ctx := context.WithValue(context.Background(), executionKey{}, e)
	call := func(raw string, blocked bool) {
		t.Helper()
		result, err := stopAfterToolFailures(ctx, &tool.BeforeToolArgs{ToolName: "query_catalog", Arguments: []byte(raw)})
		if err != nil || (result != nil) != blocked {
			t.Fatalf("unexpected call gate for %s: %v %v", raw, result, err)
		}
		if !blocked {
			recordToolFailure(ctx, &tool.AfterToolArgs{ToolName: "query_catalog", Arguments: []byte(raw), Error: errors.New("not found")})
		}
	}
	call(`{"kind":"repositories","repo_owner":"kr-poc","repo_name":"repo-one","keyword":"general"}`, false)
	call(`{"kind":"branches","repo_owner":"kr-poc","repo_name":"repo-one","branch":"general"}`, false)
	call(`{"kind":"configuration_paths","repo_owner":"kr-poc","repo_name":"repo-one","path":"yaml/general"}`, false)
	call(`{"kind":"configuration_paths","repo_name":"repo-one","path":"k8s/general"}`, true)
	call(`{"kind":"repositories","repo_name":"guessed-repo"}`, true)
	call(`{"kind":"repositories","repo_name":"repo"}`, true)
	call(`{"kind":"repositories","repo_owner":"kr-poc","repo_name":"repo-two"}`, false)
	if e.ToolErrorCount != 1 {
		t.Fatal("a different repository explicitly named by the user must start a fresh count")
	}
}

func TestToolFailureLimitTracksNestedServiceTargets(t *testing.T) {
	e := &execution{Message: "检查 service1 和 service2", ToolErrorCount: 2, ToolErrorTarget: map[string]string{"service_name": "service1"}}
	updateToolErrorTarget(e, []byte(`{"target":{"service_name":"service2"}}`))
	if e.ToolErrorCount != 0 || e.ToolErrorTarget["service_name"] != "service2" {
		t.Fatal("explicit service changes must reset the count across nested tool arguments")
	}
}

func TestToolFailureLimitAllowsExplicitPathChange(t *testing.T) {
	e := &execution{Message: "检查 yaml/one.yaml 和 yaml/two.yaml", ToolErrorCount: 2, ToolErrorTarget: map[string]string{"path": "yaml/one.yaml"}}
	updateToolErrorTarget(e, []byte(`{"path":"yaml/two.yaml"}`))
	if e.ToolErrorCount != 0 {
		t.Fatal("a second path explicitly named by the user is a different target")
	}
}

func TestToolFailureKeepsCauseAndClearsRecoveredTool(t *testing.T) {
	e := &execution{Language: language.English}
	ctx := context.WithValue(context.Background(), executionKey{}, e)
	callbacks := tool.NewCallbacks().RegisterAfterTool(recordToolFailure)
	call := func(name string, err error) {
		t.Helper()
		if result, callbackErr := callbacks.RunAfterTool(ctx, &tool.AfterToolArgs{ToolName: name, Error: err}); callbackErr != nil || (result != nil && (result.CustomResult != nil || result.Context != nil || result.SkipSummarization)) {
			t.Fatalf("callback must leave tool results unchanged: %v %v", result, callbackErr)
		}
	}
	call("efficiency_diagnosis", errors.New("stop_reason=max_tokens token=hidden-secret"))
	call("query_catalog", nil)
	message := incompleteSkillFailure(e, language.English)
	if !strings.Contains(message, "stop_reason=max_tokens") || strings.Contains(message, "hidden-secret") {
		t.Fatalf("must retain the cause and redact credentials: %s", message)
	}
	call("efficiency_diagnosis", nil)
	if len(e.ToolFailures) != 0 {
		t.Fatal("a recovered tool must not retain its previous failure")
	}
	call("efficiency_diagnosis", &PlatformError{Status: 403, Message: "hidden-permission-detail"})
	if message := incompleteSkillFailure(e, language.English); !strings.Contains(message, "permission") || strings.Contains(message, "hidden-permission-detail") {
		t.Fatalf("permission errors must retain their localized public message: %s", message)
	}
	call("efficiency_diagnosis", errors.New(strings.Repeat("错", 600)))
	if message := incompleteSkillFailure(e, language.Chinese); utf8.RuneCountInString(message) != 512 || !utf8.ValidString(message) {
		t.Fatal("failure must fit ai_run.error without breaking UTF-8")
	}
}

func TestExecutePersistsSpecificToolFailure(t *testing.T) {
	store, mock := mockStore(t)
	r := &models.Run{ID: "run", ConversationID: "conversation", CreatedAt: time.Now().UTC()}
	e := &execution{SkillID: "efficiency_diagnosis", ToolFailures: map[string]string{"efficiency_diagnosis": "stop_reason=max_tokens"}}
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id FROM ai_conversation .*FOR UPDATE").WithArgs("conversation", "user").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("conversation"))
	mock.ExpectQuery("SELECT status,model_id,model_name FROM ai_run .*FOR UPDATE").WithArgs("run").WillReturnRows(sqlmock.NewRows([]string{"status", "model_id", "model_name"}).AddRow("running", "model-id", "model-name"))
	mock.ExpectExec("UPDATE ai_run SET status").WithArgs("failed", "任务未完成：efficiency_diagnosis: stop_reason=max_tokens", sqlmock.AnyArg(), 10, 2, sqlmock.AnyArg(), true, "run").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO ai_usage_daily").WithArgs(sqlmock.AnyArg(), "user", "", "model-id", "model-name", 10, 2, sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE ai_conversation SET last_active_at").WithArgs(sqlmock.AnyArg(), "conversation").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	s := &AgentService{Store: store}
	ctx, cancel := context.WithCancel(context.WithValue(context.Background(), executionKey{}, e))
	s.wg.Add(1)
	s.execute(ctx, cancel, answerRunner{}, "user", r, "分析项目", &types.AIAssistantRuntime{})
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
