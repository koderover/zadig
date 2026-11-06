package orm

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
)

func (s *Store) Feedback(ctx context.Context, user string, r *models.Run, version int, helpful bool) error {
	tx := s.DB.WithContext(ctx).Begin()
	err := tx.Error
	if err != nil {
		return err
	}

	defer tx.Rollback()
	var found string
	if err := tx.WithContext(ctx).Raw("SELECT id FROM ai_conversation WHERE id=? AND user_id=? AND deleted_at IS NULL FOR UPDATE", r.ConversationID, user).
		Row().
		Scan(&found); err != nil {
		return common.ErrNotFound
	}

	if r.SkillExecutionID != nil {
		if err := tx.WithContext(ctx).Raw("SELECT id FROM ai_run WHERE id=? FOR UPDATE", *r.SkillExecutionID).Row().Scan(&found); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return common.ErrNotFound
		}
	}

	var actual int
	var status, skill string
	var available bool
	if err := tx.WithContext(ctx).Raw("SELECT answer_version,status,skill_id,answer_available FROM ai_run WHERE id=? AND user_id=? FOR UPDATE", r.ID, user).
		Row().
		Scan(&actual, &status, &skill, &available); err != nil {
		return common.ErrNotFound
	}

	if actual != version || !available {
		return fmt.Errorf("%w: 只能评价已经生成的指定回答版本", common.ErrRequestConflict)
	}

	var previous bool
	var submitted time.Time
	err = tx.WithContext(ctx).Raw(
		"SELECT helpful,created_at FROM ai_feedback WHERE user_id=? AND answer_run_id=? AND answer_version=? FOR UPDATE",
		user,
		r.ID,
		version,
	).
		Row().
		Scan(&previous, &submitted)
	if err == nil {
		if previous == helpful {
			return nil
		}

		return fmt.Errorf("%w: 该回答版本已评价", common.ErrRequestConflict)
	} else if !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	now := time.Now().UTC()
	positive, negative := feedbackCounts(helpful)
	if err := tx.WithContext(ctx).Exec(
		"INSERT INTO ai_feedback (id,user_id,answer_run_id,answer_version,helpful,created_at) VALUES (?,?,?,?,?,?)",
		common.NewID(),
		user,
		r.ID,
		version,
		helpful,
		now,
	).
		Error; err != nil {
		return err
	}

	if err := tx.WithContext(ctx).Exec(
		"INSERT INTO ai_usage_daily (date,user_id,skill_id,positive_count,negative_count)"+
			" VALUES (?,?,?,?,?)"+
			" ON DUPLICATE KEY UPDATE positive_count=positive_count+VALUES(positive_count),"+
			"negative_count=negative_count+VALUES(negative_count)",
		now.In(time.Local).Format("2006-01-02"),
		user,
		skill,
		positive,
		negative,
	).
		Error; err != nil {
		return err
	}

	return tx.Commit().Error
}

func feedbackCounts(helpful bool) (int, int) {
	if helpful {
		return 1, 0
	}

	return 0, 1
}
