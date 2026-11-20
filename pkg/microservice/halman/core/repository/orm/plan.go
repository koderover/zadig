package orm

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
)

// 领取动作与取消串行；事务提交后才发 HTTP，崩溃后绝不自动重放 submitting。
func (s *Store) ClaimOperation(ctx context.Context, user string, r *models.Run, rootID string, item models.OperationItem) error {
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockActiveConversation(tx, user, r.ConversationID); err != nil {
			return common.ErrNotFound
		}

		var active string
		if err := tx.Raw("SELECT active_plan_run_id FROM ai_run WHERE id=? FOR UPDATE", rootID).Row().
			Scan(&active); err != nil {
			return err
		}

		if active != r.ID {
			return common.ErrRequestConflict
		}

		var raw []byte
		var status string
		if err := tx.Raw("SELECT plan,status FROM ai_run WHERE id=? FOR UPDATE", r.ID).Row().Scan(&raw, &status); err != nil {
			return err
		}

		if status != "executing" {
			return context.Canceled
		}

		var plan models.OperationPlan
		if err := json.Unmarshal(raw, &plan); err != nil {
			return err
		}

		for i := range plan.Items {
			current := &plan.Items[i]
			if current.ID == "" || current.ID != item.ID {
				continue
			}
			if current.Status != "pending" {
				return common.ErrRequestConflict
			}
			current.Status, current.Params = "submitting", item.Params
			return tx.Exec("UPDATE ai_run SET plan=? WHERE id=?", string(common.JSON(plan)), r.ID).Error
		}
		return common.ErrRequestConflict
	})
}

func (s *Store) SaveOperation(ctx context.Context, user, id string, index int, item models.OperationItem) error {
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var raw []byte
		if err := tx.Raw("SELECT plan FROM ai_run WHERE id=? AND user_id=? FOR UPDATE", id, user).Row().
			Scan(&raw); err != nil {
			return err
		}

		var plan models.OperationPlan
		if err := json.Unmarshal(raw, &plan); err != nil {
			return err
		}

		if index < 0 || index >= len(plan.Items) || plan.Items[index].ID != item.ID {
			return common.ErrRequestConflict
		}

		plan.Items[index] = item
		if err := tx.Exec("UPDATE ai_run SET plan=? WHERE id=?", string(common.JSON(plan)), id).Error; err != nil {
			return err
		}

		return nil
	})
}

func (s *Store) FinishPlan(ctx context.Context, user string, r *models.Run, rootID, status string, elapsed int64) error {
	tx := s.DB.WithContext(ctx).Begin()
	err := tx.Error
	if err != nil {
		return err
	}

	defer tx.Rollback()
	var found string
	if err := tx.WithContext(ctx).Raw("SELECT id FROM ai_conversation WHERE id=? AND user_id=? FOR UPDATE", r.ConversationID, user).
		Row().
		Scan(&found); err != nil {
		return err
	}

	if err := tx.WithContext(ctx).Raw("SELECT id FROM ai_run WHERE id=? FOR UPDATE", rootID).Row().Scan(&found); err != nil {
		return err
	}

	var current string
	if err := tx.WithContext(ctx).Raw("SELECT status FROM ai_run WHERE id=? FOR UPDATE", r.ID).Row().Scan(&current); err != nil {
		return err
	}

	if current != "executing" && current != "cancelling" {
		return nil
	}

	if current == "cancelling" && status != "unknown" && status != "succeeded" {
		status = "cancelled"
	}

	if err := s.updateSkillUsage(ctx, tx, user, rootID, status, elapsed); err != nil {
		return err
	}

	if err := tx.WithContext(ctx).Exec(
		"UPDATE ai_run SET status=?,plan_status=?,finished_at=?,elapsed_ms=elapsed_ms+? WHERE id=?",
		status,
		status,
		time.Now().
			UTC(),
		elapsed,
		r.ID,
	).
		Error; err != nil {
		return err
	}

	return tx.Commit().Error
}

// ConfirmPlan uses the snapshot that passed the caller's preflight checks.
func (s *Store) ConfirmPlan(ctx context.Context, user string, run *models.Run, cutoff time.Time) (*models.Run, bool, error) {
	return s.confirmPlan(ctx, user, run, cutoff, false)
}

func (s *Store) ExpirePlan(ctx context.Context, user string, run *models.Run, cutoff time.Time) error {
	_, _, err := s.confirmPlan(ctx, user, run, cutoff, true)
	return err
}

