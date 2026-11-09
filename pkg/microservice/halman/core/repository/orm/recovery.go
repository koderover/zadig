package orm

import (
	"context"
	"database/sql"
	"log/slog"
	"strings"
	"time"
)

type abandonedRun struct {
	id, user, conversation string
	root                   sql.NullString
}

// 进程内入场锁由调用方持有，防止把已建行、尚未登记内存的运行误判为孤儿。
func (s *Store) RecoverAbandoned(ctx context.Context, activeIDs []string) (int, error) {
	query := "SELECT id,user_id,conversation_id,skill_execution_id FROM ai_run WHERE status IN ('running','cancelling','executing')"
	args := []any{}
	if len(activeIDs) > 0 {
		marks := make([]string, len(activeIDs))
		for i, id := range activeIDs {
			marks[i] = "?"
			args = append(args, id)
		}

		query += " AND id NOT IN (" + strings.Join(marks, ",") + ")"
	}

	query += " ORDER BY created_at,id LIMIT 20"
	rows, err := s.DB.WithContext(ctx).Raw(query, args...).Rows()
	if err != nil {
		return 0, err
	}

	items := []abandonedRun{}
	for rows.Next() {
		var item abandonedRun
		if err := rows.Scan(&item.id, &item.user, &item.conversation, &item.root); err != nil {
			rows.Close()
			return 0, err
		}

		items = append(items, item)
	}

	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, err
	}

	for i, item := range items {
		if err := s.recoverRun(ctx, item); err != nil {
			return i, err
		}
	}

	return len(items), nil
}

func (s *Store) recoverRun(ctx context.Context, item abandonedRun) error {
	tx := s.DB.WithContext(ctx).Begin()
	if tx.Error != nil {
		return tx.Error
	}

	defer tx.Rollback()
	var found string
	if err := tx.Raw("SELECT id FROM ai_conversation WHERE id=? AND user_id=? FOR UPDATE", item.conversation, item.user).
		Row().
		Scan(&found); err != nil {
		return err
	}

	rootExists := item.root.Valid
	if rootExists {
		err := tx.Raw("SELECT id FROM ai_run WHERE id=? FOR UPDATE", item.root.String).Row().Scan(&found)
		if err == sql.ErrNoRows {
			rootExists = false
		} else if err != nil {
			return err
		}
	}

	var status string
	var plan, questionSource sql.NullString
	if err := tx.Raw("SELECT status,plan_status,question_source_run_id FROM ai_run WHERE id=? AND user_id=? FOR UPDATE", item.id, item.user).
		Row().
		Scan(&status, &plan, &questionSource); err != nil {
		return err
	}

	if status != "running" && status != "cancelling" && status != "executing" {
		return nil
	}

	next, message := "interrupted", "运行已中断或结果保存失败，未自动重放；请重新发起请求"
	uncertain := status == "executing" || plan.String == "executing"
	if uncertain {
		next, message = "unknown", "已提交动作的结果需要核实；禁止自动重试"
	} else if status == "cancelling" {
		next = "cancelled"
	}

	if rootExists {
		goal := next
		if questionSource.Valid {
			goal = ""
		}

		if err := s.updateSkillUsage(ctx, tx, item.user, item.root.String, goal, 0); err != nil {
			return err
		}
	}

	if err := tx.Exec(
		"UPDATE ai_run SET status=?,plan_status=CASE WHEN ? THEN 'unknown' ELSE plan_status END,error=?,finished_at=? WHERE id=?",
		next,
		uncertain,
		message,
		time.Now().
			UTC(),
		item.id,
	).
		Error; err != nil {
		return err
	}

	if err := tx.Commit().Error; err != nil {
		return err
	}
	slog.Info("AI 异常运行已恢复", "run_id", item.id, "previous_status", status, "status", next)
	return nil
}
