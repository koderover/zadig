package orm

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
)

func TestLegacyTextClaimsCurrentCardAndExplicitReplyRejectsStaleRevision(t *testing.T) {
	for _, test := range []struct {
		name    string
		reply   *models.UICardReply
		claimed int64
		want    error
	}{
		{name: "legacy reply", claimed: 1},
		{name: "stale explicit reply", reply: &models.UICardReply{CardID: "card", Revision: 1}, want: common.ErrRequestConflict},
	} {
		t.Run(test.name, func(t *testing.T) {
			store, mock := mockStore(t)
			now := time.Now().UTC()
			pattern := "UPDATE ai_ui_card SET status='superseded',revision=revision\\+1 WHERE run_id="
			update := mock.ExpectQuery("SELECT id,status,plan_status FROM ai_run .* FOR UPDATE").WithArgs("conversation", "user", "next")
			update.WillReturnRows(sqlmock.NewRows([]string{"id", "status", "plan_status"}).AddRow("source", "awaiting_input", nil))
			exec := mock.ExpectExec(pattern)
			if test.reply == nil {
				exec.WithArgs("source", "user", now)
			} else {
				exec.WithArgs("source", "user", now, "card", 1)
			}
			exec.WillReturnResult(sqlmock.NewResult(0, test.claimed))
			run := &models.Run{ID: "next", ConversationID: "conversation", CreatedAt: now}
			err := saveUIRun(store.DB.WithContext(context.Background()), "user", models.RunRequest{ChatMessage: true, CardReply: test.reply}, run)
			if !errors.Is(err, test.want) || (run.RepliedCardRunID == "source") != (test.claimed == 1) {
				t.Fatalf("claimed source %q, error %v", run.RepliedCardRunID, err)
			}
		})
	}
}

func TestCardWireSnapshotKeepsSubmissionValuesPrivate(t *testing.T) {
	store, mock := mockStore(t)
	now := time.Now().UTC()
	card := models.UICard{ID: "card", RunID: "source", Type: "resource_selector"}
	mock.ExpectQuery("SELECT c.payload,c.status,c.revision").WithArgs("user", "user", now, now, "source", 51).WillReturnRows(
		sqlmock.NewRows([]string{"payload", "status", "revision", "draft_fields", "run_status", "submission_id", "action_id", "first_run", "attempt_run", "attempt_status", "selected", "latest"}).
			AddRow(string(common.JSON(card)), "submitted", 3, false, "awaiting_input", "submission", "submit", "first", "next", "interrupted", `{"item":{"project_key":"internal-project"}}`, "next"))
	cards, err := store.RunUICards(context.Background(), "user", "source", now)
	if err != nil || len(cards) != 1 {
		t.Fatalf("cards = %#v, %v", cards, err)
	}
	got := cards[0]
	if got.Submission == nil || got.Submission.CurrentAttemptStatus != "interrupted" || got.Submission.CurrentAttemptRunID != "next" || len(got.ResourceSelections) != 1 {
		t.Fatalf("snapshot did not load latest attempt and private selection: %#v", got)
	}
	if matches := string(common.JSON(got)); strings.Contains(matches, "internal-project") {
		t.Fatalf("snapshot leaked submitted resource: %s", matches)
	}
}
