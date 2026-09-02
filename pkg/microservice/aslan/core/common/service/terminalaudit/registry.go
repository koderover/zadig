package terminalaudit

import (
	"context"
	"fmt"
	"sync"

	"github.com/koderover/zadig/v2/pkg/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/tool/cache"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

var processContext context.Context

// SetProcessContext updates the parent context used by active terminal sessions.
func SetProcessContext(ctx context.Context) {
	processContext = ctx
}

type AuditSession struct {
	*asciicastRecorder
	SessionID string
}

func NewAuditSession(meta *SessionMeta, terminate func()) (*AuditSession, error) {
	recorder, err := newRecorder(meta)
	if err != nil {
		return nil, err
	}
	audit := &AuditSession{asciicastRecorder: recorder, SessionID: recorder.session.SessionID}
	if err := registerActiveSession(audit.SessionID, terminate); err != nil {
		// Live-watch/remote-terminate registration is best-effort. If it fails we
		// keep recording; only this session's live spectating is unavailable.
		log.Warnf("register terminal live session failed, recording continues, sessionID=%s err=%v", audit.SessionID, err)
		return audit, nil
	}
	log.Infof("register terminal audit session, sessionID=%s type=%s target=%s", audit.SessionID, meta.SessionType, meta.TargetName)
	return audit, nil
}

func (a *AuditSession) Close(finalStatus models.TerminalSessionStatus) error {
	finalStatus = unregisterActiveSession(a.SessionID, finalStatus)
	return a.asciicastRecorder.Close(finalStatus)
}

type activeSession struct {
	mu              sync.Mutex
	aborted         bool
	closing         bool
	terminate       func()
	terminateOnce   sync.Once
	done            chan struct{}
	terminateCancel context.CancelFunc
	closeTerminate  func()
}

// activeSessions tracks live terminal sessions separately from persisted audit records.
var activeSessions sync.Map

func registerActiveSession(sessionID string, terminate func()) error {
	sessionContext, cancel := context.WithCancel(processContext)
	terminateMessages, closeTerminate, err := subscribeRedis(sessionContext, cache.NewRedisCache(config.RedisCommonCacheTokenDB()), liveTerminateChannel(sessionID))
	if err != nil {
		cancel()
		return fmt.Errorf("subscribe terminal session termination: %w", err)
	}
	session := &activeSession{
		terminate:       terminate,
		done:            make(chan struct{}),
		terminateCancel: cancel,
		closeTerminate:  closeTerminate,
	}
	activeSessions.Store(sessionID, session)

	go func() {
		for {
			select {
			case <-processContext.Done():
				session.abort()
				return
			case <-session.done:
				return
			case message, ok := <-terminateMessages:
				if !ok {
					return
				}
				if message == liveMessageTerminate {
					session.abort()
				}
			}
		}
	}()
	return nil
}

func unregisterActiveSession(sessionID string, defaultStatus models.TerminalSessionStatus) models.TerminalSessionStatus {
	value, ok := activeSessions.LoadAndDelete(sessionID)
	if !ok {
		return defaultStatus
	}
	session := value.(*activeSession)
	status := session.closeWithStatus(defaultStatus)
	close(session.done)
	session.terminateCancel()
	session.closeTerminate()
	return status
}

func (s *activeSession) abort() {
	s.mu.Lock()
	if s.closing {
		s.mu.Unlock()
		return
	}
	s.aborted = true
	terminate := s.terminate
	s.mu.Unlock()
	s.terminateOnce.Do(func() {
		terminate()
	})
}

func (s *activeSession) closeWithStatus(defaultStatus models.TerminalSessionStatus) models.TerminalSessionStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closing = true
	if s.aborted {
		return models.TerminalSessionStatusAborted
	}
	return defaultStatus
}
