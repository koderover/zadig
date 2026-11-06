package orm

import (
	"context"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
)

func (s *Store) TitleNeedsGeneration(ctx context.Context, user string, run *models.Run) (bool, error) {
	var count int64
	err := s.DB.WithContext(ctx).Table("ai_conversation").
		Where("id=? AND user_id=? AND deleted_at IS NULL AND title_source_at=?", run.ConversationID, user, run.CreatedAt).
		Count(&count).Error
	return count > 0, err
}

func (s *Store) SetGeneratedTitle(ctx context.Context, user string, run *models.Run, title string) (bool, error) {
	// 删除或后台重建已经改变标题来源时，不让迟到的模型结果覆盖新状态。
	result := s.DB.WithContext(ctx).Exec(
		"UPDATE ai_conversation SET title=? WHERE id=? AND user_id=? AND deleted_at IS NULL AND title_source_at=?",
		title, run.ConversationID, user, run.CreatedAt,
	)
	return result.RowsAffected > 0, result.Error
}
