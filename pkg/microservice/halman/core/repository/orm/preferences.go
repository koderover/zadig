package orm

import (
	"context"
	"database/sql"
	"errors"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
)

func (s *Store) ModelPreference(ctx context.Context, user string) (models.ModelSelection, error) {
	var preference models.ModelSelection
	err := s.DB.WithContext(ctx).Raw("SELECT model_id,reasoning_effort FROM ai_user_preference WHERE user_id=?", user).
		Row().Scan(&preference.LLMIntegrationID, &preference.ReasoningEffort)
	if errors.Is(err, sql.ErrNoRows) {
		err = nil
	}
	return preference, err
}

func (s *Store) SaveModelPreference(ctx context.Context, user string, preference models.ModelSelection) error {
	db := s.DB.WithContext(ctx)
	if preference.LLMIntegrationID == "" {
		return db.Exec("DELETE FROM ai_user_preference WHERE user_id=?", user).Error
	}
	return db.Exec("INSERT INTO ai_user_preference (user_id,model_id,reasoning_effort) VALUES (?,?,?) "+
		"ON DUPLICATE KEY UPDATE model_id=?,reasoning_effort=?", user,
		preference.LLMIntegrationID, preference.ReasoningEffort, preference.LLMIntegrationID, preference.ReasoningEffort).Error
}
