package orm

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
	"trpc.group/trpc-go/trpc-agent-go/event"
	"trpc.group/trpc-go/trpc-agent-go/model"

	halmanconfig "github.com/koderover/zadig/v2/pkg/microservice/halman/config"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
)

const runColumns = "id, request_id, conversation_id, status, context, created_at, finished_at, error, input_tokens, " +
	"output_tokens, skill_id, skill_execution_id, skill_status, plan, plan_version, plan_status, " +
	"plan_expires_at, answer_version, question_source_run_id, answer_available"

func (s *Store) FindRequest(ctx context.Context, user, conversation string, req models.RunRequest, cutoff time.Time) (*models.Run, error) {
	var id, fingerprint string
	var created time.Time
	err := s.DB.WithContext(ctx).Raw(
		"SELECT r.id,r.request_hash,r.created_at"+
			" FROM ai_run r"+
			" JOIN ai_conversation c ON c.id=r.conversation_id"+
			" WHERE r.user_id=? AND r.request_id=? AND c.user_id=? AND c.deleted_at IS NULL",
		user,
		req.RequestID,
		user,
	).Row().
		Scan(&id, &fingerprint, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, common.ErrNotFound
	}

	if err != nil {
		return nil, err
	}

	if fingerprint != common.RequestFingerprint(conversation, req) || created.Before(cutoff) {
		return nil, common.ErrRequestConflict
	}

	return s.GetRun(ctx, user, id, cutoff)
}

func scanRun(row interface{ Scan(...any) error }) (*models.Run, error) {
	var r models.Run
	var raw, plan []byte
	if err := row.Scan(
		&r.ID,
		&r.RequestID,
		&r.ConversationID,
		&r.Status,
		&raw,
		&r.CreatedAt,
		&r.FinishedAt,
		&r.Error,
		&r.InputTokens,
		&r.OutputTokens,
		&r.SkillID,
		&r.SkillExecutionID,
		&r.SkillStatus,
		&plan,
		&r.PlanVersion,
		&r.PlanStatus,
		&r.PlanExpiresAt,
		&r.AnswerVersion,
		&r.QuestionSourceRunID,
		&r.AnswerAvailable,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, common.ErrNotFound
		}

		return nil, err
	}

	r.Plan = plan
	if err := json.Unmarshal(raw, &r.Context); err != nil {
		return nil, err
	}

	return &r, nil
}

func (s *Store) CreateConversation(ctx context.Context, user string) (*models.Conversation, error) {
	now := time.Now().UTC().Truncate(time.Microsecond)
	c := &models.Conversation{
		ID:           common.NewID(),
		Title:        "新会话",
		CreatedAt:    now,
		LastActiveAt: now,
	}
	err := s.DB.WithContext(ctx).Table("ai_conversation").Create(map[string]any{
		"id":             c.ID,
		"user_id":        user,
		"title":          c.Title,
		"created_at":     now,
		"last_active_at": now,
	}).Error
	return c, err
}

func (s *Store) CheckConversation(ctx context.Context, user, id string) error {
	var found models.Conversation
	err := s.DB.WithContext(ctx).Table("ai_conversation").Select("id").
		Where("id=? AND user_id=? AND deleted_at IS NULL", id, user).Take(&found).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return common.ErrNotFound
	}

	return err
}

func (s *Store) ListConversations(ctx context.Context, user, after string, limit int, cutoff time.Time) ([]models.Conversation, string, error) {
	c, err := common.DecodeCursor(after)
	if err != nil {
		return nil, "", err
	}
	// 到期标题立即隐藏，不等待后台重建。
	query := "SELECT id,CASE WHEN COALESCE(title_source_at,created_at) >= ? THEN title ELSE '历史会话' END,created_at," +
		"last_active_at FROM ai_conversation WHERE user_id=? AND deleted_at IS NULL AND last_active_at>=?"
	args := []any{cutoff, user, cutoff}
	if after != "" {
		query += " AND (last_active_at < ? OR (last_active_at = ? AND id < ?))"
		args = append(args, c.At, c.At, c.ID)
	}

	query += " ORDER BY last_active_at DESC,id DESC LIMIT ?"
	args = append(args, limit+1)
	rows, err := s.DB.WithContext(ctx).Raw(query, args...).Rows()
	if err != nil {
		return nil, "", err
	}

	defer rows.Close()
	items := make([]models.Conversation, 0, limit+1)
	for rows.Next() {
		var c models.Conversation
		if err := rows.Scan(&c.ID, &c.Title, &c.CreatedAt, &c.LastActiveAt); err != nil {
			return nil, "", err
		}

		items = append(items, c)
	}

	if err := rows.Err(); err != nil {
		return nil, "", err
	}

	next := ""
	if len(items) > limit {
		items = items[:limit]
		last := items[len(items)-1]
		next = common.EncodeCursor(last.LastActiveAt, last.ID)
	}

	return items, next, nil
}

