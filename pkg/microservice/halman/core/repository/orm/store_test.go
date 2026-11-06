package orm

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"trpc.group/trpc-go/trpc-agent-go/event"
	"trpc.group/trpc-go/trpc-agent-go/model"

	halmanconfig "github.com/koderover/zadig/v2/pkg/microservice/halman/config"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
)

func mockStore(t *testing.T) (*Store, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Error(err)
		}

		db.Close()
	})
	store, err := NewStore(db, halmanconfig.DefaultDBMaxConnections)
	if err != nil {
		t.Fatal(err)
	}

	return store, mock
}

func runRows(id string, now time.Time) *sqlmock.Rows {
	return sqlmock.NewRows(strings.Split(runColumns, ", ")).AddRow(id, "req-1", "conversation-1", "running", "{}", now, nil, "", 0, 0, "", nil, nil, nil, 0, nil, nil, 1, nil, false)
}

func TestBeginRunRejectsOtherOwner(t *testing.T) {
	s, mock := mockStore(t)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT user_id,title,title_source_at .* FOR UPDATE").WithArgs("conversation-1").WillReturnRows(sqlmock.NewRows([]string{"user_id", "title", "title_source_at"}).
		AddRow("other-user", "标题", nil))
	mock.ExpectRollback()
	if _, _, err := s.BeginRun(context.Background(), "user-1", "conversation-1", models.RunRequest{RequestID: "req-1", Message: "你好"}, "model", "model"); !errors.Is(err, common.ErrNotFound) {
		t.Fatalf("不应允许访问他人的会话: %v", err)
	}
}

func TestConversationLookupDistinguishesMissingFromDatabaseFailure(t *testing.T) {
	query := regexp.QuoteMeta("SELECT `id` FROM `ai_conversation` WHERE id=? AND user_id=? AND deleted_at IS NULL LIMIT ?")
	dbErr := errors.New("数据库查询失败")
	for _, missing := range []bool{true, false} {
		t.Run(map[bool]string{true: "记录不存在", false: "数据库异常"}[missing], func(t *testing.T) {
			s, mock := mockStore(t)
			expect := mock.ExpectQuery(query).WithArgs("conversation", "user", 1)
			want := dbErr
			if missing {
				expect.WillReturnRows(sqlmock.NewRows([]string{"id"}))
				want = common.ErrNotFound
			} else {
				expect.WillReturnError(dbErr)
			}

			if err := s.CheckConversation(context.Background(), "user", "conversation"); !errors.Is(err, want) {
				t.Fatalf("查询错误不能被当作不存在或成功: %v", err)
			}
		})
	}
}

func TestClaimOperationPropagatesTransactionFailure(t *testing.T) {
	for _, stage := range []string{"begin", "update", "commit"} {
		t.Run(stage, func(t *testing.T) {
			s, mock := mockStore(t)
			// 单连接也必须完成事务，不能在事务内另取池连接。
			s.SQLDB.SetMaxOpenConns(1)
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			dbErr := errors.New("事务失败")
			r := &models.Run{ID: "run", ConversationID: "conversation"}
			begin := mock.ExpectBegin()
			if stage == "begin" {
				begin.WillReturnError(dbErr)
			} else {
				mock.ExpectQuery("SELECT id FROM ai_conversation .*FOR UPDATE").WithArgs("conversation", "user").WillReturnRows(sqlmock.NewRows([]string{"id"}).
					AddRow("conversation"))
				mock.ExpectQuery("SELECT active_plan_run_id .*FOR UPDATE").WithArgs("root").WillReturnRows(sqlmock.NewRows([]string{"active"}).
					AddRow("run"))
				plan := common.JSON(models.OperationPlan{Items: []models.OperationItem{{ID: "item", Status: "pending"}}})
				mock.ExpectQuery("SELECT plan,status .*FOR UPDATE").WithArgs("run").WillReturnRows(sqlmock.NewRows([]string{"plan", "status"}).
					AddRow([]byte(plan), "executing"))
				update := mock.ExpectExec("UPDATE ai_run SET plan").WithArgs(sqlmock.AnyArg(), "run")
				if stage == "update" {
					update.WillReturnError(dbErr)
					mock.ExpectRollback()
				} else {
					update.WillReturnResult(sqlmock.NewResult(0, 1))
					mock.ExpectCommit().WillReturnError(dbErr)
				}
			}

			if err := s.ClaimOperation(ctx, "user", r, "root", "item"); !errors.Is(err, dbErr) {
				t.Fatalf("事务失败时不能视为领取成功: %v", err)
			}
		})
	}
}

