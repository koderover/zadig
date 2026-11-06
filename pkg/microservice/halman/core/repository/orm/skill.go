package orm

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
)

func (s *Store) BindSkillExecution(ctx context.Context, user string, r *models.Run, rootID, skill string, cutoff time.Time) error {
	tx := s.DB.WithContext(ctx).Begin()
	err := tx.Error
	if err != nil {
		return err
	}

	defer tx.Rollback()
	var owner string
	if err := tx.WithContext(ctx).Raw("SELECT user_id FROM ai_conversation WHERE id=? AND deleted_at IS NULL FOR UPDATE", r.ConversationID).
		Row().
		Scan(&owner); err != nil || owner != user {
		return common.ErrNotFound
	}

	var rootSkill, rootConversation, rootUser, rootStatus string
	var parent, rootSkillStatus *string
	var created time.Time
	if err := tx.WithContext(ctx).Raw(
		"SELECT skill_id,conversation_id,user_id,created_at,status,skill_execution_id,skill_status"+
			" FROM ai_run WHERE id=? FOR UPDATE",
		rootID,
	).Row().Scan(&rootSkill, &rootConversation, &rootUser, &created, &rootStatus, &parent, &rootSkillStatus); err != nil {
		return common.ErrNotFound
	}

	if rootID != r.ID && rootSkillStatus != nil && (*rootSkillStatus == "cancelled" || *rootSkillStatus == "unknown") {
		return fmt.Errorf("该业务执行已经取消或结果待核实")
	}

	if rootID != r.ID && (parent == nil || *parent != rootID || rootStatus == "cancelled" || rootStatus == "unknown") {
		return fmt.Errorf("业务执行已经失效，请为新目标开始新的执行")
	}

	if rootID != r.ID && rootSkill == "" {
		return fmt.Errorf("目标运行不是可继续的业务执行")
	}

	if rootConversation != r.ConversationID || rootUser != user || created.Before(cutoff) || (rootSkill != "" && rootSkill != skill) {
		return fmt.Errorf("Skill 执行已失效或不属于本次目标")
	}

	if err := tx.WithContext(ctx).Exec("UPDATE ai_run SET skill_id=?,skill_execution_id=? WHERE id=?", skill, rootID, r.ID).
		Error; err != nil {
		return err
	}

	if rootSkill == "" {
		if err := tx.WithContext(ctx).Exec("UPDATE ai_run SET skill_status='running' WHERE id=?", rootID).Error; err != nil {
			return err
		}
	}

	if rootSkill != "" {
		if err := s.updateSkillUsage(ctx, tx, user, rootID, "running", 0); err != nil {
			return err
		}
	}

	if err := tx.Commit().Error; err != nil {
		return err
	}

	return nil
}

func (s *Store) appendScope(ctx context.Context, user, id string, target *models.PageContext, scope models.ResourceScope) error {
	tx := s.DB.WithContext(ctx).Begin()
	err := tx.Error
	if err != nil {
		return err
	}

	defer tx.Rollback()
	var raw []byte
	if err := tx.WithContext(ctx).Raw("SELECT resource_scopes FROM ai_run WHERE id=? AND user_id=? FOR UPDATE", id, user).
		Row().
		Scan(&raw); err != nil {
		return common.ErrNotFound
	}

	var scopes []models.ResourceScope
	if json.Unmarshal(raw, &scopes) != nil {
		return fmt.Errorf("运行权限范围无效")
	}

	found := false
	for _, entry := range scopes {
		if string(common.JSON(entry)) == string(common.JSON(scope)) {
			found = true
		}
	}

	if !found {
		if len(scopes) >= 32 {
			return common.ErrCapacity
		}

		scopes = append(scopes, scope)
	}

	if target != nil {
		err = tx.WithContext(ctx).Exec(
			"UPDATE ai_run SET context=?,resource_scopes=? WHERE id=? AND user_id=?",
			string(common.JSON(target)),
			string(common.JSON(scopes)),
			id,
			user,
		).
			Error
	} else {
		err = tx.WithContext(ctx).Exec("UPDATE ai_run SET resource_scopes=? WHERE id=? AND user_id=?", string(common.JSON(scopes)), id, user).
			Error
	}

	if err != nil {
		return err
	}

	return tx.Commit().Error
}
func (s *Store) RecordScope(ctx context.Context, user, id string, target models.PageContext) error {
	return s.appendScope(ctx, user, id, &target, models.ResourceScope{PageContext: target})
}
func (s *Store) RecordCatalogScope(ctx context.Context, user, id, kind string) error {
	return s.appendScope(ctx, user, id, nil, models.ResourceScope{Catalog: kind})
}

func (s *Store) RecordResourceScope(ctx context.Context, user, id string, scope models.ResourceScope) error {
	return s.appendScope(ctx, user, id, nil, scope)
}

func (s *Store) RecordSystemScope(ctx context.Context, user, id, action string) error {
	page := models.PageContext{PageType: "efficiency"}
	return s.appendScope(ctx, user, id, &page, models.ResourceScope{SystemAction: action})
}
