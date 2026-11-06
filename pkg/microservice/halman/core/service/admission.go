package service

import (
	"context"
	"time"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
)

// 成功时保留入场锁，由调用方完成短事务并登记运行后释放；等待时不持锁或数据库连接。
func (s *AgentService) admit(ctx context.Context) error {
	var shutdown <-chan struct{}
	if s.Context != nil {
		shutdown = s.Context.Done()
	}

	deadline := time.Now().Add(s.admissionWaitTimeout)
	timer := time.NewTimer(s.admissionWaitTimeout)
	defer timer.Stop()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	firstAttempt := true
	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		if s.Context != nil && s.Context.Err() != nil {
			return common.ErrCapacity
		}

		if !firstAttempt && !time.Now().Before(deadline) {
			return common.ErrCapacity
		}

		firstAttempt = false
		if s.admissionMu.TryLock() {
			if s.closing {
				s.admissionMu.Unlock()
				return common.ErrCapacity
			}

			s.mu.Lock()
			available := len(s.active) < s.maxConcurrentRuns
			s.mu.Unlock()
			if available {
				return nil
			}

			s.admissionMu.Unlock()
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-shutdown:
			return common.ErrCapacity
		case <-timer.C:
			if err := ctx.Err(); err != nil {
				return err
			}
			return common.ErrCapacity
		case <-ticker.C:
		}
	}
}