func TestBeginRunDuplicateDoesNotWrite(t *testing.T) {
	s, mock := mockStore(t)
	now := time.Now().UTC()
	req := models.RunRequest{RequestID: "req-1", Message: "你好"}
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT user_id,title,title_source_at .* FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"user_id", "title", "title_source_at"}).
		AddRow("user-1", "标题", nil))
	mock.ExpectQuery("SELECT id,request_hash FROM ai_run").WithArgs("user-1", "req-1").WillReturnRows(sqlmock.NewRows([]string{"id", "request_hash"}).
		AddRow("run-1", common.RequestFingerprint("conversation-1", req)))
	mock.ExpectQuery("SELECT .* FROM ai_run WHERE id").WithArgs("run-1").WillReturnRows(runRows("run-1", now))
	mock.ExpectRollback()
	r, created, err := s.BeginRun(context.Background(), "user-1", "conversation-1", req, "model", "model")
	if err != nil || created || r.ID != "run-1" {
		t.Fatalf("重复请求应返回原运行且不写入: %v %v %+v", err, created, r)
	}
}

func TestDeleteRunningConversationRollsBack(t *testing.T) {
	s, mock := mockStore(t)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id FROM ai_conversation .* FOR UPDATE").WithArgs("conversation-1", "user-1").WillReturnRows(sqlmock.NewRows([]string{"id"}).
		AddRow("conversation-1"))
	mock.ExpectQuery("SELECT COUNT.* FROM ai_run").WithArgs("conversation-1").WillReturnRows(sqlmock.NewRows([]string{"count"}).
		AddRow(1))
	mock.ExpectRollback()
	if err := s.DeleteConversation(context.Background(), "user-1", "conversation-1"); !errors.Is(err, common.ErrBusy) {
		t.Fatalf("执行中会话不应被删除: %v", err)
	}
}

func TestFinishRespectsCancellationAndCountsOnce(t *testing.T) {
	s, mock := mockStore(t)
	r := &models.Run{ID: "run-1", ConversationID: "conversation-1", CreatedAt: time.Now().Add(-time.Minute)}
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id FROM ai_conversation .* FOR UPDATE").WithArgs("conversation-1", "user-1").WillReturnRows(sqlmock.NewRows([]string{"id"}).
		AddRow("conversation-1"))
	mock.ExpectQuery("SELECT status FROM ai_run .* FOR UPDATE").WithArgs("run-1").WillReturnRows(sqlmock.NewRows([]string{"status"}).
		AddRow("cancelling"))
	mock.ExpectExec("UPDATE ai_run SET status").WithArgs("cancelled", "运行已取消", sqlmock.AnyArg(), 12, 5, sqlmock.AnyArg(), false, "run-1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO ai_usage_daily").WithArgs(sqlmock.AnyArg(), "user-1", "", 12, 5, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE ai_conversation SET last_active_at").WithArgs(sqlmock.AnyArg(), "conversation-1").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	if err := s.FinishRun(context.Background(), "user-1", r, "succeeded", "", 12, 5); err != nil {
		t.Fatal(err)
	}

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id FROM ai_conversation .* FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"id"}).
		AddRow("conversation-1"))
	mock.ExpectQuery("SELECT status FROM ai_run .* FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"status"}).
		AddRow("cancelled"))
	mock.ExpectRollback()
	if err := s.FinishRun(context.Background(), "user-1", r, "succeeded", "", 12, 5); err != nil {
		t.Fatal(err)
	}
}

