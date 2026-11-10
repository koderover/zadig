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
	"trpc.group/trpc-go/trpc-agent-go/tool"
)

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
