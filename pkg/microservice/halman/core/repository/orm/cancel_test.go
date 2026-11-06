package orm

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
)

type stopRunFixture struct {
	root, active, skillStatus, status, planStatus string
	version                                       int
}

func expectStopRun(mock sqlmock.Sqlmock, f stopRunFixture) {
	var root, active any
	if f.root != "" {
		root = f.root
	}
	if f.active != "" {
		active = f.active
	}
	mock.ExpectQuery("SELECT conversation_id,skill_execution_id FROM ai_run").WithArgs("run", "user").
		WillReturnRows(sqlmock.NewRows([]string{"conversation_id", "skill_execution_id"}).AddRow("conversation", root))
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id FROM ai_conversation .*FOR UPDATE").WithArgs("conversation", "user").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("conversation"))
	if root != nil {
		mock.ExpectQuery("SELECT active_plan_run_id,skill_status FROM ai_run .*FOR UPDATE").WithArgs(f.root, "user").
			WillReturnRows(sqlmock.NewRows([]string{"active_plan_run_id", "skill_status"}).AddRow(active, f.skillStatus))
	}
	mock.ExpectQuery("SELECT status,plan_version,plan_status FROM ai_run .*FOR UPDATE").WithArgs("run", "user").
		WillReturnRows(sqlmock.NewRows([]string{"status", "plan_version", "plan_status"}).AddRow(f.status, f.version, f.planStatus))
}

func expectSkillCancellation(mock sqlmock.Sqlmock, skillStatus string) {
	mock.ExpectQuery("SELECT skill_id,skill_status,usage_contribution FROM ai_run .*FOR UPDATE").WithArgs("root", "user").
		WillReturnRows(sqlmock.NewRows([]string{"skill_id", "skill_status", "usage_contribution"}).AddRow("project_onboarding", skillStatus, nil))
	mock.ExpectExec("INSERT INTO ai_usage_daily").WithArgs(sqlmock.AnyArg(), "user", "project_onboarding", 1, 0, 0).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE ai_run SET skill_status").WithArgs("cancelled", sqlmock.AnyArg(), "root").WillReturnResult(sqlmock.NewResult(0, 1))
}

func TestRejectPlanRejectsStaleCardsAndConflictingStates(t *testing.T) {
	for _, input := range []struct {
		name string
		f    stopRunFixture
	}{
		{"旧卡片已被替代", stopRunFixture{"root", "new-run", "pending_confirmation", "pending_confirmation", "superseded", 1}},
		{"版本已变化", stopRunFixture{"root", "run", "pending_confirmation", "pending_confirmation", "pending_confirmation", 2}},
		{"计划指针缺失", stopRunFixture{"root", "", "pending_confirmation", "pending_confirmation", "pending_confirmation", 1}},
		{"根执行缺失", stopRunFixture{status: "pending_confirmation", planStatus: "pending_confirmation", version: 1}},
		{"不能拒绝正在生成的清单", stopRunFixture{"root", "run", "running", "running", "pending_confirmation", 1}},
		{"新补问正在处理时旧卡片不能取消根执行", stopRunFixture{"root", "run", "running", "pending_confirmation", "pending_confirmation", 1}},
		{"确认先获得锁后不能拒绝执行", stopRunFixture{"root", "run", "pending_confirmation", "executing", "executing", 1}},
		{"旧运行的取消不能作为新清单的重复拒绝", stopRunFixture{"root", "new-run", "pending_confirmation", "cancelled", "cancelled", 1}},
		{"不能拒绝已失效清单", stopRunFixture{"root", "run", "pending_confirmation", "pending_confirmation", "superseded", 1}},
		{"不能拒绝补问", stopRunFixture{"root", "run", "awaiting_input", "awaiting_input", "awaiting_input", 1}},
		{"不能取消待核实根执行", stopRunFixture{"root", "run", "unknown", "pending_confirmation", "pending_confirmation", 1}},
	} {
		t.Run(input.name, func(t *testing.T) {
			s, mock := mockStore(t)
			expectStopRun(mock, input.f)
			mock.ExpectRollback()
			if err := s.RejectPlan(context.Background(), "user", "run", 1); !errors.Is(err, common.ErrRequestConflict) {
				t.Fatalf("旧卡片或状态冲突必须在任何状态和统计写入前拒绝: %v", err)
			}
		})
	}
}