func TestHistoryUsesOwnerAndTimeFilterWithoutResourcePermission(t *testing.T) {
	s, mock := mockStore(t)
	now := time.Now().UTC().Truncate(time.Microsecond)
	cutoff := now.AddDate(0, 0, -90)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT `id` FROM `ai_conversation` WHERE id=? AND user_id=? AND deleted_at IS NULL LIMIT ?")).
		WithArgs("conversation-1", "user-1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).
			AddRow("conversation-1"))
	e := event.NewResponseEvent("inv", "assistant", &model.Response{Choices: []model.Choice{{Message: model.NewAssistantMessage("已保存的诊断")}}})
	e.RequestID = "run-1"
	raw, _ := json.Marshal(e)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id,event,created_at"+
		" FROM session_events"+
		" WHERE app_name=?"+
		" AND user_id=?"+
		" AND session_id=? AND deleted_at IS NULL AND created_at>=? ORDER BY created_at DESC,id DESC LIMIT ?")).WithArgs(halmanconfig.AppName, "user-1", "conversation-1", cutoff, 2).
		WillReturnRows(sqlmock.NewRows([]string{"id", "event", "created_at"}).
			AddRow(2, raw, now).
			AddRow(1, raw, now))
	mock.ExpectQuery("SELECT r.id,COALESCE").WithArgs(cutoff, cutoff, "user-1", "conversation-1", cutoff, "run-1").
		WillReturnRows(sqlmock.NewRows([]string{
			"id",
			"origin",
			"version",
			"regenerated",
			"question_available",
			"latest",
		}).
			AddRow("run-1", "run-1", 1, false, true, "run-1"))
	items, next, err := s.Messages(context.Background(), "user-1", "conversation-1", "", 1, cutoff, false)
	if err != nil || len(items) != 1 || items[0].Messages[0].Content != "已保存的诊断" || next == "" {
		t.Fatalf("历史快照分页失败: %+v %s %v", items, next, err)
	}

	c, err := common.DecodeCursor(next)
	if err != nil || c.ID != "2" || !c.At.Equal(now) {
		t.Fatalf("同一时间的分页需要记录行 ID: %+v %v", c, err)
	}
}

func TestSkillSuccessDoesNotCountAgain(t *testing.T) {
	s, mock := mockStore(t)
	mock.ExpectBegin()
	tx := s.DB.WithContext(context.Background()).Begin()
	err := tx.Error
	if err != nil {
		t.Fatal(err)
	}

	previous := skillContribution{
		Date:               "2026-10-28",
		Completed:          1,
		Terminal:           1,
		ActiveMS:           50,
		CompletedElapsedMS: 50,
	}
	mock.ExpectQuery("SELECT skill_id,skill_status,usage_contribution").WithArgs("root", "user").WillReturnRows(sqlmock.NewRows([]string{"skill", "status", "usage"}).
		AddRow("project_onboarding", "succeeded", []byte(common.JSON(previous))))
	mock.ExpectExec("UPDATE ai_usage_daily SET completed_elapsed_ms").WithArgs(int64(20), "2026-10-28", "user", "project_onboarding").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE ai_run SET skill_status").WithArgs("succeeded", sqlmock.AnyArg(), "root").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	if err := s.updateSkillUsage(context.Background(), tx, "user", "root", "failed", 20); err != nil {
		t.Fatal(err)
	}

	if err := tx.Commit().Error; err != nil {
		t.Fatal(err)
	}
}

func TestAwaitingInputDoesNotCountCompletedSkill(t *testing.T) {
	s, mock := mockStore(t)
	mock.ExpectBegin()
	tx := s.DB.WithContext(context.Background()).Begin()
	err := tx.Error
	if err != nil {
		t.Fatal(err)
	}

	mock.ExpectQuery("SELECT skill_id,skill_status,usage_contribution").WillReturnRows(sqlmock.NewRows([]string{"skill", "status", "usage"}).
		AddRow("project_onboarding", "running", nil))
	mock.ExpectExec("UPDATE ai_run SET skill_status").WithArgs("awaiting_input", sqlmock.AnyArg(), "root").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	if err := s.updateSkillUsage(context.Background(), tx, "user", "root", "awaiting_input", time.Second.Milliseconds()); err != nil {
		t.Fatal(err)
	}

	if err := tx.Commit().Error; err != nil {
		t.Fatal(err)
	}
}