func (s *Store) BeginRun(ctx context.Context, user, conversation string, req models.RunRequest, modelID, modelName string) (*models.Run, bool, error) {
	tx := s.DB.WithContext(ctx).Begin()
	err := tx.Error
	if err != nil {
		return nil, false, err
	}

	defer tx.Rollback()
	var owner, title string
	var titleSource sql.NullTime
	if err = tx.WithContext(ctx).Raw("SELECT user_id,title,title_source_at FROM ai_conversation WHERE id=? AND deleted_at IS NULL FOR UPDATE", conversation).
		Row().
		Scan(&owner, &title, &titleSource); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, false, common.ErrNotFound
		}

		return nil, false, err
	}

	if owner != user {
		return nil, false, common.ErrNotFound
	}

	fingerprint := common.RequestFingerprint(conversation, req)
	var previousID, previousHash string
	err = tx.WithContext(ctx).Raw("SELECT id,request_hash FROM ai_run WHERE user_id=? AND request_id=?", user, req.RequestID).
		Row().
		Scan(&previousID, &previousHash)
	if err == nil {
		if previousHash != fingerprint {
			return nil, false, common.ErrRequestConflict
		}

		r, err := scanRun(tx.WithContext(ctx).Raw("SELECT "+runColumns+" FROM ai_run WHERE id=?", previousID).Row())
		return r, false, err
	}

	if !errors.Is(err, sql.ErrNoRows) {
		return nil, false, err
	}

	if source := req.RegenerateFrom; source != nil {
		origin := source.ID
		if source.QuestionSourceRunID != nil {
			origin = *source.QuestionSourceRunID
		}

		var latest, latestOrigin, status string
		var available bool
		if err := tx.WithContext(ctx).Raw(
			"SELECT id,COALESCE(question_source_run_id,id),status,answer_available"+
				" FROM ai_run WHERE conversation_id=? ORDER BY created_at DESC,id DESC LIMIT 1 FOR UPDATE",
			conversation,
		).Row().
			Scan(&latest, &latestOrigin, &status, &available); err != nil {
			return nil, false, err
		}

		if latest != source.ID &&
			(latestOrigin != origin || available || status == "succeeded" || status == "pending_confirmation" || status == "awaiting_input") {
			return nil, false, common.ErrRequestConflict
		}
	}

	var active int
	if err = tx.WithContext(ctx).Raw("SELECT COUNT(*) FROM ai_run WHERE user_id=? AND status IN ('running','cancelling','executing')", user).
		Row().
		Scan(&active); err != nil {
		return nil, false, err
	}

	if active > 0 {
		return nil, false, common.ErrBusy
	}

	now := time.Now().UTC().Truncate(time.Microsecond)
	r := &models.Run{
		ID:             common.NewID(),
		RequestID:      req.RequestID,
		ConversationID: conversation,
		Status:         "running",
		Context:        req.Context,
		CreatedAt:      now,
	}
	page, _ := json.Marshal(req.Context)
	err = tx.WithContext(ctx).Exec(
		"INSERT INTO ai_run (id,user_id,conversation_id,request_id,request_hash,status,"+
			"context,resource_scopes,model_id,model_name,created_at) VALUES (?,?,?,?,?,'running',?,'[]',?,?,?)",
		r.ID,
		user,
		conversation,
		req.RequestID,
		fingerprint,
		string(page),
		modelID,
		modelName,
		now,
	).
		Error
	if err != nil {
		return nil, false, err
	}

	if source := req.RegenerateFrom; source != nil {
		origin := source.ID
		if source.QuestionSourceRunID != nil {
			origin = *source.QuestionSourceRunID
		}

		version := source.AnswerVersion + 1
		var scopes []byte
		if err := tx.WithContext(ctx).Raw("SELECT resource_scopes FROM ai_run WHERE id=?", source.ID).Row().Scan(&scopes); err != nil {
			return nil, false, err
		}

		if err := tx.WithContext(ctx).Exec(
			"UPDATE ai_run"+
				" SET answer_version=?,question_source_run_id=?,skill_id=?,skill_execution_id=?,"+
				"resource_scopes=? WHERE id=?",
			version,
			origin,
			source.SkillID,
			source.SkillExecutionID,
			string(scopes),
			r.ID,
		).
			Error; err != nil {
			return nil, false, err
		}

		r.AnswerVersion = version
		r.QuestionSourceRunID = &origin
		r.SkillID = source.SkillID
		r.SkillExecutionID = source.SkillExecutionID
	} else {
		r.AnswerVersion = 1
	}

	if title == "新会话" && !titleSource.Valid {
		chars := []rune(strings.TrimSpace(req.Message))
		if len(chars) > 40 {
			chars = chars[:40]
		}

		title = string(chars)
		titleSource = sql.NullTime{Time: now, Valid: true}
	}

	if err = tx.WithContext(ctx).Exec("UPDATE ai_conversation SET title=?,title_source_at=?,last_active_at=? WHERE id=?", title, titleSource, now, conversation).
		Error; err != nil {
		return nil, false, err
	}

	if err = tx.Commit().Error; err != nil {
		return nil, false, err
	}

	return r, true, nil
}

