package orm

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
)

// 领取动作与取消串行；事务提交后才发 HTTP，崩溃后绝不自动重放 submitting。
func (s *Store) ClaimOperation(ctx context.Context, user string, r *models.Run, rootID, id string) error {
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

	var active string
	if err := tx.WithContext(ctx).Raw("SELECT active_plan_run_id FROM ai_run WHERE id=? FOR UPDATE", rootID).Row().
		Scan(&active); err != nil {
		return err
	}

	if active != r.ID {
		return common.ErrRequestConflict
	}

	var raw []byte
	var status string
	if err := tx.WithContext(ctx).Raw("SELECT plan,status FROM ai_run WHERE id=? FOR UPDATE", r.ID).Row().Scan(&raw, &status); err != nil {
		return err
	}

	if status != "executing" {
		return context.Canceled
	}

	var plan models.OperationPlan
	if err := json.Unmarshal(raw, &plan); err != nil {
		return err
	}

	found = ""
	for i := range plan.Items {
		if plan.Items[i].ID == id {
			if plan.Items[i].Status != "pending" {
				return common.ErrRequestConflict
			}

			plan.Items[i].Status = "submitting"
			found = id
			break
		}
	}

	if found == "" {
		return common.ErrRequestConflict
	}

	if err := tx.WithContext(ctx).Exec("UPDATE ai_run SET plan=? WHERE id=?", string(common.JSON(plan)), r.ID).Error; err != nil {
		return err
	}

	return tx.Commit().Error
}

func (s *Store) SaveOperation(ctx context.Context, user, id string, index int, item models.OperationItem) error {
	tx := s.DB.WithContext(ctx).Begin()
	err := tx.Error
	if err != nil {
		return err
	}

	defer tx.Rollback()
	var raw []byte
	if err := tx.WithContext(ctx).Raw("SELECT plan FROM ai_run WHERE id=? AND user_id=? FOR UPDATE", id, user).Row().
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
	if err := tx.WithContext(ctx).Exec("UPDATE ai_run SET plan=? WHERE id=?", string(common.JSON(plan)), id).Error; err != nil {
		return err
	}

	return tx.Commit().Error
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

func (s *Store) ConfirmPlan(ctx context.Context, user, id string, version int, cutoff time.Time) (*models.Run, models.OperationPlan, bool, error) {
	r, err := s.GetRun(ctx, user, id, cutoff)
	if err != nil {
		return nil, models.OperationPlan{}, false, err
	}

	var plan models.OperationPlan
	if json.Unmarshal(r.Plan, &plan) != nil {
		return nil, plan, false, fmt.Errorf("确认清单无效")
	}

	tx := s.DB.WithContext(ctx).Begin()
	err = tx.Error
	if err != nil {
		return nil, plan, false, err
	}

	defer tx.Rollback()
	var found string
	if err := tx.WithContext(ctx).Raw("SELECT id FROM ai_conversation WHERE id=? AND user_id=? AND deleted_at IS NULL FOR UPDATE", r.ConversationID, user).
		Row().
		Scan(&found); err != nil {
		return nil, plan, false, common.ErrNotFound
	}

	if r.SkillExecutionID == nil {
		return nil, plan, false, fmt.Errorf("%w: 清单缺少业务执行关联", common.ErrRequestConflict)
	}

	var active sql.NullString
	if err := tx.WithContext(ctx).Raw("SELECT active_plan_run_id FROM ai_run WHERE id=? AND created_at>=? FOR UPDATE", *r.SkillExecutionID, cutoff).
		Row().
		Scan(&active); err != nil {
		return nil, plan, false, common.ErrNotFound
	}

	current, err := scanRun(tx.WithContext(ctx).Raw("SELECT "+runColumns+" FROM ai_run WHERE id=? FOR UPDATE", id).
		Row())
	if err != nil {
		return nil, plan, false, err
	}

	if current.PlanVersion != version || !active.Valid || active.String != id {
		return nil, plan, false, fmt.Errorf("%w: 清单已变化，请重新预览并确认", common.ErrRequestConflict)
	}

	if current.PlanStatus != nil &&
		(*current.PlanStatus == "executing" || *current.PlanStatus == "succeeded" || *current.PlanStatus == "failed" ||
			*current.PlanStatus == "unknown") {
		return current, plan, false, nil
	}

	if current.Status != "pending_confirmation" || current.PlanStatus == nil || *current.PlanStatus != "pending_confirmation" ||
		current.PlanExpiresAt == nil ||
		!current.PlanExpiresAt.After(time.Now()) {
		return nil, plan, false, fmt.Errorf("%w: 清单已取消、过期或尚未准备完成", common.ErrRequestConflict)
	}

	var busy int
	if err := tx.WithContext(ctx).Raw("SELECT COUNT(*) FROM ai_run WHERE user_id=? AND status IN ('running','cancelling','executing')", user).
		Row().
		Scan(&busy); err != nil {
		return nil, plan, false, err
	}

	if busy > 0 {
		return nil, plan, false, common.ErrBusy
	}

	if string(current.Plan) != string(r.Plan) {
		return nil, plan, false, fmt.Errorf("%w: 清单内容已变化", common.ErrRequestConflict)
	}

	if err := tx.WithContext(ctx).Exec("UPDATE ai_run SET status='executing',plan_status='executing',finished_at=NULL WHERE id=?", id).
		Error; err != nil {
		return nil, plan, false, err
	}

	if err := tx.Commit().Error; err != nil {
		return nil, plan, false, err
	}

	current.Status = "executing"
	return current, plan, true, nil
}

func (s *Store) SavePlan(ctx context.Context, user string, r *models.Run, rootID string, cutoff time.Time, raw []byte) (int, time.Time, error) {
	tx := s.DB.WithContext(ctx).Begin()
	err := tx.Error
	if err != nil {
		return 0, time.Time{}, err
	}

	defer tx.Rollback()
	var found string
	if err := tx.WithContext(ctx).Raw("SELECT id FROM ai_conversation WHERE id=? AND user_id=? AND deleted_at IS NULL FOR UPDATE", r.ConversationID, user).
		Row().
		Scan(&found); err != nil {
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
	tx := s.DB.WithContext(ctx).Begin()
	if tx.Error != nil {
		return tx.Error
	}
	defer tx.Rollback()
	var found string
	if err := tx.Raw("SELECT id FROM ai_conversation WHERE id=? AND user_id=? AND deleted_at IS NULL FOR UPDATE", r.ConversationID, user).Row().Scan(&found); err != nil {
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
	if err := tx.Exec("UPDATE ai_run SET active_plan_run_id=NULL WHERE id=? AND user_id=?", rootID, user).Error; err != nil {
		return err
	}
	var planStatus any
	if len(plan.MissingFields) > 0 {
		planStatus = "awaiting_input"
	}
	if err := tx.Exec("UPDATE ai_run SET plan=?,plan_status=? WHERE id=? AND user_id=?", string(common.JSON(plan)), planStatus, r.ID, user).Error; err != nil {
		return err
	}
	return tx.Commit().Error
}
