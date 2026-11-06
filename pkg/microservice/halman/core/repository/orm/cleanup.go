package orm

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"trpc.group/trpc-go/trpc-agent-go/event"
	"trpc.group/trpc-go/trpc-agent-go/model"

	halmanconfig "github.com/koderover/zadig/v2/pkg/microservice/halman/config"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
)

func cleanupDelay(ctx context.Context) bool {
	timer := time.NewTimer(200 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

type cleanupJob struct {
	query string
	args  []any
}

func historyCleanupJobs(cutoff time.Time) []cleanupJob {
	jobs := []cleanupJob{
		{
			"UPDATE ai_run" +
				" SET status='expired',plan_status='expired',error='已超过保留期，原执行和确认入口失效'" +
				" WHERE (status='unknown' OR plan_status='unknown') AND created_at<? ORDER BY created_at,id LIMIT 500",
			[]any{cutoff},
		},
	}
	for _, table := range []string{"session_events", "session_track_events"} {
		jobs = append(
			jobs,
			cleanupJob{
				"DELETE FROM " + table + " WHERE app_name=? AND created_at<? ORDER BY created_at,id LIMIT 32",
				[]any{halmanconfig.AppName, cutoff},
			},
		)
	}

	for _, table := range []string{
		"session_states",
		"session_summaries",
		"app_states",
		"user_states",
	} {
		jobs = append(
			jobs,
			cleanupJob{
				"DELETE FROM " + table + " WHERE app_name=? AND updated_at<? ORDER BY updated_at,id LIMIT 32",
				[]any{halmanconfig.AppName, cutoff},
			},
		)
	}

	jobs = append(
		jobs,
		cleanupJob{
			"DELETE FROM ai_feedback WHERE created_at<? ORDER BY created_at,id LIMIT 500",
			[]any{cutoff},
		},
		cleanupJob{
			"DELETE FROM ai_usage_daily WHERE date<? ORDER BY date,user_id,skill_id LIMIT 500",
			[]any{time.Now().
				In(time.Local).
				AddDate(0, 0, -364).
				Format("2006-01-02")},
		},
		cleanupJob{
			"UPDATE ai_conversation" +
				" SET deleted_at=?" +
				" WHERE deleted_at IS NULL" +
				" AND last_active_at<?" +
				" AND NOT EXISTS (SELECT 1" +
				" FROM ai_run r" +
				" WHERE r.conversation_id=ai_conversation.id" +
				" AND (r.status IN ('running','cancelling','executing','unknown') OR r.plan_status='unknown'))" +
				" ORDER BY last_active_at,id LIMIT 500",
			[]any{time.Now().UTC(), cutoff},
		},
	)
	return jobs
}

func (s *Store) CleanupHistory(ctx context.Context, cutoff time.Time) (models.CleanupStats, error) {
	stats := models.CleanupStats{}
	jobs := historyCleanupJobs(cutoff)
	for {
		changed := false
		for i := 0; i < len(jobs)+4; i++ {
			if err := ctx.Err(); err != nil {
				return stats, err
			}
			// 不与已占满预算的在线请求争抢业务连接。
			if pool := s.SQLDB.Stats(); pool.InUse >= pool.MaxOpenConnections {
				return stats, common.ErrBusy
			}

			batchCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			var count int64
			var err error
			if i < len(jobs) {
				result := s.DB.WithContext(batchCtx).Exec(jobs[i].query, jobs[i].args...)
				count, err = result.RowsAffected, result.Error
			} else if i == len(jobs) {
				count, err = s.cleanupDeletedConversation(batchCtx)
			} else if i == len(jobs)+1 {
				count, err = s.refreshExpiredTitle(batchCtx, cutoff)
			} else if i == len(jobs)+2 {
				count, err = s.cleanupExpiredRuns(batchCtx, cutoff)
			} else {
				count, err = s.expireDetachedUnknown(batchCtx, cutoff)
			}

			cancel()
			if err != nil {
				return stats, err
			}

			stats.Batches++
			stats.Rows += count
			changed = changed || count > 0
			if !cleanupDelay(ctx) {
				return stats, ctx.Err()
			}
		}

		if !changed {
			return stats, nil
		}
	}
}

// 根执行到期后，近期补问不延长旧目标寿命，也不以未核实子记录永久保留过期正文。
func (s *Store) expireDetachedUnknown(ctx context.Context, cutoff time.Time) (int64, error) {
	rows, err := s.DB.WithContext(ctx).Raw(
		"SELECT child.id"+
			" FROM ai_run child"+
			" LEFT JOIN ai_run root ON root.id=child.skill_execution_id"+
			" AND root.created_at>=?"+
			" WHERE (child.status='unknown' OR child.plan_status='unknown')"+
			" AND child.skill_execution_id IS NOT NULL"+
			" AND child.skill_execution_id<>child.id"+
			" AND root.id IS NULL ORDER BY child.created_at,child.id LIMIT 50",
		cutoff,
	).Rows()
	if err != nil {
		return 0, err
	}

	ids := []any{}
	marks := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}

		ids = append(ids, id)
		marks = append(marks, "?")
	}

	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, err
	}

	if len(ids) == 0 {
		return 0, nil
	}

	result := s.DB.WithContext(ctx).Exec(
		"UPDATE ai_run SET status='expired',plan_status='expired',error='原 Skill 已到期，不能继续或重试；新需求需要新建执行' WHERE id IN ("+strings.Join(marks, ",")+") AND (status='unknown' OR plan_status='unknown')",
		ids...,
	)
	return result.RowsAffected, result.Error
}

