package orm

import (
	"context"
	"database/sql"
	"time"

	"gorm.io/gorm"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
)

func (s *Store) SaveReconciliation(ctx context.Context, user string, source *models.Run, plan models.OperationPlan, status string, cutoff time.Time) error {
	if source.SkillExecutionID == nil {
		return common.ErrRequestConflict
	}

	root := *source.SkillExecutionID
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockActiveConversation(tx, user, source.ConversationID); err != nil {
			return common.ErrNotFound
		}

		var active sql.NullString
		if err := tx.Raw("SELECT active_plan_run_id FROM ai_run WHERE id=? AND user_id=? AND created_at>=? FOR UPDATE", root, user, cutoff).
			Row().
			Scan(&active); err != nil {
			return common.ErrNotFound
		}

		if active.String != source.ID {
			return common.ErrRequestConflict
		}

		current, err := scanRun(tx.Raw("SELECT "+runColumns+" FROM ai_run WHERE id=? AND user_id=? FOR UPDATE", source.ID, user).
			Row())
		if err != nil {
			return err
		}

		if current.Status != "unknown" || current.PlanVersion != source.PlanVersion || string(current.Plan) != string(source.Plan) ||
			current.CreatedAt.Before(cutoff) {
			return common.ErrRequestConflict
		}

		if err := s.updateSkillUsage(ctx, tx, user, root, status, 0); err != nil {
			return err
		}

		message := ""
		if status == "unknown" {
			message = "尚有已提交动作需要核实，不能直接重试"
		}

		if err := tx.Exec(
			"UPDATE ai_run SET plan=?,status=?,plan_status=?,error=?,finished_at=? WHERE id=?",
			string(common.JSON(plan)),
			status,
			status,
			message,
			time.Now().
				UTC(),
			source.ID,
		).
			Error; err != nil {
			return err
		}

		return nil
	})
}
