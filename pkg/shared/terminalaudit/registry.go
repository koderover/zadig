package terminalaudit

import (
	"fmt"
	"sync"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
)

type activeSession struct {
	mu            sync.Mutex
	finalStatus   models.TerminalSessionStatus
	terminate     func()
	terminateOnce sync.Once
	done          chan struct{}
	doneOnce      sync.Once
}

type activeSessionRegistry struct {
	sessions sync.Map
}

var registry = &activeSessionRegistry{}

func RegisterActiveSession(sessionID string, terminate func()) {
	session := &activeSession{
		terminate: terminate,
		done:      make(chan struct{}),
	}
	registry.sessions.Store(sessionID, session)

	go func() {
		select {
		case <-ProcessContext().Done():
			session.terminateWithStatus(models.TerminalSessionStatusAborted)
		case <-session.done:
		}
	}()
}

func UnregisterActiveSession(sessionID string) {
	if session, ok := registry.load(sessionID); ok {
		session.signalDone()
	}
	registry.sessions.Delete(sessionID)
}

func ResolveSessionStatus(sessionID string, defaultStatus models.TerminalSessionStatus) models.TerminalSessionStatus {
	session, ok := registry.load(sessionID)
	if !ok {
		return defaultStatus
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	if session.finalStatus != "" {
		return session.finalStatus
	}
	return defaultStatus
}

func TerminateActiveSession(sessionID string) error {
	session, ok := registry.load(sessionID)
	if !ok {
		return fmt.Errorf("terminal session %s is not active", sessionID)
	}
	session.terminateWithStatus(models.TerminalSessionStatusAborted)
	return nil
}

func (s *activeSession) terminateWithStatus(status models.TerminalSessionStatus) {
	s.mu.Lock()
	s.finalStatus = status
	terminate := s.terminate
	s.mu.Unlock()
	s.terminateOnce.Do(func() {
		if terminate != nil {
			terminate()
		}
	})
}

func (s *activeSession) signalDone() {
	s.doneOnce.Do(func() {
		close(s.done)
	})
}

func (r *activeSessionRegistry) load(sessionID string) (*activeSession, bool) {
	value, ok := r.sessions.Load(sessionID)
	if !ok {
		return nil, false
	}
	session, ok := value.(*activeSession)
	return session, ok
}
