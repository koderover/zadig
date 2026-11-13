package orm

import (
	"context"
	"database/sql"
	"time"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
)

func (s *Store) CreateRetryPlan(ctx context.Context, user string, source *models.Run, req models.RunRequest, plan models.OperationPlan, cutoff time.Time) (*models.Run, error) {
	if source.SkillExecutionID == nil {
		return nil, common.ErrRequestConflict
	}

	root := *source.SkillExecutionID
	tx := s.DB.WithContext(ctx).Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}

	defer tx.Rollback()
	if err := lockActiveConversation(tx, user, source.ConversationID); err != nil {
		return nil, common.ErrNotFound
	}

	var active, skillStatus sql.NullString
	if err := tx.Raw(
		"SELECT active_plan_run_id,skill_status FROM ai_run WHERE id=? AND user_id=? AND created_at>=? FOR UPDATE",
		root,
		user,
		cutoff,
	).
		Row().
		Scan(&active, &skillStatus); err != nil {
		return nil, common.ErrNotFound
	}

	if active.String != source.ID || skillStatus.String == "unknown" || skillStatus.String == "succeeded" ||
		skillStatus.String == "cancelled" {
		return nil, common.ErrRequestConflict
	}

	current, err := scanRun(tx.Raw("SELECT "+runColumns+" FROM ai_run WHERE id=? AND user_id=? FOR UPDATE", source.ID, user).
		Row())
	if err != nil {
		return nil, err
	}

	if current.Status != "failed" || current.PlanVersion != source.PlanVersion || string(current.Plan) != string(source.Plan) ||
		current.CreatedAt.Before(cutoff) {
		return nil, common.ErrRequestConflict
	}

	var busy int
	if err := tx.Raw("SELECT COUNT(*) FROM ai_run WHERE user_id=? AND status IN ('running','cancelling','executing')", user).
		Row().
		Scan(&busy); err != nil {
		return nil, err
	}

	if busy > 0 {
		return nil, common.ErrBusy
	}

	now := time.Now().UTC().Truncate(time.Microsecond)
	expires := now.Add(30 * time.Minute)
	status := "pending_confirmation"
	r := &models.Run{
		ID:               common.NewID(),
		RequestID:        req.RequestID,
		ConversationID:   source.ConversationID,
		Status:           status,
		Context:          source.Context,
		CreatedAt:        now,
		SkillID:          source.SkillID,
		SkillExecutionID: &root,
		Plan:             common.JSON(plan),
		PlanVersion:      1,
		PlanStatus:       &status,
		PlanExpiresAt:    &expires,
		AnswerVersion:    1,
	}
	if err := tx.Exec(
		"INSERT INTO ai_run (id,user_id,conversation_id,request_id,request_hash,status,"+
			"context,resource_scopes,skill_id,skill_execution_id,plan,plan_version,plan_status,plan_expires_at,"+
			"created_at) SELECT ?,?,?,?,?,'pending_confirmation',context,resource_scopes,skill_id,"+
			"skill_execution_id,?,1,'pending_confirmation',?,? FROM ai_run WHERE id=? AND user_id=?",
		r.ID,
		user,
		r.ConversationID,
		req.RequestID,
		common.RequestFingerprint(r.ConversationID, req),
		string(r.Plan),
		expires,
		now,
		source.ID,
		user,
	).
		Error; err != nil {
		return nil, err
	}

	if err := s.updateSkillUsage(ctx, tx, user, root, status, 0); err != nil {
		return nil, err
	}

	if err := tx.Exec("UPDATE ai_run SET active_plan_run_id=?,skill_status='pending_confirmation' WHERE id=?", r.ID, root).
		Error; err != nil {
		return nil, err
	}

	if err := tx.Exec("UPDATE ai_conversation SET last_active_at=? WHERE id=?", now, r.ConversationID).Error; err != nil {
		return nil, err
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	return r, nil
}

func (s *Store) MarkPreparationAnswer(ctx context.Context, user, id string) error {
	return s.DB.WithContext(ctx).Exec("UPDATE ai_run SET answer_available=TRUE WHERE id=? AND user_id=? AND status='pending_confirmation'", id, user).
		Error
}
