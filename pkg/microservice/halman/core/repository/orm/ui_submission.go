package orm

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
	"gorm.io/gorm"
)

func saveUIRun(tx *gorm.DB, user string, req models.RunRequest, run *models.Run) error {
	u := req.UIContinuation
	if u == nil && !req.ChatMessage {
		return nil
	}
	// BeginRun already holds the conversation lock. Both card and text replies
	// must use the same latest-run check before claiming an interaction.
	var latest, status string
	var planStatus sql.NullString
	err := tx.Raw("SELECT id,status,plan_status FROM ai_run WHERE conversation_id=? AND user_id=? AND id<>? ORDER BY created_at DESC,id DESC LIMIT 1 FOR UPDATE", run.ConversationID, user, run.ID).Row().Scan(&latest, &status, &planStatus)
	if errors.Is(err, sql.ErrNoRows) && u == nil && req.CardReply == nil {
		return nil
	}
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return common.ErrRequestConflict
		}
		return err
	}

	if u == nil {
		if status != "awaiting_input" {
			if req.CardReply != nil {
				return common.ErrRequestConflict
			}
			return nil
		}
		query := "UPDATE ai_ui_card SET status='superseded',revision=revision+1 WHERE run_id=? AND user_id=? AND status='pending' AND expires_at>?"
		args := []any{latest, user, run.CreatedAt}
		if reply := req.CardReply; reply != nil {
			query += " AND id=? AND revision=?"
			args = append(args, reply.CardID, reply.Revision)
		}
		result := tx.Exec(query, args...)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected > 0 {
			run.RepliedCardRunID = latest
		} else if req.CardReply != nil {
			return common.ErrRequestConflict
		}
		return nil
	}

	source := u.SourceRun
	if source == nil || source.ConversationID != run.ConversationID {
		return common.ErrRequestConflict
	}
	var raw []byte
	var cardStatus string
	var revision int
	var expiry sql.NullTime
	err = tx.Raw("SELECT payload,status,revision,expires_at FROM ai_ui_card WHERE id=? AND user_id=? AND run_id=? FOR UPDATE",
		u.CardID, user, source.ID).Row().Scan(&raw, &cardStatus, &revision, &expiry)
	if errors.Is(err, sql.ErrNoRows) {
		return common.ErrNotFound
	}
	if err != nil {
		return err
	}

	next := "submitted"
	if u.Retry {
		var current string
		err := tx.Raw("SELECT current_attempt_run_id FROM ai_ui_submission WHERE id=? AND card_id=? FOR UPDATE", u.SubmissionID, u.CardID).Row().Scan(&current)
		if errors.Is(err, sql.ErrNoRows) {
			return common.ErrRequestConflict
		}
		if err != nil {
			return err
		}
		if cardStatus != "submitted" || current != latest || status != "interrupted" || planStatus.String == "pending_confirmation" || planStatus.String == "unknown" {
			return common.ErrRequestConflict
		}
		if err := tx.Exec("UPDATE ai_ui_submission SET current_attempt_run_id=? WHERE id=?", run.ID, u.SubmissionID).Error; err != nil {
			return err
		}
	} else {
		if latest != source.ID || status != "awaiting_input" || cardStatus != "pending" || revision != u.Revision || !expiry.Valid || !expiry.Time.After(run.CreatedAt) {
			return common.ErrRequestConflict
		}
		var card models.UICard
		if err := json.Unmarshal(raw, &card); err != nil {
			return err
		}
		if err := common.ValidateUIValues(card, u.ActionID, u.Values); err != nil {
			return err
		}
		if err := tx.Exec("INSERT INTO ai_ui_submission (id,card_id,revision,action_id,values_json,continuation_run_id,current_attempt_run_id,created_at) VALUES (?,?,?,?,?,?,?,?)",
			common.NewID(), u.CardID, revision, u.ActionID, string(common.JSON(u.Values)), run.ID, run.ID, run.CreatedAt).Error; err != nil {
			return err
		}
	}
	if err := tx.Exec("UPDATE ai_ui_card SET status=?,revision=revision+1 WHERE id=?", next, u.CardID).Error; err != nil {
		return err
	}
	var rawScopes []byte
	if err := tx.Raw("SELECT resource_scopes FROM ai_run WHERE id=? AND user_id=?", source.ID, user).Row().Scan(&rawScopes); err != nil {
		return err
	}
	var scopes []models.ResourceScope
	if err := json.Unmarshal(rawScopes, &scopes); err != nil {
		return err
	}
	scopes = append(scopes, u.ResourceScopes...)
	if scopes == nil {
		scopes = []models.ResourceScope{}
	}
	if err := tx.Exec("UPDATE ai_run SET skill_id=?,skill_execution_id=?,resource_scopes=? WHERE id=?", source.SkillID, source.SkillExecutionID, string(common.JSON(scopes)), run.ID).Error; err != nil {
		return err
	}
	run.SkillID = source.SkillID
	run.SkillExecutionID = source.SkillExecutionID

	return nil
}

func (s *Store) UISubmissionValues(ctx context.Context, user, id string) (map[string]any, string, error) {
	var raw []byte
	var action string
	err := s.DB.WithContext(ctx).Raw("SELECT s.values_json,s.action_id FROM ai_ui_submission s JOIN ai_ui_card c ON c.id=s.card_id WHERE s.id=? AND c.user_id=?", id, user).Row().Scan(&raw, &action)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, "", common.ErrNotFound
	}
	if err != nil {
		return nil, "", err
	}
	var values map[string]any
	err = json.Unmarshal(raw, &values)
	return values, action, err
}