func TestFirstSkillSuccessCountsOneCompletedTask(t *testing.T) {
	s, mock := mockStore(t)
	mock.ExpectBegin()
	tx := s.DB.WithContext(context.Background()).Begin()
	err := tx.Error
	if err != nil {
		t.Fatal(err)
	}

	mock.ExpectQuery("SELECT skill_id,skill_status,usage_contribution").WillReturnRows(sqlmock.NewRows([]string{"skill", "status", "usage"}).
		AddRow("service_onboarding", "pending_confirmation", nil))
	mock.ExpectExec("INSERT INTO ai_usage_daily .*terminal_count,completed_count").WithArgs(sqlmock.AnyArg(), "user", "service_onboarding", 1, 1, int64(100)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE ai_run SET skill_status").WithArgs("succeeded", sqlmock.AnyArg(), "root").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	if err := s.updateSkillUsage(context.Background(), tx, "user", "root", "succeeded", 100); err != nil {
		t.Fatal(err)
	}

	if err := tx.Commit().Error; err != nil {
		t.Fatal(err)
	}
}

func TestConfirmationRejectsExpiredAndChangedPlanWithoutWriting(t *testing.T) {
	for _, changed := range []bool{false, true} {
		t.Run(map[bool]string{false: "过期", true: "清单被替换"}[changed], func(t *testing.T) {
			s, mock := mockStore(t)
			now := time.Now().UTC()
			plan := string(common.JSON(models.OperationPlan{Items: []models.OperationItem{{ID: "workflow", Action: "create_workflow", Status: "pending"}}}))
			expires := now.Add(-time.Minute)
			rows := func() *sqlmock.Rows {
				return sqlmock.NewRows(strings.Split(runColumns, ", ")).AddRow(
					"run",
					"request",
					"conversation",
					"pending_confirmation",
					"{}",
					now,
					nil,
					"",
					0,
					0,
					"project_onboarding",
					"root",
					nil,
					plan,
					1,
					"pending_confirmation",
					expires,
					1,
					nil,
					true,
				)
			}
			mock.ExpectQuery("SELECT r.id,.*JOIN ai_conversation").WillReturnRows(rows())
			mock.ExpectBegin()
			mock.ExpectQuery("SELECT id FROM ai_conversation .*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"id"}).
				AddRow("conversation"))
			active := "run"
			if changed {
				active = "new-plan"
			}

			mock.ExpectQuery("SELECT active_plan_run_id .*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"active"}).
				AddRow(active))
			mock.ExpectQuery("SELECT id, request_id,.*FOR UPDATE").WillReturnRows(rows())
			mock.ExpectRollback()
			if _, _, start, err := s.ConfirmPlan(context.Background(), "user", "run", 1, now.Add(-time.Hour)); err == nil || start {
				t.Fatalf("不能提交已经失效的清单: %v %v", start, err)
			}
		})
	}
}