// 每次只删除一个表的至多 500 行，不能调用框架的整会话级联删除。
func (s *Store) cleanupDeletedConversation(ctx context.Context) (int64, error) {
	var conversation, user string
	err := s.DB.WithContext(ctx).Raw("SELECT c.id,c.user_id"+
		" FROM ai_conversation c"+
		" WHERE c.deleted_at IS NOT NULL"+
		" AND NOT EXISTS (SELECT 1"+
		" FROM ai_run r"+
		" WHERE r.conversation_id=c.id"+
		" AND (r.status IN ('running','cancelling','executing','unknown') OR r.plan_status='unknown'))"+
		" ORDER BY c.deleted_at,c.id LIMIT 1").Row().Scan(&conversation, &user)
	if err == sql.ErrNoRows {
		return 0, nil
	}

	if err != nil {
		return 0, err
	}

	for _, table := range []string{
		"session_events",
		"session_track_events",
		"session_summaries",
		"session_states",
	} {
		result := s.DB.WithContext(ctx).Exec(
			"DELETE FROM "+table+" WHERE app_name=? AND user_id=? AND session_id=? LIMIT 32",
			halmanconfig.AppName,
			user,
			conversation,
		)
		if result.Error != nil || result.RowsAffected > 0 {
			return result.RowsAffected, result.Error
		}
	}

	rows, err := s.DB.WithContext(ctx).Raw("SELECT id FROM ai_run WHERE conversation_id=? ORDER BY created_at,id LIMIT 50", conversation).
		Rows()
	if err != nil {
		return 0, err
	}

	ids := []any{}
	placeholders := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}

		ids = append(ids, id)
		placeholders = append(placeholders, "?")
	}

	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, err
	}

	if len(ids) > 0 {
		args := append([]any{user}, ids...)
		result := s.DB.WithContext(ctx).Exec("DELETE FROM ai_feedback WHERE user_id=? AND answer_run_id IN ("+strings.Join(placeholders, ",")+") LIMIT 500", args...)
		if result.Error != nil || result.RowsAffected > 0 {
			return result.RowsAffected, result.Error
		}

		result = s.DB.WithContext(ctx).Exec(
			"DELETE FROM ai_run WHERE id IN ("+strings.Join(placeholders, ",")+") AND status NOT IN ('running','cancelling','executing','unknown') AND (plan_status IS NULL OR plan_status<>'unknown') LIMIT 50",
			ids...,
		)
		return result.RowsAffected, result.Error
	}

	result := s.DB.WithContext(ctx).Exec("DELETE FROM ai_conversation WHERE id=? AND user_id=? AND deleted_at IS NOT NULL", conversation, user)
	return result.RowsAffected, result.Error
}

