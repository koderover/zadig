package orm

import (
	"context"
	"time"

	"gorm.io/gorm"
)

// 只更新等待中的根执行，终态贡献此前为零；报告继续保留不含聊天正文的日聚合。
func cancelWaitingSkills(ctx context.Context, tx *gorm.DB, user, conversation string) error {
	where := "r.user_id=? AND c.deleted_at IS NULL AND r.skill_execution_id=r.id AND r.skill_status IN ('awaiting_input','pending_confirmation') AND r.conversation_id=?"
	args := []any{user, conversation}

	day := time.Now().In(time.Local).Format("2006-01-02")
	countArgs := append([]any{day}, args...)
	if err := tx.WithContext(ctx).Exec(
		"INSERT INTO ai_usage_daily (date,user_id,skill_id,terminal_count) SELECT ?,r.user_id,r.skill_id,"+
			"COUNT(*) FROM ai_run r JOIN ai_conversation c ON c.id=r.conversation_id WHERE "+where+" GROUP BY r.user_id,r.skill_id ON DUPLICATE KEY UPDATE terminal_count=terminal_count+VALUES(terminal_count)",
		countArgs...,
	).
		Error; err != nil {
		return err
	}

	updateArgs := append([]any{day}, args...)
	err := tx.WithContext(ctx).Exec(
		"UPDATE ai_run r"+
			" JOIN ai_conversation c ON c.id=r.conversation_id"+
			" SET r.skill_status='cancelled',r.usage_contribution=JSON_SET(COALESCE(r.usage_contribution,"+
			"JSON_OBJECT()),'$.date',?,'$.completed',0,'$.terminal',1,'$.completed_elapsed_ms',0) WHERE "+where,
		updateArgs...,
	).
		Error
	return err
}

// 调用方决定如何映射查询错误，锁条件与事务中的加锁顺序保持一致。
func lockActiveConversation(tx *gorm.DB, user, id string) error {
	var found string
	return tx.Raw("SELECT id FROM ai_conversation WHERE id=? AND user_id=? AND deleted_at IS NULL FOR UPDATE", id, user).Row().Scan(&found)
}
