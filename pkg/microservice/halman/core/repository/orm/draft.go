package orm

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
)

// LatestSavedSkillRun 只定位本会话的操作快照；读取结果仍须按当前权限验权。
func (s *Store) LatestSavedSkillRun(ctx context.Context, user, conversation string, cutoff, before time.Time) (*models.Run, error) {
	var id string
	err := s.DB.WithContext(ctx).Raw(
		"SELECT r.id FROM ai_run r JOIN ai_conversation c ON c.id=r.conversation_id"+
			" WHERE r.user_id=? AND c.user_id=? AND c.deleted_at IS NULL AND r.conversation_id=?"+
			" AND r.created_at>=? AND r.created_at<? AND r.question_source_run_id IS NULL"+
			" AND r.skill_execution_id IS NOT NULL AND r.plan IS NOT NULL"+
			" ORDER BY r.created_at DESC,r.id DESC LIMIT 1",
		user, user, conversation, cutoff, before,
	).Row().Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, common.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	r, err := s.GetRun(ctx, user, id, cutoff)
	if err != nil {
		return nil, err
	}
	if r.ConversationID != conversation || r.SkillExecutionID == nil {
		return nil, common.ErrRequestConflict
	}
	root, err := s.GetRunReference(ctx, user, *r.SkillExecutionID, cutoff)
	if err != nil {
		return nil, err
	}
	if root.ConversationID != conversation || root.SkillID == "" || root.SkillID != r.SkillID ||
		root.SkillExecutionID == nil || *root.SkillExecutionID != root.ID {
		return nil, common.ErrRequestConflict
	}
	// 子轮次的状态可能早于取消或执行完成，是否可继续以根执行为准。
	r.SkillStatus = root.SkillStatus
	return r, nil
}

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
