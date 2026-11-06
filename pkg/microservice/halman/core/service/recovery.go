package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
)

func (s *AgentService) recoverAbandoned(ctx context.Context) error {
	if !s.admissionMu.TryLock() {
		return common.ErrBusy
	}

	defer s.admissionMu.Unlock()
	if s.closing {
		return nil
	}

	if stats := s.Store.SQLDB.Stats(); stats.InUse >= stats.MaxOpenConnections {
		return common.ErrBusy
	}

	s.mu.Lock()
	ids := make([]string, 0, len(s.active))
	for id := range s.active {
		ids = append(ids, id)
	}

	s.mu.Unlock()
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	_, err := s.Store.RecoverAbandoned(ctx, ids)
	return err
}

func (s *AgentService) maintainRuns(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		if err := s.recoverAbandoned(ctx); err != nil && ctx.Err() == nil && err != common.ErrBusy {
			slog.Warn("AI 异常运行恢复未完成，下次继续", "error_type", "run_recovery_failed")
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
