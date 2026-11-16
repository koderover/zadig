package service

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestCancelStopsBackgroundOnlyAfterDatabaseCommit(t *testing.T) {
	for _, committed := range []bool{false, true} {
		t.Run(map[bool]string{false: "保存失败不能停止后台", true: "保存后停止后台"}[committed], func(t *testing.T) {
			store, mock := mockStore(t)
			mock.ExpectQuery("SELECT conversation_id,skill_execution_id FROM ai_run").WithArgs("run", "user").
				WillReturnRows(sqlmock.NewRows([]string{"conversation_id", "skill_execution_id"}).AddRow("conversation", nil))
			mock.ExpectBegin()
			mock.ExpectQuery("SELECT id FROM ai_conversation .*FOR UPDATE").WithArgs("conversation", "user").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("conversation"))
			mock.ExpectQuery("SELECT status,plan_version,plan_status FROM ai_run .*FOR UPDATE").WithArgs("run", "user").
				WillReturnRows(sqlmock.NewRows([]string{"status", "plan_version", "plan_status"}).AddRow("running", 0, nil))
			save := mock.ExpectExec("UPDATE ai_run SET status='cancelling'").WithArgs("run")
			var want error
			if committed {
				save.WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectCommit()
			} else {
				want = errors.New("保存取消状态失败")
				save.WillReturnError(want)
				mock.ExpectRollback()
			}
			cancelled := false
			s := &AgentService{Store: store, active: map[string]*activeRun{"run": {cancel: func() { cancelled = true }}}}
			if err := s.Cancel(context.Background(), "user", "run"); !errors.Is(err, want) || cancelled != committed {
				t.Fatalf("只有持久化取消成功才允许中断后台: err=%v cancelled=%v", err, cancelled)
			}
		})
	}
}
