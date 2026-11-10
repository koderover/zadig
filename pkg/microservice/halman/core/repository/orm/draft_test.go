package orm

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
)

func TestLatestSavedSkillRunLookupErrors(t *testing.T) {
	dbErr := errors.New("database unavailable")
	for _, missing := range []bool{true, false} {
		t.Run(map[bool]string{true: "no_snapshot", false: "database_error"}[missing], func(t *testing.T) {
			store, mock := mockStore(t)
			before := time.Now().UTC()
			cutoff := before.Add(-time.Hour)
			query := mock.ExpectQuery("SELECT r.id FROM ai_run r JOIN ai_conversation.*r.user_id=.*c.user_id=.*c.deleted_at IS NULL.*r.conversation_id=.*r.created_at>=.*r.created_at<.*r.question_source_run_id IS NULL.*r.skill_execution_id IS NOT NULL.*r.plan IS NOT NULL.*ORDER BY r.created_at DESC,r.id DESC LIMIT 1").
				WithArgs("user", "user", "conversation", cutoff, before)
			want := dbErr
			if missing {
				query.WillReturnRows(sqlmock.NewRows([]string{"id"}))
				want = common.ErrNotFound
			} else {
				query.WillReturnError(dbErr)
			}
			got, err := store.LatestSavedSkillRun(context.Background(), "user", "conversation", cutoff, before)
			if got != nil || !errors.Is(err, want) {
				t.Fatalf("快照不存在与数据库故障应分别返回，不能创建新草稿: %+v %v", got, err)
			}
		})
	}
}