func (s *Store) refreshExpiredTitle(ctx context.Context, cutoff time.Time) (int64, error) {
	var id, user string
	err := s.DB.WithContext(ctx).Raw(
		"SELECT id,user_id"+
			" FROM ai_conversation"+
			" WHERE deleted_at IS NULL"+
			" AND title_source_at IS NULL AND created_at<? ORDER BY title_source_at,created_at,id LIMIT 1",
		cutoff,
	).Row().Scan(&id, &user)
	if err == sql.ErrNoRows {
		err = s.DB.WithContext(ctx).Raw(
			"SELECT id,user_id"+
				" FROM ai_conversation"+
				" WHERE deleted_at IS NULL AND title_source_at<? ORDER BY title_source_at,created_at,id LIMIT 1",
			cutoff,
		).Row().
			Scan(&id, &user)
	}

	if err == sql.ErrNoRows {
		return 0, nil
	}

	if err != nil {
		return 0, err
	}

	tx := s.DB.WithContext(ctx).Begin()
	err = tx.Error
	if err != nil {
		return 0, err
	}

	defer tx.Rollback()
	var found string
	if err := tx.WithContext(ctx).Raw("SELECT id FROM ai_conversation WHERE id=? AND user_id=? AND deleted_at IS NULL FOR UPDATE", id, user).
		Row().
		Scan(&found); err != nil {
		return 0, err
	}

	rows, err := tx.WithContext(ctx).Raw(
		"SELECT event,created_at"+
			" FROM session_events"+
			" WHERE app_name=?"+
			" AND user_id=?"+
			" AND session_id=? AND deleted_at IS NULL AND created_at>=? ORDER BY created_at,id LIMIT 20",
		halmanconfig.AppName,
		user,
		id,
		cutoff,
	).
		Rows()
	if err != nil {
		return 0, err
	}

	title, source := "历史会话", time.Now().UTC()
	for rows.Next() {
		var raw []byte
		var created time.Time
		if err := rows.Scan(&raw, &created); err != nil {
			rows.Close()
			return 0, err
		}

		var e event.Event
		if json.Unmarshal(raw, &e) != nil {
			rows.Close()
			return 0, common.ErrRequestConflict
		}

		if e.Response == nil {
			continue
		}

		for _, choice := range e.Choices {
			if choice.Message.Role == model.RoleUser && strings.TrimSpace(choice.Message.Content) != "" {
				runes := []rune(choice.Message.Content)
				if len(runes) > 40 {
					runes = runes[:40]
				}

				title, source = string(runes), created
				break
			}
		}

		if title != "历史会话" {
			break
		}
	}

	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, err
	}

	result := tx.WithContext(ctx).Exec("UPDATE ai_conversation SET title=?,title_source_at=? WHERE id=?", title, source, id)
	if result.Error != nil {
		return 0, result.Error
	}

	if err := tx.Commit().Error; err != nil {
		return 0, err
	}

	return result.RowsAffected, result.Error
}

func (s *Store) cleanupExpiredRuns(ctx context.Context, cutoff time.Time) (int64, error) {
	var conversation, user string
	err := s.DB.WithContext(ctx).Raw(
		"SELECT r.conversation_id,r.user_id"+
			" FROM ai_run r"+
			" WHERE r.created_at<?"+
			" AND r.status NOT IN ('running','cancelling','executing','unknown')"+
			" AND (plan_status IS NULL OR plan_status<>'unknown')"+
			" AND NOT EXISTS (SELECT 1"+
			" FROM ai_run child"+
			" WHERE child.skill_execution_id=r.id"+
			" AND (child.status IN ('running','cancelling','executing',"+
			"'unknown') OR child.plan_status='unknown')) ORDER BY r.created_at,r.id LIMIT 1",
		cutoff,
	).Row().Scan(&conversation, &user)
	if err == sql.ErrNoRows {
		return 0, nil
	}

	if err != nil {
		return 0, err
	}

	tx := s.DB.WithContext(ctx).Begin()
	err = tx.Error
	if err != nil {
		return 0, err
	}

	defer tx.Rollback()
	var found string
	if err := tx.WithContext(ctx).Raw("SELECT id FROM ai_conversation WHERE id=? AND user_id=? FOR UPDATE", conversation, user).
		Row().
		Scan(&found); err != nil {
		return 0, err
	}

	rows, err := tx.WithContext(ctx).Raw(
		"SELECT r.id"+
			" FROM ai_run r"+
			" WHERE r.conversation_id=?"+
			" AND r.created_at<?"+
			" AND r.status NOT IN ('running','cancelling','executing','unknown')"+
			" AND (plan_status IS NULL OR plan_status<>'unknown')"+
			" AND NOT EXISTS (SELECT 1"+
			" FROM ai_run child"+
			" WHERE child.skill_execution_id=r.id"+
			" AND (child.status IN ('running','cancelling','executing',"+
			"'unknown') OR child.plan_status='unknown')) ORDER BY r.created_at,r.id LIMIT 50 FOR UPDATE",
		conversation,
		cutoff,
	).
		Rows()
	if err != nil {
		return 0, err
	}

	ids, placeholders := []any{}, []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}

		ids = append(ids, id)
		placeholders = append(placeholders, "?")
	}

	err = rows.Err()
	rows.Close()
	if err != nil || len(ids) == 0 {
		return 0, err
	}

	args := append([]any{user}, ids...)
	result := tx.WithContext(ctx).Exec("DELETE FROM ai_feedback WHERE user_id=? AND answer_run_id IN ("+strings.Join(placeholders, ",")+") LIMIT 500", args...)
	if result.Error != nil {
		return 0, result.Error
	}

	count := result.RowsAffected
	if count == 0 {
		result = tx.WithContext(ctx).Exec("DELETE FROM ai_run WHERE id IN ("+strings.Join(placeholders, ",")+") LIMIT 50", ids...)
		if result.Error != nil {
			return 0, result.Error
		}

		count = result.RowsAffected
	}

	if err := tx.Commit().Error; err != nil {
		return 0, err
	}

	return count, nil
}