func (s *Store) GetRun(ctx context.Context, user, id string, cutoff time.Time) (*models.Run, error) {
	return scanRun(s.DB.WithContext(ctx).Raw(
		"SELECT r."+strings.ReplaceAll(runColumns, ", ", ", r.")+" FROM ai_run r JOIN ai_conversation c ON c.id=r.conversation_id WHERE r.id=? AND r.user_id=? AND c.user_id=? AND c.deleted_at IS NULL AND r.created_at>=?",
		id,
		user,
		user,
		cutoff,
	).
		Row())
}

func (s *Store) CancelRun(ctx context.Context, user, id string) error {
	return s.stopRun(ctx, user, id, 0)
}

func (s *Store) RejectPlan(ctx context.Context, user, id string, version int) error {
	if version < 1 {
		return common.ErrInvalidRequest
	}
	return s.stopRun(ctx, user, id, version)
}

func (s *Store) stopRun(ctx context.Context, user, id string, rejectedVersion int) error {
	var conversation string
	var root sql.NullString
	if err := s.DB.WithContext(ctx).Raw("SELECT conversation_id,skill_execution_id FROM ai_run WHERE id=? AND user_id=?", id, user).
		Row().
		Scan(&conversation, &root); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return common.ErrNotFound
		}

		return err
	}

	tx := s.DB.WithContext(ctx).Begin()
	err := tx.Error
	if err != nil {
		return err
	}

	defer tx.Rollback()
	var found string
	if err := tx.WithContext(ctx).Raw("SELECT id FROM ai_conversation WHERE id=? AND user_id=? AND deleted_at IS NULL FOR UPDATE", conversation, user).
		Row().
		Scan(&found); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return common.ErrNotFound
		}

		return err
	}

	var active, skillStatus sql.NullString
	if root.Valid {
		if err := tx.WithContext(ctx).Raw("SELECT active_plan_run_id,skill_status FROM ai_run WHERE id=? AND user_id=? FOR UPDATE", root.String, user).
			Row().Scan(&active, &skillStatus); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return common.ErrNotFound
			}
			return err
		}
	}

	var status string
	var version int
	var planStatus sql.NullString
	if err := tx.WithContext(ctx).Raw("SELECT status,plan_version,plan_status FROM ai_run WHERE id=? AND user_id=? FOR UPDATE", id, user).
		Row().Scan(&status, &version, &planStatus); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return common.ErrNotFound
		}
		return err
	}

	if rejectedVersion > 0 {
		// 旧清单自身的版本未变，也必须确认它仍是当前计划。
		if !root.Valid || !active.Valid || active.String != id || version != rejectedVersion {
			return common.ErrRequestConflict
		}
		if status == "cancelled" && planStatus.String == "cancelled" {
			return tx.Commit().Error
		}
		if status != "pending_confirmation" || planStatus.String != "pending_confirmation" || skillStatus.String != "pending_confirmation" {
			return common.ErrRequestConflict
		}
	} else {
		switch status {
		case "pending_confirmation":
			return common.ErrRequestConflict
		case "running", "executing":
			if err := tx.WithContext(ctx).Exec("UPDATE ai_run SET status='cancelling' WHERE id=?", id).Error; err != nil {
				return err
			}
			return tx.Commit().Error
		case "awaiting_input":
			if !root.Valid || skillStatus.String != "awaiting_input" || planStatus.String != "awaiting_input" {
				return common.ErrRequestConflict
			}
			var latest string
			if err := tx.WithContext(ctx).Raw(
				"SELECT id FROM ai_run WHERE conversation_id=? AND user_id=? AND skill_execution_id=? AND question_source_run_id IS NULL ORDER BY created_at DESC,id DESC LIMIT 1 FOR UPDATE",
				conversation, user, root.String,
			).Row().Scan(&latest); err != nil {
				return err
			}
			if latest != id {
				return common.ErrRequestConflict
			}
			// 放弃补问时，同时使该 Skill 尚未执行的旧清单失效。
			if active.Valid && active.String != id {
				if err := tx.WithContext(ctx).Exec("UPDATE ai_run SET plan_status='superseded' WHERE id=? AND plan_status='pending_confirmation'", active.String).Error; err != nil {
					return err
				}
			}
		default:
			return tx.Commit().Error
		}
	}

	if err := s.updateSkillUsage(ctx, tx, user, root.String, "cancelled", 0); err != nil {
		return err
	}
	if err := tx.WithContext(ctx).Exec("UPDATE ai_run SET status='cancelled',plan_status='cancelled',finished_at=? WHERE id=?", time.Now().UTC(), id).Error; err != nil {
		return err
	}

	return tx.Commit().Error
}

