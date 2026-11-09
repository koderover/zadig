package service

import (
	"bytes"
	"context"
	"errors"
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
