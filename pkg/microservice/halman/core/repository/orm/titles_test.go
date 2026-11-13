package orm

import (
	"context"
	"database/sql/driver"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
)

type titleSourceTime struct{ earliest time.Time }

func (a titleSourceTime) Match(value driver.Value) bool {
	stamp, ok := value.(time.Time)
	return ok && !stamp.Before(a.earliest)
}

func TestFirstQuestionSetsTitleSourceWithFallback(t *testing.T) {
	for _, initial := range []string{"新会话", "New conversation", "已有标题"} {
		t.Run(initial, func(t *testing.T) {
			s, mock := mockStore(t)
			source := time.Now().UTC().Add(-time.Hour).Truncate(time.Microsecond)
			var savedSource any = source
			var expectedSource any = source
			wantTitle := initial
			if initial == "新会话" || initial == "New conversation" {
				savedSource = nil
				expectedSource = titleSourceTime{earliest: time.Now().UTC().Add(-time.Second)}
				wantTitle = "帮我分析支付服务启动失败"
			}
			req := models.RunRequest{RequestID: "req", Message: "  帮我分析支付服务启动失败  "}
			mock.ExpectBegin()
			mock.ExpectQuery("SELECT user_id,title,title_source_at .*FOR UPDATE").WithArgs("conversation").WillReturnRows(sqlmock.NewRows([]string{"user_id", "title", "title_source_at"}).AddRow("user", initial, savedSource))
			mock.ExpectQuery("SELECT id,request_hash FROM ai_run").WithArgs("user", "req").WillReturnRows(sqlmock.NewRows([]string{"id", "hash"}))
			mock.ExpectQuery("SELECT COUNT.* FROM ai_run").WithArgs("user").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
			mock.ExpectExec("INSERT INTO ai_run").WithArgs(sqlmock.AnyArg(), "user", "conversation", "req", common.RequestFingerprint("conversation", req), "{}", "model-id", "model-name", sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectExec("UPDATE ai_conversation SET title=.*,title_source_at=").WithArgs(wantTitle, expectedSource, sqlmock.AnyArg(), "conversation").WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectCommit()
			if _, created, err := s.BeginRun(context.Background(), "user", "conversation", req, "model-id", "model-name"); err != nil || !created {
				t.Fatalf("首次标题及来源时间写入失败: %v", err)
			}
		})
	}
}

func TestGeneratedTitleCannotOverwriteDeletedOrRefreshedConversation(t *testing.T) {
	s, mock := mockStore(t)
	r := &models.Run{ConversationID: "conversation", CreatedAt: time.Now().UTC().Truncate(time.Microsecond)}
	mock.ExpectExec("UPDATE ai_conversation SET title=.*WHERE id=.*user_id=.*deleted_at IS NULL AND title_source_at=").WithArgs("AI 标题", "conversation", "user", r.CreatedAt).WillReturnResult(sqlmock.NewResult(0, 0))
	if updated, err := s.SetGeneratedTitle(context.Background(), "user", r, "AI 标题"); err != nil || updated {
		t.Fatalf("迟到标题不得覆盖新状态: updated=%v err=%v", updated, err)
	}
}

func TestCreateConversationEnglishDefaultTitle(t *testing.T) {
	store, mock := mockStore(t)
	mock.ExpectExec("INSERT INTO `ai_conversation`").WillReturnResult(sqlmock.NewResult(1, 1))
	conversation, err := store.CreateConversation(context.Background(), "user", "en-us")
	if err != nil || conversation.Title != "New conversation" {
		t.Fatalf("新会话标题应使用系统语言: %+v %v", conversation, err)
	}
}
