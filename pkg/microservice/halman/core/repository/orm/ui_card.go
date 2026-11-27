package orm

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
	"gorm.io/gorm"
)

func uiHash(v string) string { h := sha256.Sum256([]byte(v)); return hex.EncodeToString(h[:]) }
func (s *Store) UpsertUICard(ctx context.Context, user string, c models.UICard, source string) (*models.UICard, error) {
	if err := common.ValidateUICard(c); err != nil {
		return nil, err
	}
	var result *models.UICard
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var owner string
		if err := tx.Raw("SELECT user_id FROM ai_conversation WHERE id=? AND deleted_at IS NULL FOR UPDATE", c.ConversationID).Row().Scan(&owner); err != nil {
			return err
		}
		if owner != user {
			return common.ErrNotFound
		}
		var status string
		if err := tx.Raw("SELECT status FROM ai_run WHERE id=? AND user_id=? AND conversation_id=? FOR UPDATE", c.RunID, user, c.ConversationID).Row().Scan(&status); err != nil {
			return err
		}
		if status != "running" {
			return common.ErrRequestConflict
		}
		if c.Status == "preparing" {
			var pendingApproval bool
			err := tx.Raw("SELECT EXISTS(SELECT 1 FROM ai_run root JOIN ai_run plan ON plan.id=root.active_plan_run_id WHERE root.conversation_id=? AND root.user_id=? AND plan.plan_status='pending_confirmation')", c.ConversationID, user).Row().Scan(&pendingApproval)
			if err != nil {
				return err
			}
			if pendingApproval {
				return common.ErrRequestConflict
			}
		}
		var raw []byte
		err := tx.Raw("SELECT payload FROM ai_ui_card WHERE user_id=? AND run_id=? AND answer_version=? AND source_key=?", user, c.RunID, c.AnswerVersion, uiHash(source)).Row().Scan(&raw)
		if err == nil {
			var old models.UICard
			if err := json.Unmarshal(raw, &old); err != nil {
				return err
			}
			result = &old
			return nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		var count, pending int
		if err := tx.Raw("SELECT COUNT(*),COALESCE(SUM(status IN ('preparing','pending')),0) FROM ai_ui_card WHERE run_id=?", c.RunID).Row().Scan(&count, &pending); err != nil {
			return err
		}
		if count >= 20 || (c.Status == "preparing" && pending > 0) {
			return common.ErrRequestConflict
		}
		c.ID = common.NewID()
		c.Revision = 1
		c.Sequence = count + 1
		c.CreatedAt = time.Now().UTC()
		c.SourceKey = uiHash(source)
		err = tx.Exec("INSERT INTO ai_ui_card (id,user_id,conversation_id,run_id,answer_version,source_key,sequence,revision,status,draft_fields,payload,created_at,run_created_at,expires_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)", c.ID, user, c.ConversationID, c.RunID, c.AnswerVersion, c.SourceKey, c.Sequence, c.Revision, c.Status, c.DraftFields, string(common.JSON(c)), c.CreatedAt, c.RunCreatedAt, c.ExpiresAt).Error
		result = &c
		return err
	})
	return result, err
}
func finishUICards(ctx context.Context, tx *gorm.DB, r *models.Run, status string) error {
	next := "superseded"
	if status == "awaiting_input" {
		next = "pending"
	}
	return tx.WithContext(ctx).Exec("UPDATE ai_ui_card SET status=?,revision=revision+1 WHERE run_id=? AND status='preparing'", next, r.ID).Error
}

type uiCursor struct {
	At       time.Time `json:"at"`
	Run      string    `json:"run"`
	Sequence int       `json:"sequence"`
	ID       string    `json:"id"`
}