func TestRejectPlanUpdatesSkillOnceAndPreservesIdempotency(t *testing.T) {
	s, mock := mockStore(t)
	expectStopRun(mock, stopRunFixture{"root", "run", "pending_confirmation", "pending_confirmation", "pending_confirmation", 1})
	expectSkillCancellation(mock, "pending_confirmation")
	mock.ExpectExec("UPDATE ai_run SET status='cancelled',plan_status='cancelled'").WithArgs(sqlmock.AnyArg(), "run").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	if err := s.RejectPlan(context.Background(), "user", "run", 1); err != nil {
		t.Fatal(err)
	}
	expectStopRun(mock, stopRunFixture{"root", "run", "cancelled", "cancelled", "cancelled", 1})
	mock.ExpectCommit()
	if err := s.RejectPlan(context.Background(), "user", "run", 1); err != nil {
		t.Fatalf("重复拒绝不能重复写状态和统计: %v", err)
	}
}

func TestCancelRunOnlyStopsActiveWorkAndIsIdempotent(t *testing.T) {
	for _, status := range []string{"running", "executing", "cancelling", "cancelled", "succeeded", "failed", "unknown"} {
		t.Run(status, func(t *testing.T) {
			s, mock := mockStore(t)
			expectStopRun(mock, stopRunFixture{status: status})
			if status == "running" || status == "executing" {
				mock.ExpectExec("UPDATE ai_run SET status='cancelling'").WithArgs("run").WillReturnResult(sqlmock.NewResult(0, 1))
			}
			mock.ExpectCommit()
			if err := s.CancelRun(context.Background(), "user", "run"); err != nil {
				t.Fatal(err)
			}
		})
	}
	s, mock := mockStore(t)
	expectStopRun(mock, stopRunFixture{"root", "run", "pending_confirmation", "pending_confirmation", "pending_confirmation", 1})
	mock.ExpectRollback()
	if err := s.CancelRun(context.Background(), "user", "run"); !errors.Is(err, common.ErrRequestConflict) {
		t.Fatalf("待确认清单必须通过带版本的拒绝操作关闭: %v", err)
	}
}

func TestCancelAwaitingInputCannotAffectNewerWork(t *testing.T) {
	for _, latest := range []string{"run", "new-run"} {
		t.Run(latest, func(t *testing.T) {
			s, mock := mockStore(t)
			expectStopRun(mock, stopRunFixture{"root", "old-plan", "awaiting_input", "awaiting_input", "awaiting_input", 0})
			mock.ExpectQuery("SELECT id FROM ai_run WHERE conversation_id=.*skill_execution_id=.*ORDER BY created_at DESC,id DESC LIMIT 1 FOR UPDATE").WithArgs("conversation", "user", "root").
				WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(latest))
			if latest != "run" {
				mock.ExpectRollback()
				if err := s.CancelRun(context.Background(), "user", "run"); !errors.Is(err, common.ErrRequestConflict) {
					t.Fatalf("旧补问不能取消新任务: %v", err)
				}
				return
			}
			mock.ExpectExec("UPDATE ai_run SET plan_status='superseded'").WithArgs("old-plan").WillReturnResult(sqlmock.NewResult(0, 1))
			expectSkillCancellation(mock, "awaiting_input")
			mock.ExpectExec("UPDATE ai_run SET status='cancelled',plan_status='cancelled'").WithArgs(sqlmock.AnyArg(), "run").WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectCommit()
			if err := s.CancelRun(context.Background(), "user", "run"); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestCancelAwaitingInputRejectsChangedSkillState(t *testing.T) {
	s, mock := mockStore(t)
	expectStopRun(mock, stopRunFixture{"root", "new-plan", "running", "awaiting_input", "awaiting_input", 0})
	mock.ExpectRollback()
	if err := s.CancelRun(context.Background(), "user", "run"); !errors.Is(err, common.ErrRequestConflict) {
		t.Fatalf("根执行已有新进展时不能再取消旧补问: %v", err)
	}
}

func TestRejectPlanRollsBackAllChangesOnFailure(t *testing.T) {
	s, mock := mockStore(t)
	expectStopRun(mock, stopRunFixture{"root", "run", "pending_confirmation", "pending_confirmation", "pending_confirmation", 1})
	expectSkillCancellation(mock, "pending_confirmation")
	want := errors.New("保存取消结果失败")
	mock.ExpectExec("UPDATE ai_run SET status='cancelled',plan_status='cancelled'").WillReturnError(want)
	mock.ExpectRollback()
	if err := s.RejectPlan(context.Background(), "user", "run", 1); !errors.Is(err, want) {
		t.Fatalf("状态和统计必须同事务回滚: %v", err)
	}
}