func (s *Store) confirmPlan(ctx context.Context, user string, run *models.Run, cutoff time.Time, expireOnly bool) (*models.Run, bool, error) {
	if run.SkillExecutionID == nil {
		return nil, false, common.ErrPlanUnavailable
	}
	tx := s.DB.WithContext(ctx).Begin()
	if tx.Error != nil {
		return nil, false, tx.Error
	}

	defer tx.Rollback()
	if err := lockActiveConversation(tx, user, run.ConversationID); err != nil {
		return nil, false, common.ErrNotFound
	}

	var active, skillStatus sql.NullString
	if err := tx.Raw("SELECT active_plan_run_id,skill_status FROM ai_run WHERE id=? AND user_id=? AND created_at>=? FOR UPDATE", *run.SkillExecutionID, user, cutoff).
		Row().
		Scan(&active, &skillStatus); err != nil {
		return nil, false, common.ErrNotFound
	}

	current, err := scanRun(tx.Raw("SELECT "+runColumns+" FROM ai_run WHERE id=? AND user_id=? FOR UPDATE", run.ID, user).
		Row())
	if err != nil {
		return nil, false, err
	}

	if current.PlanVersion != run.PlanVersion || !active.Valid || active.String != run.ID {
		return nil, false, common.ErrPlanChanged
	}

	if !expireOnly && current.PlanStatus != nil &&
		(*current.PlanStatus == "executing" || *current.PlanStatus == "succeeded" || *current.PlanStatus == "failed" ||
			*current.PlanStatus == "unknown") {
		return current, false, nil
	}

	if current.Status == "cancelled" && current.Error == common.ErrPlanExpired.Error() {
		return nil, false, common.ErrPlanExpired
	}
	if current.Status != "pending_confirmation" || current.PlanStatus == nil || *current.PlanStatus != "pending_confirmation" ||
		current.PlanExpiresAt == nil {
		return nil, false, common.ErrPlanUnavailable
	}
	now := time.Now().UTC()
	if !current.PlanExpiresAt.After(now) {
		if skillStatus.String != "pending_confirmation" {
			return nil, false, common.ErrPlanUnavailable
		}
		if err := s.updateSkillUsage(ctx, tx, user, *run.SkillExecutionID, "cancelled", 0); err != nil {
			return nil, false, err
		}
		if err := tx.Exec("UPDATE ai_run SET status='cancelled',plan_status='cancelled',finished_at=?,error=? WHERE id=?",
			now, common.ErrPlanExpired.Error(), run.ID).Error; err != nil {
			return nil, false, err
		}
		if err := tx.Commit().Error; err != nil {
			return nil, false, err
		}
		return nil, false, common.ErrPlanExpired
	}
	if expireOnly {
		return nil, false, common.ErrPlanUnavailable
	}
	if !bytes.Equal(current.Plan, run.Plan) {
		return nil, false, common.ErrPlanChanged
	}

	var busy int
	if err := tx.Raw("SELECT COUNT(*) FROM ai_run WHERE user_id=? AND status IN ('running','cancelling','executing')", user).
		Row().
		Scan(&busy); err != nil {
		return nil, false, err
	}

	if busy > 0 {
		return nil, false, common.ErrBusy
	}

	if err := tx.Exec("UPDATE ai_run SET status='executing',plan_status='executing',finished_at=NULL WHERE id=?", run.ID).
		Error; err != nil {
		return nil, false, err
	}

	if err := tx.Commit().Error; err != nil {
		return nil, false, err
	}

	current.Status = "executing"
	return current, true, nil
}

func (s *Store) ExpirePendingPlans(ctx context.Context) error {
	var plans []struct {
		ID, UserID, ConversationID, SkillExecutionID string
		PlanVersion                                  int
	}
	if err := s.DB.WithContext(ctx).Raw(
		"SELECT r.id,r.user_id,r.plan_version,r.conversation_id,r.skill_execution_id FROM ai_run r"+
			" JOIN ai_run root ON root.id=r.skill_execution_id AND root.active_plan_run_id=r.id"+
			" JOIN ai_conversation c ON c.id=r.conversation_id AND c.user_id=r.user_id AND c.deleted_at IS NULL"+
			" WHERE r.status='pending_confirmation' AND r.plan_status='pending_confirmation'"+
			" AND root.skill_status='pending_confirmation' AND r.plan_expires_at<=?"+
			" ORDER BY r.created_at,r.id LIMIT 100",
		time.Now().UTC(),
	).Scan(&plans).Error; err != nil {
		return err
	}

	var sweepErr error
	for _, plan := range plans {
		if ctx.Err() != nil {
			return errors.Join(sweepErr, ctx.Err())
		}
		run := &models.Run{ID: plan.ID, ConversationID: plan.ConversationID, SkillExecutionID: &plan.SkillExecutionID, PlanVersion: plan.PlanVersion}
		err := s.ExpirePlan(ctx, plan.UserID, run, time.Unix(0, 0).UTC())
		if err != nil && !errors.Is(err, common.ErrPlanExpired) && !errors.Is(err, common.ErrNotFound) &&
			!errors.Is(err, common.ErrPlanChanged) && !errors.Is(err, common.ErrPlanUnavailable) {
			sweepErr = errors.Join(sweepErr, err)
		}
	}
	return sweepErr
}