func (s *Store) readUICards(ctx context.Context, user, conversation, run, cursor string, limit int, cutoff time.Time) ([]models.UICard, string, error) {
	if limit < 1 || limit > 50 {
		return nil, "", common.ErrInvalidRequest
	}
	query := "SELECT c.payload,c.status,c.revision,c.draft_fields,r.status,sub.id,sub.action_id,sub.continuation_run_id,sub.current_attempt_run_id,COALESCE(a.status,''),CASE WHEN sub.id IS NULL OR JSON_UNQUOTE(JSON_EXTRACT(c.payload, '$.type'))<>'resource_selector' THEN NULL ELSE sub.values_json END,(SELECT n.id FROM ai_run n WHERE n.conversation_id=c.conversation_id ORDER BY n.created_at DESC,n.id DESC LIMIT 1) FROM ai_ui_card c JOIN ai_run r ON r.id=c.run_id JOIN ai_conversation v ON v.id=c.conversation_id LEFT JOIN ai_ui_submission sub ON sub.card_id=c.id LEFT JOIN ai_run a ON a.id=sub.current_attempt_run_id WHERE c.user_id=? AND v.user_id=? AND v.deleted_at IS NULL AND c.created_at>=? AND r.created_at>=?"
	args := []any{user, user, cutoff, cutoff}
	if conversation != "" {
		query += " AND c.conversation_id=?"
		args = append(args, conversation)
	}
	if run != "" {
		query += " AND c.run_id=?"
		args = append(args, run)
	}
	if cursor != "" {
		if len(cursor) > 512 {
			return nil, "", common.ErrInvalidCursor
		}
		var cur uiCursor
		raw, err := base64.RawURLEncoding.DecodeString(cursor)
		if err != nil || json.Unmarshal(raw, &cur) != nil || cur.At.IsZero() || cur.Run == "" || cur.ID == "" {
			return nil, "", common.ErrInvalidCursor
		}
		query += " AND (c.run_created_at,c.run_id,c.sequence,c.id)<(?,?,?,?)"
		args = append(args, cur.At, cur.Run, cur.Sequence, cur.ID)
	}
	query += " ORDER BY c.run_created_at DESC,c.run_id DESC,c.sequence DESC,c.id DESC LIMIT ?"
	args = append(args, limit+1)
	rows, err := s.DB.WithContext(ctx).Raw(query, args...).Rows()
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	cards := []models.UICard{}
	for rows.Next() {
		var raw, selected []byte
		var c models.UICard
		var status, runStatus, attemptStatus, latest string
		var revision int
		var draftFields bool
		var submissionID, action, firstRun, attemptRun sql.NullString
		if err := rows.Scan(&raw, &status, &revision, &draftFields, &runStatus, &submissionID, &action, &firstRun, &attemptRun, &attemptStatus, &selected, &latest); err != nil {
			return nil, "", err
		}
		if err := json.Unmarshal(raw, &c); err != nil {
			return nil, "", err
		}
		c.Status = status
		c.Revision = revision
		c.DraftFields = draftFields
		if submissionID.Valid {
			c.Submission = &models.UISubmissionResult{SubmissionID: submissionID.String, ActionID: action.String, ContinuationRunID: firstRun.String, CurrentAttemptRunID: attemptRun.String, CurrentAttemptStatus: attemptStatus}
			if len(selected) > 0 {
				if err := json.Unmarshal(selected, &c.ResourceSelections); err != nil {
					return nil, "", err
				}
			}
		}
		if c.ExpiresAt != nil && c.ExpiresAt.Before(time.Now()) && (status == "pending" || status == "preparing") {
			c.Status = "expired"
		} else if (status == "pending" && runStatus != "awaiting_input") || (status == "preparing" && runStatus != "running") || ((status == "pending" || status == "preparing") && latest != c.RunID) {
			c.Status = "superseded"
		}
		cards = append(cards, c)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}
	next := ""
	if len(cards) > limit {
		cards = cards[:limit]
		last := cards[len(cards)-1]
		next = base64.RawURLEncoding.EncodeToString(common.JSON(uiCursor{last.RunCreatedAt, last.RunID, last.Sequence, last.ID}))
	}
	return cards, next, nil
}
func (s *Store) ListUICards(ctx context.Context, user, conversation, cursor string, limit int, cutoff time.Time) ([]models.UICard, string, error) {
	if err := s.CheckConversation(ctx, user, conversation); err != nil {
		return nil, "", err
	}
	return s.readUICards(ctx, user, conversation, "", cursor, limit, cutoff)
}
func (s *Store) RunUICards(ctx context.Context, user, run string, cutoff time.Time) ([]models.UICard, error) {
	cards, _, err := s.readUICards(ctx, user, "", run, "", 50, cutoff)
	return cards, err
}
