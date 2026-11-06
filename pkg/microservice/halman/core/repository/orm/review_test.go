package orm

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"trpc.group/trpc-go/trpc-agent-go/event"
	"trpc.group/trpc-go/trpc-agent-go/model"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
)

func TestHistoryVersionsAndFilteredPageCursor(t *testing.T) {
	for _, trace := range []bool{false, true} {
		t.Run(map[bool]string{false: "默认隐藏旧回答", true: "显式查看旧版本"}[trace], func(t *testing.T) {
			s, mock := mockStore(t)
			now := time.Now().UTC().Truncate(time.Microsecond)
			mock.ExpectQuery("SELECT `id` FROM `ai_conversation`").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("c"))
			e := event.NewResponseEvent("inv", "assistant", &model.Response{Choices: []model.Choice{{Message: model.NewAssistantMessage("旧回答")}}})
			e.RequestID = "old"
			mock.ExpectQuery("SELECT id,event,created_at FROM session_events").WillReturnRows(sqlmock.NewRows([]string{"id", "event", "created_at"}).
				AddRow(2, []byte(common.JSON(e)), now).
				AddRow(1, []byte(common.JSON(e)), now))
			mock.ExpectQuery("SELECT r.id,COALESCE").WillReturnRows(sqlmock.NewRows([]string{
				"id",
				"origin",
				"version",
				"regenerated",
				"question_available",
				"latest",
			}).
				AddRow("old", "old", 1, false, true, "new"))
			items, next, err := s.Messages(context.Background(), "u", "c", "", 1, now.Add(-time.Hour), trace)
			if err != nil || next == "" || len(items) != map[bool]int{false: 0, true: 1}[trace] {
				t.Fatalf("过滤后空页也必须推进游标: %+v %s %v", items, next, err)
			}

			cursor, err := common.DecodeCursor(next)
			if err != nil || cursor.ID != "2" {
				t.Fatalf("游标必须来自原始页边界: %+v %v", cursor, err)
			}
		})
	}
}

func TestHistoryKeepsVisibleReasoningWithLatestAnswer(t *testing.T) {
	s, mock := mockStore(t)
	now := time.Now().UTC().Truncate(time.Microsecond)
	message := model.NewAssistantMessage("构建失败，请检查依赖配置")
	message.ReasoningContent = "先检查失败步骤和日志证据"
	e := event.NewResponseEvent("inv", "assistant", &model.Response{Choices: []model.Choice{{Message: message}}})
	e.RequestID = "run"
	mock.ExpectQuery("SELECT `id` FROM `ai_conversation`").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("conversation"))
	mock.ExpectQuery("SELECT id,event,created_at FROM session_events").WillReturnRows(sqlmock.NewRows([]string{"id", "event", "created_at"}).AddRow(1, []byte(common.JSON(e)), now))
	mock.ExpectQuery("SELECT r.id,COALESCE").WillReturnRows(sqlmock.NewRows([]string{"id", "origin", "version", "regenerated", "question_available", "latest"}).AddRow("run", "run", 1, false, true, "run"))
	items, _, err := s.Messages(context.Background(), "user", "conversation", "", 50, now.Add(-time.Hour), false)
	if err != nil || len(items) != 1 || len(items[0].Messages) != 1 || items[0].Messages[0].ReasoningContent != message.ReasoningContent {
		t.Fatalf("历史消息应保留最新回答及其可见推理: %+v %v", items, err)
	}
	if !strings.Contains(string(common.JSON(items)), `"reasoning_content":"先检查失败步骤和日志证据"`) {
		t.Fatal("前端应能从历史消息恢复可见推理")
	}
}

func TestDeleteProtectsUnknownResult(t *testing.T) {
	s, mock := mockStore(t)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id FROM ai_conversation .*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"id"}).
		AddRow("c"))
	mock.ExpectQuery("SELECT COUNT.*status IN .*'unknown'.*OR plan_status='unknown'").WillReturnRows(sqlmock.NewRows([]string{"count"}).
		AddRow(1))
	mock.ExpectRollback()
	if err := s.DeleteConversation(context.Background(), "u", "c"); !errors.Is(err, common.ErrBusy) {
		t.Fatalf("未核实写结果不能删除: %v", err)
	}
}

