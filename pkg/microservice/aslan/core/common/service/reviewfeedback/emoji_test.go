package reviewfeedback

import (
	"errors"
	"testing"
	"time"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/setting"
)

func TestGitLabWebhookReactionSequence(t *testing.T) {
	pr := models.AIReviewFeedback{CycleStartedAt: time.Now()}
	for _, event := range []struct {
		id           int64
		name, action string
		up, down     int
		changed      bool
	}{
		{1, "thumbsup", "award", 1, 0, true},
		{1, "thumbsup", "award", 1, 0, false},
		{2, "thumbsdown", "award", 1, 1, true},
		{1, "thumbsup", "revoke", 0, 1, true},
		{1, "thumbsup", "revoke", 0, 1, false},
		{1, "thumbsup", "award", 0, 1, false},
		{3, "thumbsup", "revoke", 0, 1, true},
		{3, "thumbsup", "award", 0, 1, false},
		{4, "thumbsup", "award", 1, 1, true},
		{2, "thumbsdown", "revoke", 1, 0, true},
	} {
		if changed := applyGitLabReaction(&pr, event.id, event.name, event.action); changed != event.changed || pr.Up != event.up || pr.Down != event.down {
			t.Fatalf("event %+v: changed=%t, up=%d, down=%d", event, changed, pr.Up, pr.Down)
		}
		if !pr.CycleStartedAt.IsZero() {
			t.Fatal("new webhook must restart any concurrent final reconciliation")
		}
	}
}

func TestGitLabFinalReconciliation(t *testing.T) {
	now := time.Now()
	pr := models.AIReviewFeedback{Provider: setting.SourceFromGitlab, FinalSync: true}
	failed := syncResultUpdate(&pr, now, true, errors.New("API failed"))
	if failed["next_sync_at"] != now.Add(time.Minute) || failed["closed"] != nil || failed["final_sync"] != nil {
		t.Fatalf("failed final reconciliation must remain pending: %v", failed)
	}
	finished := syncResultUpdate(&pr, now, true, nil)
	if finished["closed"] != true || finished["final_sync"] != false || finished["next_sync_at"] != nil {
		t.Fatalf("final reconciliation must freeze without periodic polling: %v", finished)
	}
}
