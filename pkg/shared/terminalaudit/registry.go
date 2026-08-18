package terminalaudit

import (
	"context"
	"fmt"
	"sync"

	"github.com/koderover/zadig/v2/pkg/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/tool/cache"
)

type activeSession struct {
	mu              sync.Mutex
	finalStatus     models.TerminalSessionStatus
	closing         bool
	terminate       func()
	terminateOnce   sync.Once
	done            chan struct{}
	terminateSub    *redisLiveSubscription
	terminateCancel context.CancelFunc
	closeOnce       sync.Once
}

// activeSessions tracks live terminal sessions separately from persisted audit records.
var activeSessions sync.Map

func registerActiveSession(sessionID string, terminate func()) error {
	processContext := processLifecycleContext()
	sessionContext, cancel := context.WithCancel(processContext)
	terminateSub, err := subscribeRedis(sessionContext, cache.NewRedisCache(config.RedisCommonCacheTokenDB()), liveTerminateChannel(sessionID))
	if err != nil {
		cancel()
		return fmt.Errorf("subscribe terminal session termination: %w", err)
	}
	session := &activeSession{
		terminate:       terminate,
		done:            make(chan struct{}),
		terminateSub:    terminateSub,
		terminateCancel: cancel,
	}
	activeSessions.Store(sessionID, session)

	go func() {
		for {
			select {
			case <-processContext.Done():
				session.terminateWithStatus(models.TerminalSessionStatusAborted)
				return
			case <-session.done:
				return
			case message, ok := <-terminateSub.Messages():
				if !ok {
					return
				}
				if message == liveMessageTerminate {
					session.terminateWithStatus(models.TerminalSessionStatusAborted)
				}
			}
		}
	}()
	return nil
}

func unregisterActiveSession(sessionID string) {
	if session, ok := loadActiveSession(sessionID); ok {
		session.close()
	}
	activeSessions.Delete(sessionID)
}

func (s *activeSession) terminateWithStatus(status models.TerminalSessionStatus) {
	s.mu.Lock()
	if s.closing {
		s.mu.Unlock()
		return
	}
	s.finalStatus = status
	terminate := s.terminate
	s.mu.Unlock()
	s.terminateOnce.Do(func() {
		if terminate != nil {
			terminate()
		}
	})
}

func (s *activeSession) closeWithStatus(defaultStatus models.TerminalSessionStatus) models.TerminalSessionStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closing = true
	if s.finalStatus != "" {
		return s.finalStatus
	}
	return defaultStatus
}

func (s *activeSession) close() {
	s.closeOnce.Do(func() {
		close(s.done)
		s.terminateCancel()
		_ = s.terminateSub.Close()
	})
}

func loadActiveSession(sessionID string) (*activeSession, bool) {
	value, ok := activeSessions.Load(sessionID)
	if !ok {
		return nil, false
	}
	session, ok := value.(*activeSession)
	return session, ok
}
