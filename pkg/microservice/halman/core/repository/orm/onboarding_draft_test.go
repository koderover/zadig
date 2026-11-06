package orm

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
)

func TestOnboardingDraftSupersedesOldConfirmationAtomically(t *testing.T) {
	for _, fail := range []bool{false, true} {
		s, mock := mockStore(t)
		mock.ExpectBegin()
		mock.ExpectQuery("SELECT id FROM ai_conversation").WithArgs("conversation", "user").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("conversation"))
		mock.ExpectQuery("SELECT active_plan_run_id,skill_status FROM ai_run").WithArgs("root", "user", sqlmock.AnyArg()).WillReturnRows(sqlmock.NewRows([]string{"active", "state"}).AddRow("old-plan", "running"))
		mock.ExpectQuery("SELECT status FROM ai_run").WithArgs("current", "user", "conversation").WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("running"))
		mock.ExpectExec("UPDATE ai_run SET plan_status='superseded'").WithArgs("old-plan").WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec("UPDATE ai_run SET active_plan_run_id=NULL").WithArgs("root", "user").WillReturnResult(sqlmock.NewResult(0, 1))
		save := mock.ExpectExec("UPDATE ai_run SET plan=.*plan_status=").WithArgs(sqlmock.AnyArg(), "awaiting_input", "current", "user")
		want := errors.New("草稿保存失败")
		if fail {
			save.WillReturnError(want)
			mock.ExpectRollback()
		} else {
			save.WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectCommit()
		}
		err := s.SaveDraft(context.Background(), "user", &models.Run{ID: "current", ConversationID: "conversation"}, "root", time.Now().Add(-time.Hour), models.OperationPlan{Draft: &models.OnboardingInput{ExpectedServiceCount: 3}, MissingFields: []string{"web 的配置路径"}})
		if (err != nil) != fail || (fail && !errors.Is(err, want)) {
			t.Fatalf("旧卡片失效与草稿必须同事务提交或回滚: %v", err)
		}
	}
}

func TestOnboardingDraftCannotOverwriteCancellation(t *testing.T) {
	s, mock := mockStore(t)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id FROM ai_conversation").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("conversation"))
	mock.ExpectQuery("SELECT active_plan_run_id,skill_status FROM ai_run").WillReturnRows(sqlmock.NewRows([]string{"active", "state"}).AddRow(nil, "running"))
	mock.ExpectQuery("SELECT status FROM ai_run").WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("cancelling"))
	mock.ExpectRollback()
	if err := s.SaveDraft(context.Background(), "user", &models.Run{ID: "current", ConversationID: "conversation"}, "root", time.Now().Add(-time.Hour), models.OperationPlan{}); !errors.Is(err, common.ErrRequestConflict) {
		t.Fatalf("正在取消时不得重写草稿或确认指针: %v", err)
	}
}