func (s *Store) DeleteConversation(ctx context.Context, user, id string) error {
	tx := s.DB.WithContext(ctx).Begin()
	err := tx.Error
	if err != nil {
		return err
	}

	defer tx.Rollback()
	var found string
	if err = tx.WithContext(ctx).Raw("SELECT id FROM ai_conversation WHERE id=? AND user_id=? AND deleted_at IS NULL FOR UPDATE", id, user).
		Row().
		Scan(&found); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return common.ErrNotFound
		}

		return err
	}

	var count int
	if err = tx.WithContext(ctx).Raw(
		"SELECT COUNT(*)"+
			" FROM ai_run"+
			" WHERE conversation_id=?"+
			" AND (status IN ('running','cancelling','executing','unknown') OR plan_status='unknown')",
		id,
	).Row().Scan(&count); err != nil {
		return err
	}

	if count > 0 {
		return common.ErrBusy
	}

	if err := cancelWaitingSkills(ctx, tx, user, id); err != nil {
		return err
	}

	if err = tx.WithContext(ctx).Exec(
		"UPDATE ai_run SET plan_status='cancelled' WHERE conversation_id=? AND plan_status IN ('pending_confirmation','ready')",
		id,
	).
		Error; err != nil {
		return err
	}

	if err = tx.WithContext(ctx).Exec("UPDATE ai_conversation SET deleted_at=? WHERE id=?", time.Now().UTC(), id).
		Error; err != nil {
		return err
	}

	return tx.Commit().Error
}

func (s *Store) FinishRun(ctx context.Context, user string, r *models.Run, status, message string, input, output int) error {
	tx := s.DB.WithContext(ctx).Begin()
	err := tx.Error
	if err != nil {
		return err
	}

	defer tx.Rollback()
	var found string
	if err = tx.WithContext(ctx).Raw("SELECT id FROM ai_conversation WHERE id=? AND user_id=? FOR UPDATE", r.ConversationID, user).
		Row().
		Scan(&found); err != nil {
		return err
	}

	if r.SkillExecutionID != nil {
		if err := tx.WithContext(ctx).Raw("SELECT id FROM ai_run WHERE id=? FOR UPDATE", *r.SkillExecutionID).Row().Scan(&found); err != nil {
			return err
		}
	}

	var current string
	if err = tx.WithContext(ctx).Raw("SELECT status FROM ai_run WHERE id=? FOR UPDATE", r.ID).Row().Scan(&current); err != nil {
		return err
	}

	if common.Terminal(current) {
		return nil
	}

	if current == "cancelling" {
		status = "cancelled"
		message = "运行已取消"
	}

	if r.SkillExecutionID != nil {
		if err := s.updateSkillUsage(
			ctx,
			tx,
			user,
			*r.SkillExecutionID,
			func() string {
				if r.QuestionSourceRunID != nil {
					return ""
				}

				return status
			}(),
			time.Since(r.CreatedAt).Milliseconds(),
		); err != nil {
			return err
		}
	}

	now := time.Now().UTC()
	err = tx.WithContext(ctx).Exec(
		"UPDATE ai_run"+
			" SET status=?,error=?,finished_at=?,input_tokens=?,output_tokens=?,elapsed_ms=?,answer_available=?"+
			" WHERE id=?",
		status,
		message,
		now,
		input,
		output,
		now.Sub(r.CreatedAt).Milliseconds(),
		r.AnswerAvailable,
		r.ID,
	).
		Error
	if err != nil {
		return err
	}

	err = tx.WithContext(ctx).Exec(
		"INSERT INTO ai_usage_daily (date,user_id,skill_id,run_count,input_tokens,output_tokens,elapsed_ms)"+
			" VALUES (?,?, ?,1,?,?,?)"+
			" ON DUPLICATE KEY UPDATE run_count=run_count+1,input_tokens=input_tokens+VALUES(input_tokens),"+
			"output_tokens=output_tokens+VALUES(output_tokens),elapsed_ms=elapsed_ms+VALUES(elapsed_ms)",
		now.In(time.Local).
			Format("2006-01-02"),
		user,
		r.SkillID,
		input,
		output,
		now.Sub(r.CreatedAt).
			Milliseconds(),
	).
		Error
	if err != nil {
		return err
	}

	if err = tx.WithContext(ctx).Exec("UPDATE ai_conversation SET last_active_at=? WHERE id=?", now, r.ConversationID).
		Error; err != nil {
		return err
	}

	return tx.Commit().Error
}