func TestFeedbackIsVersionedAtomicAndIdempotent(t *testing.T) {
	for _, mode := range []string{
		"new",
		"same",
		"changed",
		"unavailable",
		"aggregate_failure",
	} {
		t.Run(mode, func(t *testing.T) {
			s, mock := mockStore(t)
			mock.ExpectBegin()
			mock.ExpectQuery("SELECT id FROM ai_conversation .*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"id"}).
				AddRow("c"))
			mock.ExpectQuery("SELECT answer_version,status,skill_id,answer_available").WillReturnRows(sqlmock.NewRows([]string{
				"version",
				"status",
				"skill",
				"available",
			}).
				AddRow(2, "succeeded", "workflow_diagnosis", mode != "unavailable"))
			if mode != "unavailable" {
				previous := sqlmock.NewRows([]string{"helpful", "created"})
				if mode == "same" || mode == "changed" {
					previous.AddRow(mode == "same", time.Now())
				}

				mock.ExpectQuery("SELECT helpful,created_at FROM ai_feedback").WithArgs("u", "r", 2).WillReturnRows(previous)
			}

			if mode == "new" || mode == "aggregate_failure" {
				mock.ExpectExec("INSERT INTO ai_feedback").WithArgs(sqlmock.AnyArg(), "u", "r", 2, true, sqlmock.AnyArg()).
					WillReturnResult(sqlmock.NewResult(0, 1))
				aggregate := mock.ExpectExec("INSERT INTO ai_usage_daily")
				if mode == "aggregate_failure" {
					aggregate.WillReturnError(errors.New("聚合失败"))
				} else {
					aggregate.WillReturnResult(sqlmock.NewResult(0, 1))
				}
			}

			if mode == "new" {
				mock.ExpectCommit()
			} else {
				mock.ExpectRollback()
			}

			err := s.Feedback(context.Background(), "u", &models.Run{ID: "r", ConversationID: "c"}, 2, true)
			if (err == nil) != (mode == "new" || mode == "same") {
				t.Fatalf("反馈不能重复计数，聚合失败必须回滚: %v", err)
			}
		})
	}
}

func TestRecoveryExcludesLiveRunAndDoesNotReplay(t *testing.T) {
	for _, state := range []string{"running", "executing", "cancelling"} {
		t.Run(state, func(t *testing.T) {
			s, mock := mockStore(t)
			s.SQLDB.SetMaxOpenConns(1)
			mock.ExpectQuery("SELECT id,user_id,conversation_id,skill_execution_id .*id NOT IN .*LIMIT 20").WithArgs("live").
				WillReturnRows(sqlmock.NewRows([]string{
					"id",
					"user",
					"conversation",
					"root",
				}).
					AddRow("orphan", "u", "c", nil))
			mock.ExpectBegin()
			mock.ExpectQuery("SELECT id FROM ai_conversation .*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"id"}).
				AddRow("c"))
			mock.ExpectQuery("SELECT status,plan_status,question_source_run_id FROM ai_run .*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"status", "plan", "question_source"}).
				AddRow(state, nil, nil))
			next := map[string]string{"running": "interrupted", "executing": "unknown", "cancelling": "cancelled"}[state]
			mock.ExpectExec("UPDATE ai_run SET status=.*plan_status=CASE").WithArgs(next, state == "executing", sqlmock.AnyArg(), sqlmock.AnyArg(), "orphan").
				WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectCommit()
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			if count, err := s.RecoverAbandoned(ctx, []string{"live"}); err != nil || count != 1 {
				t.Fatalf("孤儿恢复不能占着结果集再次取连接: %d %v", count, err)
			}
		})
	}
}

func TestRecoveryOfRegeneratedAnswerPreservesWaitingSkill(t *testing.T) {
	s, mock := mockStore(t)
	mock.ExpectQuery("SELECT id,user_id,conversation_id,skill_execution_id").WillReturnRows(sqlmock.NewRows([]string{
		"id",
		"user",
		"conversation",
		"root",
	}).
		AddRow("regenerated", "u", "c", "root"))
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id FROM ai_conversation .*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"id"}).
		AddRow("c"))
	mock.ExpectQuery("SELECT id FROM ai_run .*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("root"))
	mock.ExpectQuery("SELECT status,plan_status,question_source_run_id FROM ai_run .*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"status", "plan", "question_source"}).
		AddRow("running", nil, "original"))
	mock.ExpectQuery("SELECT skill_id,skill_status,usage_contribution").WillReturnRows(sqlmock.NewRows([]string{"skill", "goal", "contribution"}).
		AddRow("project_onboarding", "pending_confirmation", nil))
	mock.ExpectExec("UPDATE ai_run SET skill_status=.*usage_contribution").WithArgs("pending_confirmation", sqlmock.AnyArg(), "root").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE ai_run SET status=.*plan_status=CASE").WithArgs("interrupted", false, sqlmock.AnyArg(), sqlmock.AnyArg(), "regenerated").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	if n, err := s.RecoverAbandoned(context.Background(), nil); err != nil || n != 1 {
		t.Fatalf("重新回答中断不能结束原业务目标或增加终态分母: %d %v", n, err)
	}
}

