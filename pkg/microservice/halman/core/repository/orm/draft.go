package orm

import (
	"context"
	"time"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
)

func (s *Store) SavedSkillRun(ctx context.Context, user, conversation, rootID string, cutoff time.Time) (*models.Run, error) {
	root, err := s.GetRunReference(ctx, user, rootID, cutoff)
	if err != nil {
		return nil, err
	}

	if root.ConversationID != conversation || root.SkillExecutionID == nil || *root.SkillExecutionID != rootID || root.SkillID == "" {
		return nil, common.ErrNotFound
	}

	if root.SkillStatus != nil &&
		(*root.SkillStatus == "succeeded" || *root.SkillStatus == "cancelled" || *root.SkillStatus == "unknown") {
		return nil, common.ErrRequestConflict
	}

	return scanRun(s.DB.WithContext(ctx).Raw(
		"SELECT "+runColumns+" FROM ai_run WHERE user_id=? AND conversation_id=? AND skill_execution_id=? AND question_source_run_id IS NULL AND created_at>=? AND plan IS NOT NULL AND (plan_status IS NULL OR plan_status IN ('awaiting_input','pending_confirmation')) ORDER BY created_at DESC,id DESC LIMIT 1",
		user,
		conversation,
		rootID,
		cutoff,
	).
		Row())
}
