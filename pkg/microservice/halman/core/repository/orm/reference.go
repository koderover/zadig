package orm

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
)

// GetRunReference 只读取归属、时间及 Skill 关联；调用方需要清单或上下文时使用 GetRun。
func (s *Store) GetRunReference(ctx context.Context, user, id string, cutoff time.Time) (*models.Run, error) {
	var r models.Run
	err := s.DB.WithContext(ctx).Raw(
		"SELECT r.id,r.conversation_id,r.created_at,r.skill_id,r.skill_execution_id,r.skill_status"+
			" FROM ai_run r JOIN ai_conversation c ON c.id=r.conversation_id"+
			" WHERE r.id=? AND r.user_id=? AND c.user_id=? AND c.deleted_at IS NULL AND r.created_at>=?",
		id, user, user, cutoff,
	).Row().Scan(&r.ID, &r.ConversationID, &r.CreatedAt, &r.SkillID, &r.SkillExecutionID, &r.SkillStatus)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, common.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}
