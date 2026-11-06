package orm

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
)

func TestRunReferenceKeepsOwnershipAndRetentionWithoutReadingJSON(t *testing.T) {
	query := regexp.QuoteMeta("SELECT r.id,r.conversation_id,r.created_at,r.skill_id,r.skill_execution_id,r.skill_status" +
		" FROM ai_run r JOIN ai_conversation c ON c.id=r.conversation_id" +
		" WHERE r.id=? AND r.user_id=? AND c.user_id=? AND c.deleted_at IS NULL AND r.created_at>=?")
	for _, outcome := range []string{"有效记录", "无权查看或已到期", "数据库失败"} {
		t.Run(outcome, func(t *testing.T) {
			store, mock := mockStore(t)
			now := time.Now().UTC()
			cutoff := now.AddDate(0, 0, -90)
			expect := mock.ExpectQuery(query).WithArgs("run", "user", "user", cutoff)
			columns := []string{"id", "conversation_id", "created_at", "skill_id", "skill_execution_id", "skill_status"}
			var want error
			switch outcome {
			case "有效记录":
				expect.WillReturnRows(sqlmock.NewRows(columns).AddRow("run", "conversation", now, "project_onboarding", "root", "pending_confirmation"))
			case "无权查看或已到期":
				expect.WillReturnRows(sqlmock.NewRows(columns))
				want = common.ErrNotFound
			default:
				want = errors.New("数据库不可用")
				expect.WillReturnError(want)
			}
			r, err := store.GetRunReference(context.Background(), "user", "run", cutoff)
			if !errors.Is(err, want) {
				t.Fatalf("轻量查询不能丢失原有错误语义: %v", err)
			}
			if want == nil && (r.ID != "run" || r.ConversationID != "conversation" || r.SkillExecutionID == nil || *r.SkillExecutionID != "root" || r.SkillStatus == nil || *r.SkillStatus != "pending_confirmation" || !r.CreatedAt.Equal(now) || len(r.Plan) != 0) {
				t.Fatalf("只应返回前置检查需要的字段: %+v", r)
			}
		})
	}
}
