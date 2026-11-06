package orm

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"gorm.io/gorm"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
)

type skillContribution struct {
	Date               string `json:"date"`
	Completed          int    `json:"completed"`
	Terminal           int    `json:"terminal"`
	CompletedElapsedMS int64  `json:"completed_elapsed_ms"`
	ActiveMS           int64  `json:"active_ms"`
}

func skillTerminal(status string) bool {
	return status == "succeeded" || status == "failed" || status == "cancelled" || status == "interrupted"
}

// 根执行只贡献一次完成数；补问和重试替换原终态贡献，模型调用量另行累计。
func (s *Store) updateSkillUsage(ctx context.Context, tx *gorm.DB, user, root, status string, elapsed int64) error {
	var skill string
	var current sql.NullString
	var raw []byte
	if err := tx.WithContext(ctx).Raw("SELECT skill_id,skill_status,usage_contribution FROM ai_run WHERE id=? AND user_id=? FOR UPDATE", root, user).
		Row().
		Scan(&skill, &current, &raw); err != nil {
		return err
	}

	if skill == "" {
		return nil
	}

	var previous skillContribution
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &previous); err != nil {
			return err
		}
	}

	if elapsed < 0 {
		elapsed = 0
	}

	next := previous
	next.ActiveMS += elapsed
	if status == "" {
		status = current.String
		if previous.Completed > 0 {
			next.CompletedElapsedMS = next.ActiveMS
			if delta := next.CompletedElapsedMS - previous.CompletedElapsedMS; delta != 0 {
				if err := tx.WithContext(ctx).Exec(
					"UPDATE ai_usage_daily SET completed_elapsed_ms=completed_elapsed_ms+? WHERE date=? AND user_id=? AND skill_id=?",
					delta,
					next.Date,
					user,
					skill,
				).
					Error; err != nil {
					return err
				}
			}
		}
	} else if current.String != "succeeded" {
		if previous.Terminal > 0 || previous.Completed > 0 {
			if err := tx.WithContext(ctx).Exec(
				"UPDATE ai_usage_daily"+
					" SET terminal_count=terminal_count-?,completed_count=completed_count-?,"+
					"completed_elapsed_ms=completed_elapsed_ms-? WHERE date=? AND user_id=? AND skill_id=?",
				previous.Terminal,
				previous.Completed,
				previous.CompletedElapsedMS,
				previous.Date,
				user,
				skill,
			).
				Error; err != nil {
				return err
			}
		}

		next.Terminal, next.Completed, next.CompletedElapsedMS = 0, 0, 0
		if skillTerminal(status) {
			next.Date = time.Now().In(time.Local).Format("2006-01-02")
			next.Terminal = 1
			if status == "succeeded" {
				next.Completed = 1
				next.CompletedElapsedMS = next.ActiveMS
			}

			if err := tx.WithContext(ctx).Exec(
				"INSERT INTO ai_usage_daily (date,user_id,skill_id,terminal_count,completed_count,"+
					"completed_elapsed_ms)"+
					" VALUES (?,?,?,?,?,?)"+
					" ON DUPLICATE KEY UPDATE terminal_count=terminal_count+VALUES(terminal_count),"+
					"completed_count=completed_count+VALUES(completed_count),"+
					"completed_elapsed_ms=completed_elapsed_ms+VALUES(completed_elapsed_ms)",
				next.Date,
				user,
				skill,
				next.Terminal,
				next.Completed,
				next.CompletedElapsedMS,
			).
				Error; err != nil {
				return err
			}
		}
	} else {
		status = "succeeded"
		next.CompletedElapsedMS = next.ActiveMS
		if delta := next.CompletedElapsedMS - previous.CompletedElapsedMS; delta != 0 {
			if err := tx.WithContext(ctx).Exec(
				"UPDATE ai_usage_daily SET completed_elapsed_ms=completed_elapsed_ms+? WHERE date=? AND user_id=? AND skill_id=?",
				delta,
				next.Date,
				user,
				skill,
			).
				Error; err != nil {
				return err
			}
		}
	}

	err := tx.WithContext(ctx).Exec("UPDATE ai_run SET skill_status=?,usage_contribution=? WHERE id=?", status, string(common.JSON(next)), root).
		Error
	return err
}
