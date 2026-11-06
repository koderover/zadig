package service

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
)

func (s *AgentService) maintainHistory(ctx context.Context) {
	// 启动补偿及每小时补偿与凌晨主清理使用同一工作者，不启动框架 TTL 清理。
	for {
		s.cleanupPass(ctx)
		local := time.Now().In(time.Local)
		next := time.Date(local.Year(), local.Month(), local.Day(), local.Hour()+1, 0, 0, 0, time.Local)
		timer := time.NewTimer(time.Until(next))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func (s *AgentService) StartMaintenance(ctx context.Context) {
	s.wg.Add(2)
	go func() { defer s.wg.Done(); s.maintainHistory(ctx) }()
	go func() { defer s.wg.Done(); s.maintainRuns(ctx) }()
}

func (s *AgentService) cleanupPass(ctx context.Context) {
	started := time.Now().UTC()
	var stats models.CleanupStats
	cfg, err := s.Platform.Runtime(ctx)
	reason := "configuration_unavailable"
	var oldest, deleted *time.Time
	backlogKnown := false
	if err == nil {
		cutoff := started.AddDate(0, 0, -cfg.HistoryRetentionDays)
		workCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
		stats, err = s.Store.CleanupHistory(workCtx, cutoff)
		cancel()
		reason = "database_error"
		if errors.Is(err, common.ErrBusy) {
			reason = "database_busy"
		} else if errors.Is(err, context.DeadlineExceeded) {
			reason = "timeout"
		}

		backlogCtx, stop := context.WithTimeout(ctx, 3*time.Second)
		var backlogErr error
		oldest, deleted, backlogErr = s.Store.HistoryBacklog(backlogCtx, cutoff)
		backlogKnown = backlogErr == nil
		stop()
		if err == nil && backlogErr != nil {
			err = backlogErr
		}
	}

	finished := time.Now().UTC()
	s.maintenanceMu.Lock()
	state := s.maintenance
	state.LastAttemptAt = &started
	state.LastDurationMS = finished.Sub(started).Milliseconds()
	state.CleanupStats = stats
	if stats.Rows > 0 {
		state.LastProgressAt = &finished
	}

	if backlogKnown {
		state.OldestExpiredAt, state.OldestDeletedAt = oldest, deleted
		state.BacklogOver24h = (oldest != nil && cfg != nil && oldest.Before(started.AddDate(0, 0, -cfg.HistoryRetentionDays).
			Add(-24*time.Hour))) ||
			(deleted != nil && deleted.Before(started.Add(-24*time.Hour)))
	}

	if err == nil {
		state.LastSuccessAt = &finished
		state.PauseReason = ""
	} else {
		state.PauseReason = reason
	}

	s.maintenance = state
	s.maintenanceMu.Unlock()
	if err != nil && ctx.Err() == nil {
		slog.Warn("AI 历史清理已暂停，下一小时补偿", "reason", reason, "rows", stats.Rows, "batches", stats.Batches, "maintenance", state, "database_pool", s.Store.SQLDB.Stats())
	} else if ctx.Err() == nil {
		slog.Info("AI 历史清理完成", "rows", stats.Rows, "batches", stats.Batches, "duration_ms", state.LastDurationMS, "maintenance", state, "database_pool", s.Store.SQLDB.Stats())
	}

	if state.BacklogOver24h && ctx.Err() == nil {
		slog.Warn("AI 历史物理清理积压已超过二十四小时", "oldest_expired_at", oldest, "oldest_deleted_at", deleted)
	}
}