func TestRegeneratedRunKeepsOriginalQuestionAcrossVersions(t *testing.T) {
	for _, again := range []bool{false, true} {
		t.Run(map[bool]string{false: "首次重新回答", true: "再次重新回答"}[again], func(t *testing.T) {
			store, mock := mockStore(t)
			now := time.Now().UTC().Truncate(time.Microsecond)
			root, origin := "skill-root", "original"
			source := &models.Run{ID: origin, AnswerVersion: 1, SkillID: "workflow_diagnosis", SkillExecutionID: &root}
			if again {
				source.ID, source.AnswerVersion, source.QuestionSourceRunID = "latest", 3, &origin
			}
			req := models.RunRequest{RequestID: "regenerate", Message: "重新分析", RegenerateFrom: source}
			mock.ExpectBegin()
			mock.ExpectQuery("SELECT user_id,title,title_source_at .*FOR UPDATE").WithArgs("conversation").WillReturnRows(sqlmock.NewRows([]string{"user", "title", "source"}).AddRow("user", "已有标题", now))
			mock.ExpectQuery("SELECT id,request_hash FROM ai_run").WithArgs("user", req.RequestID).WillReturnRows(sqlmock.NewRows([]string{"id", "hash"}))
			mock.ExpectQuery("SELECT id,COALESCE.*answer_available.*FOR UPDATE").WithArgs("conversation").WillReturnRows(sqlmock.NewRows([]string{"id", "origin", "status", "available"}).AddRow(source.ID, origin, "succeeded", true))
			mock.ExpectQuery("SELECT COUNT.*FROM ai_run").WithArgs("user").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
			mock.ExpectExec("INSERT INTO ai_run \\(id,user_id,conversation_id,request_id,request_hash,status,").WithArgs(sqlmock.AnyArg(), "user", "conversation", req.RequestID, common.RequestFingerprint("conversation", req), "{}", "model", "model", sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectQuery("SELECT resource_scopes FROM ai_run").WithArgs(source.ID).WillReturnRows(sqlmock.NewRows([]string{"scopes"}).AddRow("[]"))
			mock.ExpectExec("UPDATE ai_run SET answer_version=.*question_source_run_id=.*skill_id=").WithArgs(source.AnswerVersion+1, origin, source.SkillID, root, "[]", sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectExec("UPDATE ai_conversation SET title=.*last_active_at=").WithArgs("已有标题", now, sqlmock.AnyArg(), "conversation").WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectCommit()
			r, created, err := store.BeginRun(context.Background(), "user", "conversation", req, "model", "model")
			if err != nil || !created || r.QuestionSourceRunID == nil || *r.QuestionSourceRunID != origin || r.AnswerVersion != source.AnswerVersion+1 || len(r.Plan) != 0 {
				t.Fatalf("重新回答必须保留原问题关联并递增版本，不复制操作清单: %+v %v", r, err)
			}
		})
	}
}

func TestFinishRegeneratedRunDoesNotChangeSkillOutcome(t *testing.T) {
	store, mock := mockStore(t)
	root, question := "root", "original"
	r := &models.Run{ID: "regenerated", ConversationID: "conversation", SkillID: "project_onboarding", SkillExecutionID: &root, QuestionSourceRunID: &question, AnswerAvailable: true, CreatedAt: time.Now().UTC()}
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id FROM ai_conversation .*FOR UPDATE").WithArgs("conversation", "user").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("conversation"))
	mock.ExpectQuery("SELECT id FROM ai_run .*FOR UPDATE").WithArgs(root).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(root))
	mock.ExpectQuery("SELECT status FROM ai_run .*FOR UPDATE").WithArgs(r.ID).WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("running"))
	mock.ExpectQuery("SELECT skill_id,skill_status,usage_contribution").WithArgs(root, "user").WillReturnRows(sqlmock.NewRows([]string{"skill", "status", "contribution"}).AddRow(r.SkillID, "pending_confirmation", nil))
	mock.ExpectExec("UPDATE ai_run SET skill_status=.*usage_contribution=").WithArgs("pending_confirmation", sqlmock.AnyArg(), root).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE ai_run SET status=").WithArgs("succeeded", "", sqlmock.AnyArg(), 10, 2, sqlmock.AnyArg(), true, r.ID).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO ai_usage_daily .*run_count,input_tokens").WithArgs(sqlmock.AnyArg(), "user", r.SkillID, 10, 2, sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE ai_conversation SET last_active_at=").WithArgs(sqlmock.AnyArg(), "conversation").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	if err := store.FinishRun(context.Background(), "user", r, "succeeded", "", 10, 2); err != nil {
		t.Fatal(err)
	}
}