func TestCleanupStopsOnCancellationAndDoesNotTouchAudit(t *testing.T) {
	s, _ := mockStore(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	stats, err := s.CleanupHistory(ctx, time.Now())
	if !errors.Is(err, context.Canceled) || stats.Batches != 0 {
		t.Fatalf("取消后不能继续清理: %+v %v", stats, err)
	}

	for _, job := range historyCleanupJobs(time.Now()) {
		if strings.Contains(job.query, "audit") || !strings.Contains(job.query, "LIMIT ") {
			t.Fatalf("清理只能访问白名单、分批处理: %s", job.query)
		}

		if strings.Contains(job.query, "session_") || strings.Contains(job.query, "app_states") || strings.Contains(job.query, "user_states") {
			if !strings.Contains(job.query, "app_name=?") || !strings.Contains(job.query, "LIMIT 32") {
				t.Fatalf("框架正文必须限制应用及批大小: %s", job.query)
			}
		}
	}
}

func TestDeletedCleanupProtectsUnresolvedBeforeDeletingMessages(t *testing.T) {
	s, mock := mockStore(t)
	mock.ExpectQuery("SELECT c.id,c.user_id .*NOT EXISTS .*'unknown'.*plan_status='unknown'").WillReturnRows(sqlmock.NewRows([]string{"id", "user"}))
	if n, err := s.cleanupDeletedConversation(context.Background()); err != nil || n != 0 {
		t.Fatalf("不能先删未知结果的消息: %d %v", n, err)
	}
}

func TestDetachedUnknownExpiresWithoutDeletingRecentMessages(t *testing.T) {
	s, mock := mockStore(t)
	cutoff := time.Now().AddDate(0, 0, -90)
	mock.ExpectQuery("SELECT child.id .*LEFT JOIN ai_run root .*LIMIT 50").WithArgs(cutoff).WillReturnRows(sqlmock.NewRows([]string{"id"}).
		AddRow("child"))
	mock.ExpectExec("UPDATE ai_run SET status='expired'.*WHERE id IN .*AND .*'unknown'").WithArgs("child").WillReturnResult(sqlmock.NewResult(0, 1))
	if n, err := s.expireDetachedUnknown(context.Background(), cutoff); err != nil || n != 1 {
		t.Fatalf("过期目标失效，但近期消息按自身时间保留: %d %v", n, err)
	}
}

func TestReconciliationRejectsStalePlanUnderLock(t *testing.T) {
	s, mock := mockStore(t)
	root := "root"
	source := &models.Run{
		ID:               "source",
		ConversationID:   "c",
		SkillExecutionID: &root,
		PlanVersion:      1,
		Plan:             common.JSON(models.OperationPlan{}),
	}
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id FROM ai_conversation .*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"id"}).
		AddRow("c"))
	mock.ExpectQuery("SELECT active_plan_run_id FROM ai_run .*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"active"}).
		AddRow("replacement"))
	mock.ExpectRollback()
	if err := s.SaveReconciliation(context.Background(), "u", source, models.OperationPlan{}, "succeeded", time.Now().
		Add(-time.Hour)); !errors.Is(err, common.ErrRequestConflict) {
		t.Fatalf("被替代的计划不能写回状态和完成数: %v", err)
	}
}

func TestRetryRejectsStaleRootBeforeWriting(t *testing.T) {
	s, mock := mockStore(t)
	root := "root"
	source := &models.Run{ID: "source", ConversationID: "c", SkillExecutionID: &root}
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id FROM ai_conversation .*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"id"}).
		AddRow("c"))
	mock.ExpectQuery("SELECT active_plan_run_id,skill_status .*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"active", "skill_status"}).
		AddRow("replacement", "failed"))
	mock.ExpectRollback()
	if _, err := s.CreateRetryPlan(
		context.Background(),
		"u",
		source,
		models.RunRequest{RequestID: "req"},
		models.OperationPlan{},
		time.Now().
			Add(-time.Hour),
	); !errors.Is(err, common.ErrRequestConflict) {
		t.Fatalf("并发新清单获胜后，旧重试不能创建执行: %v", err)
	}
}