func (s *Store) InterruptRuns(ctx context.Context) error {
	for {
		n, err := s.RecoverAbandoned(ctx, nil)
		if err != nil {
			return err
		}

		if n == 0 {
			return nil
		}
	}
}

func (s *Store) Messages(ctx context.Context, user, conversation, after string, limit int, cutoff time.Time, includeVersions bool) ([]models.HistoryEvent, string, error) {
	if err := s.CheckConversation(ctx, user, conversation); err != nil {
		return nil, "", err
	}

	c, err := common.DecodeCursor(after)
	if err != nil {
		return nil, "", err
	}

	query := "SELECT id,event,created_at" +
		" FROM session_events" +
		" WHERE app_name=? AND user_id=? AND session_id=? AND deleted_at IS NULL AND created_at>=?"
	args := []any{
		halmanconfig.AppName,
		user,
		conversation,
		cutoff,
	}
	if after != "" {
		id, err := strconv.ParseInt(c.ID, 10, 64)
		if err != nil || id < 1 {
			return nil, "", common.ErrInvalidCursor
		}

		query += " AND (created_at<? OR (created_at=? AND id<?))"
		args = append(args, c.At, c.At, id)
	}

	query += " ORDER BY created_at DESC,id DESC LIMIT ?"
	args = append(args, limit+1)
	rows, err := s.DB.WithContext(ctx).Raw(query, args...).Rows()
	if err != nil {
		return nil, "", err
	}

	defer rows.Close()
	items := make([]models.HistoryEvent, 0, limit+1)
	for rows.Next() {
		var item models.HistoryEvent
		var raw []byte
		if err = rows.Scan(&item.ID, &raw, &item.CreatedAt); err != nil {
			return nil, "", err
		}

		var e event.Event
		if err = json.Unmarshal(raw, &e); err != nil {
			return nil, "", err
		}

		item.RunID = e.RequestID
		item.Messages = []model.Message{}
		if e.Response != nil {
			for _, choice := range e.Choices {
				item.Messages = append(item.Messages, choice.Message)
			}
		}

		items = append(items, item)
	}

	if err = rows.Err(); err != nil {
		return nil, "", err
	}

	next := ""
	if len(items) > limit {
		items = items[:limit]
		last := items[len(items)-1]
		next = common.EncodeCursor(last.CreatedAt, strconv.FormatInt(last.ID, 10))
	}

	ids := []string{}
	seen := map[string]bool{}
	for _, item := range items {
		if item.RunID != "" && !seen[item.RunID] {
			ids = append(ids, item.RunID)
			seen[item.RunID] = true
		}
	}

	versions, err := s.MessageVersions(ctx, user, conversation, cutoff, ids)
	if err != nil {
		return nil, "", err
	}

	visible := make([]models.HistoryEvent, 0, len(items))
	for _, item := range items {
		v, ok := versions[item.RunID]
		if !ok {
			continue
		}

		item.QuestionRunID, item.AnswerVersion, item.Latest = v.OriginalID, v.Version, item.RunID == v.LatestID
		messages := make([]model.Message, 0, len(item.Messages))
		for _, message := range item.Messages {
			if includeVersions || item.Latest || (!v.Regenerated && message.Role == model.RoleUser) {
				messages = append(messages, message)
			}
		}

		if len(messages) == 0 {
			continue
		}

		item.Messages = messages
		visible = append(visible, item)
	}

	return visible, next, nil
}