func (s *Store) SavePlan(ctx context.Context, user string, r *models.Run, rootID string, cutoff time.Time, raw []byte) (int, time.Time, error) {
	tx := s.DB.WithContext(ctx).Begin()
	err := tx.Error
	if err != nil {
		return 0, time.Time{}, err
	}

	defer tx.Rollback()
	if err := lockActiveConversation(tx, user, r.ConversationID); err != nil {
		return 0, time.Time{}, common.ErrNotFound
	}

	var active, rootStatus sql.NullString
	if err := tx.WithContext(ctx).Raw("SELECT active_plan_run_id,skill_status FROM ai_run WHERE id=? AND created_at>=? FOR UPDATE", rootID, cutoff).
		Row().
		Scan(&active, &rootStatus); err != nil {
		return 0, time.Time{}, common.ErrNotFound
	}

	if rootStatus.String == "succeeded" || rootStatus.String == "cancelled" || rootStatus.String == "unknown" {
		return 0, time.Time{}, fmt.Errorf("该业务执行已结束或结果待核实，请为新目标发起新的执行")
	}

	var version int
	var status string
	if err := tx.WithContext(ctx).Raw("SELECT plan_version,status FROM ai_run WHERE id=? FOR UPDATE", r.ID).Row().
		Scan(&version, &status); err != nil {
		return 0, time.Time{}, err
	}

	if status != "running" {
		return 0, time.Time{}, fmt.Errorf("运行已停止，不能生成新的确认清单")
	}

	if active.Valid && active.String != r.ID {
		if err := tx.WithContext(ctx).Exec("UPDATE ai_run SET plan_status='superseded' WHERE id=? AND plan_status='pending_confirmation'", active.String).
			Error; err != nil {
			return 0, time.Time{}, err
		}
	}

	expires := time.Now().UTC().Add(30 * time.Minute)
	version++
	if err := tx.WithContext(ctx).Exec(
		"UPDATE ai_run SET plan=?,plan_version=?,plan_status='pending_confirmation',plan_expires_at=? WHERE id=?",
		string(raw),
		version,
		expires,
		r.ID,
	).
		Error; err != nil {
		return 0, time.Time{}, err
	}

	if err := tx.WithContext(ctx).Exec("UPDATE ai_run SET active_plan_run_id=?,skill_status='pending_confirmation' WHERE id=?", r.ID, rootID).
		Error; err != nil {
		return 0, time.Time{}, err
	}

	if err := tx.Commit().Error; err != nil {
		return 0, time.Time{}, err
	}

	return version, expires, nil
}

func (s *Store) SaveDraft(ctx context.Context, user string, r *models.Run, rootID string, cutoff time.Time, plan models.OperationPlan) error {
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockActiveConversation(tx, user, r.ConversationID); err != nil {
			return common.ErrNotFound
		}
		var active, rootStatus sql.NullString
		if err := tx.Raw("SELECT active_plan_run_id,skill_status FROM ai_run WHERE id=? AND user_id=? AND created_at>=? FOR UPDATE", rootID, user, cutoff).Row().Scan(&active, &rootStatus); err != nil {
			return common.ErrNotFound
		}
		if rootStatus.String == "succeeded" || rootStatus.String == "cancelled" || rootStatus.String == "unknown" {
			return common.ErrRequestConflict
		}
		var status string
		if err := tx.Raw("SELECT status FROM ai_run WHERE id=? AND user_id=? AND conversation_id=? FOR UPDATE", r.ID, user, r.ConversationID).Row().Scan(&status); err != nil {
			return common.ErrNotFound
		}
		if status != "running" {
			return common.ErrRequestConflict
		}
		if active.Valid && active.String != r.ID {
			if err := tx.Exec("UPDATE ai_run SET plan_status='superseded' WHERE id=? AND plan_status='pending_confirmation'", active.String).Error; err != nil {
				return err
			}
		}
		// 保存新参数快照后，旧卡片不能再确认执行。
		updateRoot := "UPDATE ai_run SET active_plan_run_id=NULL"
		if rootID != r.ID && plan.Draft != nil && len(plan.MissingFields) > 0 {
			updateRoot += ",skill_status='awaiting_input'"
		}
		if err := tx.Exec(updateRoot+" WHERE id=? AND user_id=?", rootID, user).Error; err != nil {
			return err
		}
		var planStatus any
		if len(plan.MissingFields) > 0 {
			planStatus = "awaiting_input"
		}
		if err := tx.Exec("UPDATE ai_run SET plan=?,plan_status=? WHERE id=? AND user_id=?", string(common.JSON(plan)), planStatus, r.ID, user).Error; err != nil {
			return err
		}
		return nil
	})
}