func TestRetryCreatesOnlyPendingPlanAndWithdrawsOldFailureContribution(t *testing.T) {
	s, mock := mockStore(t)
	now := time.Now().UTC()
	root := "root"
	original := models.OperationPlan{Items: []models.OperationItem{{ID: "done", Status: "succeeded"}, {ID: "retry", Status: "failed"}}}
	source := &models.Run{
		ID:               "source",
		ConversationID:   "c",
		SkillID:          "project_onboarding",
		SkillExecutionID: &root,
		PlanVersion:      1,
		Plan:             common.JSON(original),
	}
	next := models.OperationPlan{
		RetryOfRunID: "source",
		Items:        []models.OperationItem{{ID: "done", Status: "succeeded"}, {ID: "retry", Status: "pending"}},
	}
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id FROM ai_conversation .*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"id"}).
		AddRow("c"))
	mock.ExpectQuery("SELECT active_plan_run_id,skill_status .*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"active", "status"}).
		AddRow("source", "failed"))
	mock.ExpectQuery("SELECT .*FROM ai_run WHERE id=.*FOR UPDATE").WillReturnRows(sqlmock.NewRows(strings.Split(runColumns, ", ")).
		AddRow(
			"source",
			"old",
			"c",
			"failed",
			"{}",
			now,
			now,
			"",
			0,
			0,
			"project_onboarding",
			"root",
			nil,
			[]byte(source.Plan),
			1,
			"failed",
			nil,
			1,
			nil,
			true,
		))
	mock.ExpectQuery("SELECT COUNT.*FROM ai_run WHERE user_id").WillReturnRows(sqlmock.NewRows([]string{"count"}).
		AddRow(0))
	mock.ExpectExec("INSERT INTO ai_run .*'pending_confirmation'").WillReturnResult(sqlmock.NewResult(0, 1))
	previous := common.JSON(skillContribution{Date: "2026-10-30", Terminal: 1, ActiveMS: 1000})
	mock.ExpectQuery("SELECT skill_id,skill_status,usage_contribution").WillReturnRows(sqlmock.NewRows([]string{"skill", "status", "usage"}).
		AddRow("project_onboarding", "failed", []byte(previous)))
	mock.ExpectExec("UPDATE ai_usage_daily SET terminal_count=terminal_count-.*completed_count=completed_count-").
		WithArgs(1, 0, int64(0), "2026-10-30", "u", "project_onboarding").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE ai_run SET skill_status=.*usage_contribution").WithArgs("pending_confirmation", sqlmock.AnyArg(), "root").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE ai_run SET active_plan_run_id=.*skill_status='pending_confirmation'").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE ai_conversation SET last_active_at").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	r, err := s.CreateRetryPlan(context.Background(), "u", source, models.RunRequest{RequestID: "new"}, next, now.Add(-time.Hour))
	if err != nil || r.Status != "pending_confirmation" || r.ID == source.ID || r.SkillExecutionID == nil || *r.SkillExecutionID != root ||
		string(r.Plan) != string(common.JSON(next)) ||
		r.AnswerAvailable {
		t.Fatalf("重试仅创建待确认清单，不执行动作、不重复计终态: %+v %v", r, err)
	}
}

func TestSaveOperationTransactionFailuresRemainAtomic(t *testing.T) {
	for _, stage := range []string{"begin", "read", "write", "commit"} {
		t.Run(stage, func(t *testing.T) {
			store, mock := mockStore(t)
			failure := errors.New("database failure")
			if stage == "begin" {
				mock.ExpectBegin().WillReturnError(failure)
			} else {
				mock.ExpectBegin()
				query := mock.ExpectQuery("SELECT plan FROM ai_run WHERE id=.* FOR UPDATE").WithArgs("run", "user")
				if stage == "read" {
					query.WillReturnError(failure)
					mock.ExpectRollback()
				} else {
					query.WillReturnRows(sqlmock.NewRows([]string{"plan"}).AddRow(`{"items":[{"id":"service-api","status":"submitting"}]}`))
					write := mock.ExpectExec("UPDATE ai_run SET plan=.*WHERE id=").WithArgs(sqlmock.AnyArg(), "run")
					if stage == "write" {
						write.WillReturnError(failure)
						mock.ExpectRollback()
					} else {
						write.WillReturnResult(sqlmock.NewResult(0, 1))
						mock.ExpectCommit().WillReturnError(failure)
					}
				}
			}
			if err := store.SaveOperation(context.Background(), "user", "run", 0, models.OperationItem{ID: "service-api", Status: "succeeded"}); !errors.Is(err, failure) {
				t.Fatalf("事务 %s 失败不能报告动作已保存: %v", stage, err)
			}
		})
	}
}
